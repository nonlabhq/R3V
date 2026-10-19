package project

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/redact"
)

// Steps (docs/design/steps.md): the project after each of an agent's moves,
// on this computer only. A step is a version never shared, as a parked set
// is; a session's turns (prompts) and steps are lines in
// .r3v/steps/<session>.jsonl, this copy's.

// Steps: agents' tool calls make steps (every build since 0.3; tests turn
// it off to play an older R3V).
var Steps = true

const stepsDir = "steps"

// ByHand is the session of steps made with `r3v step` outside an agent's
// session.
const ByHand = "by-hand"

// How long a session is kept once all its steps are in versions (or left
// behind), and the room steps may take on their own.
var (
	stepsKeep         = 7 * 24 * time.Hour
	stepsBudget int64 = 2 << 30
)

// StepSettings are this copy's choices about steps (the app's project
// settings): never shared.
type StepSettings struct {
	Off       bool `json:"off,omitempty"`        // no steps at all
	NoPrompts bool `json:"no_prompts,omitempty"` // steps, but prompts not kept
}

// Step is one step of a session.
type Step struct {
	N    int    `json:"n"`
	Turn int    `json:"turn"`
	ID   string `json:"version"` // the files as they were (a version never shared)
	On   string `json:"on"`      // the version the project was on
	Time string `json:"time"`
	Tool string `json:"tool"`
	// Detail: what the call did, in a line (a command's description).
	Detail string   `json:"detail,omitempty"`
	Actor  string   `json:"actor,omitempty"`
	Files  []string `json:"files"` // changed since the step before
}

// Turn is a prompt (or a note given with a step) and the steps after it.
type Turn struct {
	N          int    `json:"n"`
	Time       string `json:"time"`
	Prompt     string `json:"prompt,omitempty"`
	Note       string `json:"note,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	Steps      []Step `json:"steps"`
}

// Session is an agent's conversation, as its steps tell it.
type Session struct {
	ID    string `json:"id"`
	Last  string `json:"last"` // the time of its latest line
	Steps int    `json:"steps"`
	Turns []Turn `json:"turns"`
	// Copy: the folder of the copy that kept it, when another than the one
	// asked (VersionSteps: an agent's workspace).
	Copy string `json:"copy,omitempty"`
}

// stepLine is a line of a session's list: a turn, or a step.
type stepLine struct {
	N          int      `json:"n,omitempty"`
	Turn       int      `json:"turn"`
	Version    string   `json:"version,omitempty"`
	On         string   `json:"on,omitempty"`
	Time       string   `json:"time"`
	Tool       string   `json:"tool,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	Actor      string   `json:"actor,omitempty"`
	Files      []string `json:"files,omitempty"`
	Own        bool     `json:"own,omitempty"` // Version was made for this step (dropped with it)
	Prompt     string   `json:"prompt,omitempty"`
	Note       string   `json:"note,omitempty"`
	Transcript string   `json:"transcript,omitempty"`
}

func (l stepLine) isStep() bool { return l.Version != "" }

// ErrNoStep: a session or a step the lists don't have.
var ErrNoStep = errors.New("no such step")

var plainSession = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// sessionFile is a session's list's name: its id when that is a plain name.
func sessionFile(id string) string {
	if !plainSession.MatchString(id) || strings.HasPrefix(id, ".") {
		sum := sha256.Sum256([]byte(id))
		id = "s-" + hex.EncodeToString(sum[:8])
	}
	return id + ".jsonl"
}

func (r *Repo) stepSettings() StepSettings {
	if r.Config.Steps == nil {
		return StepSettings{}
	}
	return *r.Config.Steps
}

// StepsOn: this copy makes steps (the team's workflow says so, and this
// copy didn't turn them off).
func (r *Repo) StepsOn() bool { return Steps && !r.stepSettings().Off && r.Workflow().StepsByTeam() }

// keepsPrompts: prompts are kept with the steps (by the team's workflow and
// this copy's own choice).
func (r *Repo) keepsPrompts() bool {
	return !r.stepSettings().NoPrompts && r.Workflow().PromptsByTeam()
}

// SetStepSettings changes this copy's choices about steps.
func (r *Repo) SetStepSettings(s StepSettings) error {
	r.Config.Steps = &s
	if s == (StepSettings{}) {
		r.Config.Steps = nil
	}
	return r.SaveConfig()
}

// StepSettingsNow are this copy's choices about steps.
func (r *Repo) StepSettingsNow() StepSettings { return r.stepSettings() }

func (r *Repo) readSession(id string) ([]stepLine, error) {
	f, err := os.Open(filepath.Join(r.Dir, stepsDir, sessionFile(id)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []stepLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		var l stepLine
		if json.Unmarshal(sc.Bytes(), &l) == nil { // a line cut short (R3V stopped) is skipped
			out = append(out, l)
		}
	}
	return out, sc.Err()
}

// appendStepLine adds a line to a session's list, in one write: stopped
// half-way, the line is cut short and skipped when read.
func (r *Repo) appendStepLine(id string, l stepLine) error {
	if err := os.MkdirAll(filepath.Join(r.Dir, stepsDir), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(r.Dir, stepsDir, sessionFile(id)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// lastOf is the latest turn number and step of a session's lines.
func lastOf(lines []stepLine) (turn int, step *stepLine) {
	for i := range lines {
		if lines[i].isStep() {
			step = &lines[i]
		} else if lines[i].Turn > turn {
			turn = lines[i].Turn
		}
	}
	return turn, step
}

// maxPrompt: a prompt longer than this is kept cut (a pasted file, a log).
const maxPrompt = 4000

// StepTurn starts a turn of a session: the person's prompt (redacted; not
// kept when the settings say so) and where the agent tool keeps the
// conversation.
func (r *Repo) StepTurn(session, prompt, transcript string) error {
	if !r.StepsOn() || session == "" {
		return nil
	}
	lines, err := r.readSession(session)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	turn, _ := lastOf(lines)
	l := stepLine{Turn: turn + 1, Time: time.Now().UTC().Format(time.RFC3339), Transcript: transcript}
	if r.keepsPrompts() {
		if len(prompt) > maxPrompt {
			prompt = prompt[:maxPrompt] + "…"
		}
		l.Prompt = redact.Text(prompt)
	}
	return r.appendStepLine(session, l)
}

// TakeStep makes a step of the files as they are, when they changed since
// the session's step before (nil when they didn't): tool is what moved
// them, note a note given by hand (a turn of its own).
func (r *Repo) TakeStep(session, tool, note string) (*Step, error) {
	return r.TakeStepWith(session, tool, "", note)
}

// TakeStepWith is TakeStep with what the call did, in a line (a command's
// description, redacted).
func (r *Repo) TakeStepWith(session, tool, detail, note string) (*Step, error) {
	return r.takeStep(session, 0, tool, detail, note)
}

// takeStep is TakeStepWith, the step in turn inTurn (0: the session's
// latest).
func (r *Repo) takeStep(session string, inTurn int, tool, detail, note string) (*Step, error) {
	if !r.StepsOn() {
		return nil, nil
	}
	if session == "" {
		session = ByHand
	}
	lines, err := r.readSession(session)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	turn, prev := lastOf(lines)
	if inTurn > 0 && note == "" {
		turn = inTurn
	}
	if note != "" {
		turn++
		if err := r.appendStepLine(session, stepLine{Turn: turn, Time: time.Now().UTC().Format(time.RFC3339), Note: note}); err != nil {
			return nil, err
		}
	}
	head := r.Head()
	var before *Manifest // the files at the step before (or the version)
	if prev != nil {
		before, _ = r.Load(prev.Version)
	}
	if before == nil && head != "" {
		if before, err = r.Load(head); err != nil {
			return nil, err
		}
	}
	only := r.Only
	r.Only = nil
	m, ix, err := r.workingManifest("Step")
	r.Only = only
	id := ""
	switch {
	case errors.Is(err, ErrNothingToSnapshot): // the files are the version's
		if before == nil || before.ID == head {
			return nil, nil
		}
		if m, err = r.Load(head); err != nil {
			return nil, err
		}
		id = head
	case err != nil:
		return nil, err
	case before != nil && sameContent(before, m):
		return nil, nil
	}
	if id == "" {
		// Every new content is kept here, even what the team's storage has.
		known := map[string]bool{}
		if before != nil {
			for _, f := range before.Files {
				known[f.Hash] = true
			}
		}
		for _, f := range m.Files {
			if known[f.Hash] || r.Store.Has(f.Hash) {
				continue
			}
			if h, _, err := r.storeFile(r.Abs(f.Path)); err != nil {
				return nil, fmt.Errorf("%s: %w", f.Path, err)
			} else if h != f.Hash {
				return nil, fmt.Errorf("%s changed while the step was made", f.Path)
			}
			known[f.Hash] = true
		}
		if prev != nil {
			m.Parents = []string{prev.Version}
		}
		m.Message = fmt.Sprintf("Step %d", stepCount(lines)+1)
		if err := r.save(m); err != nil {
			return nil, err
		}
		if err := ix.save(); err != nil {
			return nil, err
		}
		id = m.ID
	}
	l := stepLine{N: stepCount(lines) + 1, Turn: turn, Version: id, On: head, Time: time.Now().UTC().Format(time.RFC3339),
		Tool: tool, Detail: redact.Text(detail), Files: changedPaths(orEmpty(before), m), Own: id != head}
	if r.Actor != nil {
		l.Actor = r.Actor.Tool
	}
	if err := r.appendStepLine(session, l); err != nil {
		return nil, err
	}
	s := l.step()
	return &s, nil
}

func stepCount(lines []stepLine) int {
	n := 0
	for _, l := range lines {
		if l.isStep() && l.N > n {
			n = l.N
		}
	}
	return n
}

func (l stepLine) step() Step {
	files := l.Files
	if files == nil {
		files = []string{}
	}
	return Step{N: l.N, Turn: l.Turn, ID: l.Version, On: l.On, Time: l.Time, Tool: l.Tool, Detail: l.Detail, Actor: l.Actor, Files: files}
}

// orEmpty is m, or no files when m is nil.
func orEmpty(m *Manifest) *Manifest {
	if m == nil {
		return &Manifest{}
	}
	return m
}

// sessionOf reads a session's list as turns and steps.
func sessionOf(id string, lines []stepLine) Session {
	s := Session{ID: id, Turns: []Turn{}}
	turns := map[int]int{} // turn number -> index in s.Turns
	turnAt := func(n int, l *stepLine) *Turn {
		i, ok := turns[n]
		if !ok {
			t := Turn{N: n, Steps: []Step{}}
			if l != nil {
				t.Time = l.Time
			}
			s.Turns = append(s.Turns, t)
			i = len(s.Turns) - 1
			turns[n] = i
		}
		return &s.Turns[i]
	}
	for i := range lines {
		l := &lines[i]
		if l.Time > s.Last {
			s.Last = l.Time
		}
		if !l.isStep() {
			t := turnAt(l.Turn, l)
			t.Time, t.Prompt, t.Note, t.Transcript = l.Time, l.Prompt, l.Note, l.Transcript
			continue
		}
		t := turnAt(l.Turn, l)
		t.Steps = append(t.Steps, l.step())
		s.Steps++
	}
	return s
}

// Sessions are this copy's sessions with steps, the latest first.
func (r *Repo) Sessions() ([]Session, error) {
	entries, err := os.ReadDir(filepath.Join(r.Dir, stepsDir))
	if errors.Is(err, os.ErrNotExist) {
		return []Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Session{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || e.IsDir() {
			continue
		}
		lines, err := r.readSession(id)
		if err != nil {
			continue
		}
		out = append(out, sessionOf(id, lines))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Last > out[j].Last })
	return out, nil
}

// SessionSteps is a session's turns and steps ("" for the latest session).
func (r *Repo) SessionSteps(id string) (*Session, error) {
	if id == "" {
		all, err := r.Sessions()
		if err != nil {
			return nil, err
		}
		if len(all) == 0 {
			return nil, ErrNoStep
		}
		return &all[0], nil
	}
	lines, err := r.readSession(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("session %s: %w", id, ErrNoStep)
	}
	if err != nil {
		return nil, err
	}
	s := sessionOf(id, lines)
	return &s, nil
}

func (r *Repo) stepIn(session string, n int) (*Session, *Step, error) {
	s, err := r.SessionSteps(session)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range s.Turns {
		for _, st := range t.Steps {
			if st.N == n {
				return s, &st, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("step %d: %w", n, ErrNoStep)
}

// StepBack puts the files as they were at step n of a session ("" the
// latest), on the version the project is on. A step of the files as they
// are comes first, so going back is a move of its own; then a line saying
// where they went. Stopped while files are written, the files are partly
// back: an unfinished switch, finished as any (RecoverSwitch).
func (r *Repo) StepBack(session string, n int) (*Step, []string, error) {
	if !r.StepsOn() {
		return nil, nil, errors.New("steps are off for this project")
	}
	s, st, err := r.stepIn(session, n)
	if err != nil {
		return nil, nil, err
	}
	if _, err := r.TakeStep(s.ID, "before going back", ""); err != nil {
		return nil, nil, err
	}
	lines, err := r.readSession(s.ID)
	if err != nil {
		return nil, nil, err
	}
	turn, prev := lastOf(lines)
	target, err := r.Load(st.ID)
	if err != nil {
		return nil, nil, err
	}
	var before *Manifest
	if prev != nil {
		before, _ = r.Load(prev.Version)
	}
	notes, err := r.putFiles(target, r.Head(), "work "+st.ID)
	if err != nil {
		return nil, nil, err
	}
	l := stepLine{N: stepCount(lines) + 1, Turn: turn, Version: st.ID, On: r.Head(), Time: time.Now().UTC().Format(time.RFC3339),
		Tool: fmt.Sprintf("back to step %d", n), Files: changedPaths(orEmpty(before), target)}
	if err := r.appendStepLine(s.ID, l); err != nil {
		return nil, nil, err
	}
	back := l.step()
	return &back, notes, nil
}

// StepChange is a file that differs between two steps.
type StepChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added, modified, deleted
}

// StepDiff is what changed from step a to step b of a session (b 0: the
// files now).
func (r *Repo) StepDiff(session string, a, b int) ([]StepChange, error) {
	s, sa, err := r.stepIn(session, a)
	if err != nil {
		return nil, err
	}
	from, err := r.Load(sa.ID)
	if err != nil {
		return nil, err
	}
	var to []FileEntry
	if b == 0 {
		if to, err = r.workingFiles(r.loadIndex()); err != nil {
			return nil, err
		}
	} else {
		_, sb, err := r.stepIn(s.ID, b)
		if err != nil {
			return nil, err
		}
		m, err := r.Load(sb.ID)
		if err != nil {
			return nil, err
		}
		to = m.Files
	}
	was := map[string]string{}
	for _, f := range from.Files {
		was[f.Path] = f.Hash
	}
	out := []StepChange{}
	for _, f := range to {
		h, ok := was[f.Path]
		switch {
		case !ok:
			out = append(out, StepChange{f.Path, "added"})
		case h != f.Hash:
			out = append(out, StepChange{f.Path, "modified"})
		}
		delete(was, f.Path)
	}
	for p := range was {
		out = append(out, StepChange{p, "deleted"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// stepHashes is the content steps hold that the versions they were made on
// don't: kept here (PruneObjects leaves it), whatever the team's storage
// has.
func (r *Repo) stepHashes() map[string]bool {
	out := map[string]bool{}
	sessions, _ := r.Sessions()
	for _, s := range sessions {
		for h := range r.sessionHashes(s) {
			out[h] = true
		}
	}
	return out
}

// sessionHashes is a session's own content, with sizes.
func (r *Repo) sessionHashes(s Session) map[string]int64 {
	out := map[string]int64{}
	known := map[string]map[string]bool{}
	for _, t := range s.Turns {
		for _, st := range t.Steps {
			if st.ID == st.On {
				continue
			}
			k, ok := known[st.On]
			if !ok {
				k = map[string]bool{}
				if v, err := r.Load(st.On); err == nil {
					for _, f := range v.Files {
						k[f.Hash] = true
					}
				}
				known[st.On] = k
			}
			m, err := r.Load(st.ID)
			if err != nil {
				continue
			}
			for _, f := range m.Files {
				if !k[f.Hash] {
					out[f.Hash] = f.Size
				}
			}
		}
	}
	return out
}

// PruneSteps forgets sessions no longer needed: a session with a step made
// on the version the project is on stays; another goes once its latest
// line is older than stepsKeep, or, oldest first, while sessions take more
// than stepsBudget of their own content. Their versions go (gc then frees
// the content nothing else uses).
func (r *Repo) PruneSteps() {
	sessions, err := r.Sessions()
	if err != nil || len(sessions) == 0 {
		return
	}
	head := r.Head()
	open := func(s Session) bool {
		for _, t := range s.Turns {
			for _, st := range t.Steps {
				if st.On == head {
					return true
				}
			}
		}
		return false
	}
	cutoff := time.Now().Add(-stepsKeep).UTC().Format(time.RFC3339)
	var kept []Session
	for _, s := range sessions {
		if !open(s) && s.Last < cutoff {
			r.dropSession(s)
			continue
		}
		kept = append(kept, s)
	}
	// Over the budget: the oldest (not open) go first.
	sizes := map[string]int64{}
	var total int64
	all := map[string]int64{}
	for _, s := range kept {
		hs := r.sessionHashes(s)
		for h, n := range hs {
			if _, seen := all[h]; !seen {
				sizes[s.ID] += n
			}
		}
		maps.Copy(all, hs)
	}
	for _, n := range all {
		total += n
	}
	for i := len(kept) - 1; i >= 0 && total > stepsBudget; i-- {
		if open(kept[i]) {
			continue
		}
		total -= sizes[kept[i].ID]
		r.dropSession(kept[i])
	}
}

// dropSession removes a session's list, then the versions made for its
// steps (never one of the history: a step whose files were a version's
// names that version).
func (r *Repo) dropSession(s Session) {
	lines, err := r.readSession(s.ID)
	if err != nil || os.Remove(filepath.Join(r.Dir, stepsDir, sessionFile(s.ID))) != nil {
		return
	}
	for _, l := range lines {
		if l.Own {
			os.Remove(r.snapshotPath(l.Version))
		}
	}
}

// VersionSteps are the steps a version was made of, from any copy of the
// project here (a workspace's agent made them there): those made on its
// parent up to its time, under their turns, by session. A merge was made of
// what it brought in too: the steps of each version on its other side not
// already on its first (the newest maxMergeSteps versions of it). None for
// a version without a parent, or whose steps aren't on this computer.
func (r *Repo) VersionSteps(id string) ([]Session, error) {
	m, err := r.Header(id)
	if err != nil {
		return nil, err
	}
	out := []Session{}
	if len(m.Parents) == 0 {
		return out, nil
	}
	made := []*Manifest{m}
	if len(m.Parents) > 1 {
		mine, err := r.ancestors(m.Parents[0])
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		queue := append([]string{}, m.Parents[1:]...)
		for len(queue) > 0 && len(made) <= maxMergeSteps {
			cur := queue[0]
			queue = queue[1:]
			if cur == "" || seen[cur] || mine[cur] {
				continue
			}
			seen[cur] = true
			h, err := r.Header(cur)
			if err != nil {
				continue
			}
			if len(h.Parents) > 0 {
				made = append(made, h)
			}
			queue = append(queue, h.Parents...)
		}
	}
	// A step made on a version's parent before it, on its branch (a lane
	// and main can go on from the same version: the step's files say which
	// branch it was on).
	branchOf := map[string]string{}
	stepBranch := func(c *Repo, st Step) string {
		b, ok := branchOf[st.ID]
		if !ok {
			if h, err := c.Header(st.ID); err == nil {
				b = h.Branch
			}
			branchOf[st.ID] = b
		}
		return b
	}
	ofVersion := func(c *Repo, st Step) bool {
		for _, v := range made {
			if st.On == v.Parents[0] && st.Time <= v.Time {
				if b := stepBranch(c, st); b == "" || v.Branch == "" || b == v.Branch {
					return true
				}
			}
		}
		return false
	}
	for _, c := range append([]*Repo{r}, r.otherCopies()...) {
		all, err := c.Sessions()
		if err != nil {
			continue
		}
		for _, s := range all {
			kept := Session{ID: s.ID, Turns: []Turn{}}
			if c != r {
				kept.Copy = c.Root
			}
			for _, t := range s.Turns {
				var steps []Step
				for _, st := range t.Steps {
					if ofVersion(c, st) {
						steps = append(steps, st)
					}
				}
				if len(steps) > 0 {
					t.Steps = steps
					kept.Turns = append(kept.Turns, t)
					kept.Steps += len(steps)
					kept.Last = steps[len(steps)-1].Time
				}
			}
			if kept.Steps > 0 {
				out = append(out, kept)
			}
		}
	}
	return out, nil
}

// maxMergeSteps is how many of the versions a merge brought in its steps
// are looked for in (the newest first): enough for a lane's work between
// merges, without reading a long history.
const maxMergeSteps = 200
