package project

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The keys of the lines the hooks write (StepTurn, TakeStepWith): a start
// and an end write no others, so the app and older R3V read them alike.
var hookTurnKeys = map[string]bool{"turn": true, "time": true, "prompt": true, "note": true, "transcript": true}
var hookStepKeys = map[string]bool{"n": true, "turn": true, "version": true, "on": true, "time": true, "tool": true,
	"detail": true, "actor": true, "files": true, "own": true}

func sessionLines(t *testing.T, r *Repo, id string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(r.Dir, stepsDir, sessionFile(id)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l map[string]any
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		out = append(out, l)
	}
	return out
}

// A task is a turn with its note, then a step of the files at its end, by
// the agent: the lines the hooks write, read as theirs are.
func TestStepStartEnd(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "thread-1"}
	st, err := r.StepStart("Warm the lights, token AKIAIOSFODNN7EXAMPLE")
	if err != nil || st == nil || st.Open.Session != "thread-1" || st.Open.Turn != 1 || st.Before != nil || st.Previous != nil {
		t.Fatalf("start: %+v %v", st, err)
	}
	if strings.Contains(st.Open.Note, "AKIA") {
		t.Fatalf("note not redacted: %q", st.Open.Note)
	}
	if o := r.OpenTask(); o == nil || o.Turn != 1 {
		t.Fatalf("open: %+v", o)
	}
	write(t, r.Root, "notes.txt", "two")
	end, err := r.StepEnd("Warmer lights")
	if err != nil || end == nil || end.Step == nil || end.Step.N != 1 || end.Step.Turn != 1 || end.Step.Tool != ToolStepEnd ||
		end.Step.Detail != "Warmer lights" || end.Step.Actor != "codex" || strings.Join(end.Step.Files, ",") != "notes.txt" {
		t.Fatalf("end: %+v %v", end, err)
	}
	if r.OpenTask() != nil {
		t.Fatal("still open after end")
	}
	if again, err := r.StepEnd(""); err != nil || again != nil {
		t.Fatalf("end with none open: %+v %v", again, err)
	}
	for i, l := range sessionLines(t, r, "thread-1") {
		keys := hookTurnKeys
		if _, ok := l["version"]; ok {
			keys = hookStepKeys
		}
		for k := range l {
			if !keys[k] {
				t.Fatalf("line %d: key %q the hooks don't write: %v", i, k, l)
			}
		}
	}
	s, err := r.SessionSteps("thread-1")
	if err != nil || s.Steps != 1 || len(s.Turns) != 1 || !strings.HasPrefix(s.Turns[0].Note, "Warm the lights") ||
		s.Turns[0].Steps[0].Tool != ToolStepEnd {
		t.Fatalf("session: %+v %v", s, err)
	}
	// Going back to the end of a task works as for a hook's step.
	write(t, r.Root, "notes.txt", "three")
	if _, _, err := r.StepBack("thread-1", 1); err != nil || read(t, r, "notes.txt") != "two" {
		t.Fatalf("back: %v %q", err, read(t, r, "notes.txt"))
	}
}

// Files changed before a start (the person's edits) are a step of their
// own at the start of the turn: the end's step has only the task's.
func TestStepStartKeepsTheFilesBefore(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "opencode", Session: "ses_1"}
	write(t, r.Root, "mine.txt", "the person's")
	st, err := r.StepStart("Add the fog")
	if err != nil || st.Before == nil || st.Before.Tool != ToolStepStart || st.Before.Turn != 1 ||
		strings.Join(st.Before.Files, ",") != "mine.txt" {
		t.Fatalf("start: %+v %v", st, err)
	}
	write(t, r.Root, "fog.txt", "thick")
	end, err := r.StepEnd("")
	if err != nil || end.Step == nil || strings.Join(end.Step.Files, ",") != "fog.txt" || end.Step.Turn != 1 {
		t.Fatalf("end: %+v %v", end, err)
	}
}

// A start while a task is open ends it first, with a step in its own turn;
// one open longer than staleStep is closed without a step (the files may be
// anyone's by then): they go into the new task's first step.
func TestStepStartEndsTheOpenOne(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "pi", Session: "p1"}
	if _, err := r.StepStart("one"); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "two")
	st, err := r.StepStart("two")
	if err != nil || st.Previous == nil || st.Previous.Turn != 1 || st.Previous.Step == nil ||
		st.Previous.Step.Turn != 1 || !strings.Contains(st.Previous.Step.Detail, "not ended") || st.Open.Turn != 2 || st.Before != nil {
		t.Fatalf("second start: %+v %v", st, err)
	}

	was := staleStep
	staleStep = 0
	t.Cleanup(func() { staleStep = was })
	write(t, r.Root, "notes.txt", "three")
	time.Sleep(1100 * time.Millisecond) // (times are to the second)
	st, err = r.StepStart("three")
	if err != nil || st.Previous == nil || !st.Previous.Stale || st.Previous.Step != nil ||
		st.Before == nil || st.Before.Turn != 3 {
		t.Fatalf("after a stale one: %+v %v", st, err)
	}
}

// Agents tell their own apart: two sessions, two open tasks.
func TestStepTasksPerSession(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "a"}
	r.StepStart("in a")
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "b"}
	if st, err := r.StepStart("in b"); err != nil || st.Previous != nil || st.Open.Session != "b" {
		t.Fatalf("b: %+v %v", st, err)
	}
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "a"}
	if o := r.OpenTask(); o == nil || o.Session != "a" {
		t.Fatalf("a's: %+v", o)
	}
}

// A tool that gives no session id goes on in its latest session while it
// is recent, and starts one after sessionGap.
func TestStepStartWithoutSessionID(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "gemini-cli"}
	first, err := r.StepStart("one")
	if err != nil || !strings.HasPrefix(first.Open.Session, "gemini-cli-") {
		t.Fatalf("first: %+v %v", first, err)
	}
	r.StepEnd("")
	next, err := r.StepStart("two")
	if err != nil || next.Open.Session != first.Open.Session || next.Open.Turn != 2 {
		t.Fatalf("next: %+v %v", next, err)
	}
	r.StepEnd("")
	was := sessionGap
	sessionGap = 0
	t.Cleanup(func() { sessionGap = was })
	time.Sleep(1100 * time.Millisecond) // (the new session's name is to the second)
	later, err := r.StepStart("three")
	if err != nil || later.Open.Session == first.Open.Session || later.Open.Turn != 1 {
		t.Fatalf("later: %+v %v", later, err)
	}
}

// Stopped half-way: the turn written, the record of what is open not (or
// cut short): nothing is open, the end is a no-op, the next start begins.
// Stopped after the end's step, before the record is cleared: ending again
// makes no second step.
func TestStepTasksInterrupted(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "k"}
	if _, err := r.StepStart("one"); err != nil {
		t.Fatal(err)
	}
	path := r.agentTasksPath("codex", "k")
	os.WriteFile(path, []byte(`{"session":"k","open":{"sess`), 0o644)
	if end, err := r.StepEnd(""); err != nil || end != nil {
		t.Fatalf("cut record: %+v %v", end, err)
	}
	st, err := r.StepStart("two")
	if err != nil || st.Previous != nil || st.Open.Turn != 2 {
		t.Fatalf("start after: %+v %v", st, err)
	}
	saved, _ := os.ReadFile(path)
	write(t, r.Root, "notes.txt", "two")
	if end, err := r.StepEnd(""); err != nil || end.Step == nil {
		t.Fatalf("end: %+v %v", end, err)
	}
	os.WriteFile(path, saved, 0o644) // (as if stopped before it was cleared)
	if end, err := r.StepEnd(""); err != nil || end == nil || end.Step != nil {
		t.Fatalf("end again: %+v %v", end, err)
	}
	if s, _ := r.SessionSteps("k"); s.Steps != 1 {
		t.Fatalf("steps: %d", s.Steps)
	}
}

// Steps off: start and end keep nothing.
func TestStepTasksOff(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "x"}
	if err := r.SetStepSettings(StepSettings{Off: true}); err != nil {
		t.Fatal(err)
	}
	if st, err := r.StepStart("one"); err != nil || st != nil {
		t.Fatalf("start: %+v %v", st, err)
	}
	if all, _ := r.Sessions(); len(all) != 0 {
		t.Fatalf("sessions: %+v", all)
	}
}

// A commit ends the task the agent left open: its step, named for the
// commit, is the version's. With none open it ends nothing, and a
// person's commit ends nothing either.
func TestCommitEndsTheOpenTask(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "c"}
	if _, err := r.StepStart("Warm the lights"); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "two")
	ended, err := r.EndTaskAtCommit("Warmer lights")
	if err != nil || ended == nil || ended.Step == nil || ended.Step.Detail != CommitEndDetail+"Warmer lights" ||
		ended.Step.Tool != ToolStepEnd || ended.Turn != 1 {
		t.Fatalf("ended: %+v %v", ended, err)
	}
	if r.OpenTask() != nil {
		t.Fatal("still open after the commit's end")
	}
	v := mustSnapshot(t, r, "Warmer lights")
	got, err := r.VersionSteps(v.ID)
	if err != nil || len(got) != 1 || got[0].Steps != 1 || got[0].Turns[0].Note != "Warm the lights" {
		t.Fatalf("version's steps: %+v %v", got, err)
	}
	if again, err := r.EndTaskAtCommit("next"); err != nil || again != nil {
		t.Fatalf("none open: %+v %v", again, err)
	}

	r.Actor = nil // a person's task by hand: their commit leaves it open
	if _, err := r.StepStart("by hand"); err != nil {
		t.Fatal(err)
	}
	if e, err := r.EndTaskAtCommit("mine"); err != nil || e != nil || r.OpenTask() == nil {
		t.Fatalf("a person's commit: %+v %v", e, err)
	}
}

// The commit fails after ending the task (or is stopped there): the task
// stays ended, and the next commit's version lists its step.
func TestCommitEndsTheTaskThenFails(t *testing.T) {
	r := stepsProject(t)
	r.Actor = &Actor{Kind: "agent", Tool: "codex", Session: "f"}
	if _, err := r.StepStart("Retitle"); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "two")
	if _, err := r.EndTaskAtCommit("Retitled"); err != nil {
		t.Fatal(err)
	}
	// (no version made: the commit failed)
	if r.OpenTask() != nil {
		t.Fatal("open again")
	}
	time.Sleep(1100 * time.Millisecond) // (times are to the second)
	v := mustSnapshot(t, r, "Retitled, again")
	if got, _ := r.VersionSteps(v.ID); len(got) != 1 || got[0].Steps != 1 {
		t.Fatalf("the next version's steps: %+v", got)
	}
}
