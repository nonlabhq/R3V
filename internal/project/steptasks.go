package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/redact"
	"github.com/nonlabhq/r3v/internal/store"
)

// Steps for agent tools without hooks (docs/design/steps.md, "Tools
// without hooks"): the agent wraps each task in `r3v step start` and `r3v
// step end`. A start is a turn (its note what the agent is about to do), an
// end a step: the same lines the hooks write. What is open is kept beside
// the lists, one small file per agent (its tool and session), so that an
// end finds its start.

// staleStep: a step left open longer (an agent that crashed or forgot) is
// closed by the next start without a step, the files then being the
// person's as much as the agent's.
var staleStep = 12 * time.Hour

// sessionGap: a tool that gives no session id goes on in its latest
// session when that was last written to within this, else starts one.
var sessionGap = 4 * time.Hour

// Tools of the steps start and end make (Step.Tool).
const (
	ToolStepStart = "step start"
	ToolStepEnd   = "step end"
)

// OpenStep is a task an agent began with StepStart and hasn't ended.
type OpenStep struct {
	Session string `json:"session"`
	Turn    int    `json:"turn"`
	Since   string `json:"since"`
	Note    string `json:"note,omitempty"`
}

// StepStarted is what StepStart did: the task begun, the step of the files
// as they were then (when they changed since the step before), and the
// task left open before it, ended now.
type StepStarted struct {
	Open     OpenStep   `json:"open"`
	Before   *Step      `json:"before"`
	Previous *StepEnded `json:"previous"`
}

// StepEnded is a task ended: its step of the files then (nil: they hadn't
// changed). Stale: it was open too long and was closed without one.
type StepEnded struct {
	OpenStep
	Step  *Step `json:"step"`
	Stale bool  `json:"stale,omitempty"`
}

// agentTasks is an agent's file beside the lists.
type agentTasks struct {
	Session string    `json:"session"` // its latest session
	Open    *OpenStep `json:"open,omitempty"`
}

// stepAgent is who makes a start or end: the actor's tool and session
// (a person: ByHand).
func (r *Repo) stepAgent() (tool, session string) {
	if r.Actor == nil || r.Actor.Tool == "" {
		return "", ByHand
	}
	return r.Actor.Tool, r.Actor.Session
}

func (r *Repo) agentTasksPath(tool, session string) string {
	key := tool
	if key == "" {
		key = ByHand
	}
	if session != "" {
		key += "." + session
	}
	name := strings.TrimSuffix(sessionFile(key), ".jsonl")
	return filepath.Join(r.Dir, stepsDir, ".open-"+name+".json")
}

func (r *Repo) readAgentTasks(path string) agentTasks {
	var a agentTasks
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &a) // (unreadable: nothing open)
	}
	return a
}

func writeAgentTasks(path string, a agentTasks) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return store.WriteAtomic(path, bytes.NewReader(data))
}

// agentNote is an agent's note as kept: one paragraph, redacted, cut.
func agentNote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxPrompt {
		s = s[:maxPrompt] + "…"
	}
	return redact.Text(s)
}

// StepStart begins a task of the agent running R3V (Repo.Actor): a turn
// whose note is what it is about to do, then a step of the files as they
// are when they changed since the step before (the person's edits, or
// what was done outside a task), in that turn. A task still open is ended
// first: with a step of the files now, or, open longer than staleStep,
// without one. Nil when steps are off.
//
// Stopped half-way: a turn with no step and nothing open (the end that
// follows is a no-op), or a turn with its first step and nothing open;
// the record of what is open is written last, whole or not at all.
func (r *Repo) StepStart(note string) (*StepStarted, error) {
	if !r.StepsOn() {
		return nil, nil
	}
	tool, id := r.stepAgent()
	path := r.agentTasksPath(tool, id)
	tasks := r.readAgentTasks(path)
	out := &StepStarted{}
	if tasks.Open != nil {
		ended, err := r.endTask(*tasks.Open, "not ended: ended by the next step start", true)
		if err != nil {
			return nil, err
		}
		out.Previous = ended
		tasks.Open = nil
		if err := writeAgentTasks(path, tasks); err != nil {
			return nil, err
		}
	}
	session := id
	if session == "" {
		session = r.latestSession(tasks.Session)
		if session == "" {
			session = tool + "-" + time.Now().UTC().Format("20060102-150405")
		}
	}
	lines, err := r.readSession(session)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	turn, _ := lastOf(lines)
	now := time.Now().UTC().Format(time.RFC3339)
	open := OpenStep{Session: session, Turn: turn + 1, Since: now, Note: agentNote(note)}
	if err := r.appendStepLine(session, stepLine{Turn: open.Turn, Time: now, Note: open.Note}); err != nil {
		return nil, err
	}
	before, err := r.takeStep(session, open.Turn, ToolStepStart, "", "")
	if err != nil {
		return nil, err
	}
	out.Open, out.Before = open, before
	tasks.Session, tasks.Open = session, &open
	return out, writeAgentTasks(path, tasks)
}

// latestSession is the session an agent without session ids goes on in:
// its latest, when it is still listed and was written to within
// sessionGap.
func (r *Repo) latestSession(id string) string {
	if id == "" {
		return ""
	}
	lines, err := r.readSession(id)
	if err != nil {
		return ""
	}
	s := sessionOf(id, lines)
	if t, err := time.Parse(time.RFC3339, s.Last); err != nil || time.Since(t) > sessionGap {
		return ""
	}
	return id
}

// StepEnd ends the task the agent running R3V began: a step of the files
// as they are (nil when unchanged), in its turn; detail says what it did.
// Nil when none is open (or steps are off). Stopped before the open record
// is cleared, the task is still open: ending it again makes no second
// step (the files are as at the first).
func (r *Repo) StepEnd(detail string) (*StepEnded, error) {
	if !r.StepsOn() {
		return nil, nil
	}
	tool, id := r.stepAgent()
	path := r.agentTasksPath(tool, id)
	tasks := r.readAgentTasks(path)
	if tasks.Open == nil {
		return nil, nil
	}
	ended, err := r.endTask(*tasks.Open, detail, false)
	if err != nil {
		return nil, err
	}
	tasks.Open = nil
	return ended, writeAgentTasks(path, tasks)
}

// OpenTask is the task the agent running R3V began and hasn't ended, nil
// when none is.
func (r *Repo) OpenTask() *OpenStep {
	tool, id := r.stepAgent()
	return r.readAgentTasks(r.agentTasksPath(tool, id)).Open
}

// endTask makes open's end step; byNext: the next start ends it (one open
// longer than staleStep then gets none).
func (r *Repo) endTask(open OpenStep, detail string, byNext bool) (*StepEnded, error) {
	out := &StepEnded{OpenStep: open}
	if since, err := time.Parse(time.RFC3339, open.Since); byNext && err == nil && time.Since(since) > staleStep {
		out.Stale = true
		return out, nil
	}
	if _, err := r.readSession(open.Session); errors.Is(err, os.ErrNotExist) {
		return out, nil // its session was let go (PruneSteps)
	}
	st, err := r.takeStep(open.Session, open.Turn, ToolStepEnd, agentNote(detail), "")
	if err != nil {
		return nil, err
	}
	out.Step = st
	return out, nil
}
