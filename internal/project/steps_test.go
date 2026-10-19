package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withSteps turns steps on for a test (every build since 0.3 has them).
func withSteps(t *testing.T) {
	t.Helper()
	was := Steps
	Steps = true
	t.Cleanup(func() { Steps = was })
}

// stepsProject is a project with a first version and notes.txt in it.
func stepsProject(t *testing.T) *Repo {
	t.Helper()
	withSteps(t)
	r, err := Init(newProject(t), "yi")
	if err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "one")
	mustSnapshot(t, r, "first")
	return r
}

func read(t *testing.T, r *Repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(data)
}

// A move that changed files is a step, under the turn of the prompt that
// asked for it; one that changed nothing isn't. The prompt's secrets are
// taken out.
func TestTakeStep(t *testing.T) {
	r := stepsProject(t)
	if st, err := r.TakeStep("s1", "Read", ""); err != nil || st != nil {
		t.Fatalf("nothing changed: %+v %v", st, err)
	}
	if err := r.StepTurn("s1", "Warmer light, the key is AKIAIOSFODNN7EXAMPLE", "C:/t/s1.jsonl"); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "notes.txt", "two")
	write(t, r.Root, "light.txt", "warm")
	st, err := r.TakeStep("s1", "Write", "")
	if err != nil || st == nil || st.N != 1 || st.Turn != 1 || strings.Join(st.Files, ",") != "light.txt,notes.txt" {
		t.Fatalf("step 1: %+v %v", st, err)
	}
	if again, _ := r.TakeStep("s1", "Bash", ""); again != nil {
		t.Fatalf("no change, a step: %+v", again)
	}
	write(t, r.Root, "notes.txt", "three")
	st2, _ := r.TakeStep("s1", "Edit", "")
	if st2 == nil || st2.N != 2 || strings.Join(st2.Files, ",") != "notes.txt" {
		t.Fatalf("step 2: %+v", st2)
	}
	s, err := r.SessionSteps("")
	if err != nil || s.ID != "s1" || s.Steps != 2 || len(s.Turns) != 1 {
		t.Fatalf("session: %+v %v", s, err)
	}
	if p := s.Turns[0].Prompt; !strings.HasPrefix(p, "Warmer light") || strings.Contains(p, "AKIA") {
		t.Fatalf("prompt: %q", p)
	}
	// Steps are never a version of the history.
	if log, _ := r.Log(); len(log) != 1 {
		t.Fatalf("history: %d versions", len(log))
	}
	assertChanged(t, r, "light.txt", "notes.txt")
}

func assertChanged(t *testing.T, r *Repo, paths ...string) {
	t.Helper()
	changes, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range changes {
		got = append(got, c.Path)
	}
	if strings.Join(got, ",") != strings.Join(paths, ",") {
		t.Fatalf("changes: %v, want %v", got, paths)
	}
}

// Going back to a step puts its files, on the version the project is on;
// the files as they were are a step first, so going back can be undone.
func TestStepBack(t *testing.T) {
	r := stepsProject(t)
	head := r.Head()
	write(t, r.Root, "notes.txt", "two")
	r.TakeStep("s1", "Write", "")
	write(t, r.Root, "notes.txt", "three")
	write(t, r.Root, "extra.txt", "made by step 2")
	r.TakeStep("s1", "Write", "")
	write(t, r.Root, "notes.txt", "three, edited by hand")

	back, _, err := r.StepBack("s1", 1)
	if err != nil || back == nil || !strings.HasPrefix(back.Tool, "back to step 1") {
		t.Fatalf("back: %+v %v", back, err)
	}
	if read(t, r, "notes.txt") != "two" || read(t, r, "extra.txt") != "" || r.Head() != head {
		t.Fatalf("after going back: %q %q %s", read(t, r, "notes.txt"), read(t, r, "extra.txt"), r.Head())
	}
	s, _ := r.SessionSteps("s1")
	if s.Steps != 4 { // 1, 2, the files before going back (3), back (4)
		t.Fatalf("steps: %d", s.Steps)
	}
	if _, _, err := r.StepBack("s1", 3); err != nil {
		t.Fatal(err)
	}
	if read(t, r, "notes.txt") != "three, edited by hand" || read(t, r, "extra.txt") != "made by step 2" {
		t.Fatal("going back undone: the files aren't as they were")
	}
	diff, err := r.StepDiff("s1", 1, 2)
	if err != nil || fmt.Sprint(diff) != "[{extra.txt added} {notes.txt modified}]" {
		t.Fatalf("diff: %v %v", diff, err)
	}
	if _, _, err := r.StepBack("s1", 99); !errors.Is(err, ErrNoStep) {
		t.Fatalf("no such step: %v", err)
	}
}

// Files back as the version's: a step naming the version itself, nothing
// new kept for it.
func TestStepAsTheVersion(t *testing.T) {
	r := stepsProject(t)
	write(t, r.Root, "notes.txt", "two")
	r.TakeStep("s1", "Write", "")
	write(t, r.Root, "notes.txt", "one")
	st, err := r.TakeStep("s1", "Write", "")
	if err != nil || st == nil || st.ID != r.Head() {
		t.Fatalf("back as the version: %+v %v", st, err)
	}
}

// A session whose steps are all in a version (or left behind) goes after
// stepsKeep, its own versions too; one with steps on the version the
// project is on stays. Steps can be turned off, and prompts.
func TestPruneSteps(t *testing.T) {
	r := stepsProject(t)
	write(t, r.Root, "notes.txt", "two")
	old, _ := r.TakeStep("old", "Write", "")
	mustSnapshot(t, r, "second") // old's steps are in a version now
	write(t, r.Root, "notes.txt", "three")
	r.TakeStep("open", "Write", "")
	was := stepsKeep
	stepsKeep = -time.Hour
	defer func() { stepsKeep = was }()
	r.PruneSteps()
	all, _ := r.Sessions()
	if len(all) != 1 || all[0].ID != "open" || r.HasSnapshot(old.ID) {
		t.Fatalf("after pruning: %+v (old's version kept: %v)", all, r.HasSnapshot(old.ID))
	}

	r.SetStepSettings(StepSettings{NoPrompts: true})
	r.StepTurn("open", "a prompt", "")
	if s, _ := r.SessionSteps("open"); s.Turns[len(s.Turns)-1].Prompt != "" {
		t.Fatal("a prompt kept with prompts off")
	}
	r.SetStepSettings(StepSettings{Off: true})
	write(t, r.Root, "notes.txt", "four")
	if st, _ := r.TakeStep("open", "Write", ""); st != nil {
		t.Fatal("a step with steps off")
	}
}

// The content a step holds stays here once the team's storage has the
// files (PruneObjects), so going back works offline.
func TestStepContentKept(t *testing.T) {
	withSteps(t)
	a, _ := team(t)
	write(t, a.Root, "notes.txt", "a step's own content")
	st, err := a.TakeStep("s1", "Write", "")
	if err != nil || st == nil {
		t.Fatal(err)
	}
	m, _ := a.Load(st.ID)
	hash := ""
	for _, f := range m.Files {
		if f.Path == "notes.txt" {
			hash = f.Hash
		}
	}
	if _, err := a.PruneObjects(); err != nil {
		t.Fatal(err)
	}
	if !a.Store.Has(hash) {
		t.Fatal("a step's content went")
	}
}

// R3V stopped while going back to a step: each file whole, the project
// knows it didn't finish, and finishing it puts the step's files; nothing
// is lost (the files before are a step).
func TestStepBackKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills steps")
	}
	setup := func() (*Repo, map[string]string, map[string]string) {
		r := stepsProject(t)
		write(t, r.Root, "notes.txt", "step one")
		write(t, r.Root, "a.txt", "a1")
		r.TakeStep("s1", "Write", "")
		target := files(t, r.Root)
		write(t, r.Root, "notes.txt", "now")
		write(t, r.Root, "a.txt", "a2")
		write(t, r.Root, "b.txt", "b")
		return r, target, files(t, r.Root)
	}
	r, _, _ := setup()
	n := writes(func() {
		if _, _, err := r.StepBack("s1", 1); err != nil {
			t.Fatal(err)
		}
	})
	if n < 2 {
		t.Fatalf("going back wrote %d files", n)
	}
	for k := 1; k <= n; k++ {
		what := fmt.Sprintf("killed after %d of %d files", k, n)
		r, target, now := setup()
		if !runKilled(t, nil, "step-back", "", 0, "R3V_KILL_DIR="+r.Root, "R3V_KILL_STEPN=1",
			fmt.Sprintf("R3V_KILL_WRITES=%d", k)) {
			t.Fatalf("%s: it finished", what)
		}
		r, _ = Open(r.Root)
		eitherOf(t, what, files(t, r.Root), now, target)
		if r.UnfinishedSwitch() == "" {
			t.Fatalf("%s: not known unfinished", what)
		}
		if changed, err := r.ChangedSinceSwitch(); err != nil || len(changed) != 0 {
			t.Fatalf("%s: changed since: %v %v", what, changed, err)
		}
		if _, err := r.RecoverSwitch(); err != nil {
			t.Fatalf("%s: finishing: %v", what, err)
		}
		// (the set is relinked for this computer as any switch does)
		if got := files(t, r.Root); !maps.Equal(withoutSets(got), withoutSets(target)) {
			t.Fatalf("%s: after finishing: %v", what, fileNames(got))
		}
		s, _ := r.SessionSteps("s1")
		before, err := r.Load(s.Turns[0].Steps[len(s.Turns[0].Steps)-1].ID)
		if err != nil || len(before.Files) == 0 || before.FileMap()["b.txt"].Hash == "" {
			t.Fatalf("%s: the files before going back weren't kept: %v", what, err)
		}
	}
}

func withoutSets(files map[string]string) map[string]string {
	out := map[string]string{}
	for p, c := range files {
		if !isSet(p) {
			out[p] = c
		}
	}
	return out
}

// R3V stopped while making a step: no step, nothing broken; the next makes
// it.
func TestTakeStepKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills steps")
	}
	r := stepsProject(t)
	write(t, r.Root, "big.bin", string(randomBytes(3, 300<<10)))
	if !runKilled(t, nil, "step", "", 0, "R3V_KILL_DIR="+r.Root, "R3V_KILL_STAGE="+StageStoring, "R3V_KILL_DONE=0") {
		t.Fatal("it finished")
	}
	r, _ = Open(r.Root)
	if s, err := r.SessionSteps("s1"); err == nil && s.Steps != 0 {
		t.Fatalf("a step half made: %+v", s)
	}
	if st, err := r.TakeStep("s1", "Write", ""); err != nil || st == nil || st.N != 1 {
		t.Fatalf("again: %+v %v", st, err)
	}
}

// A version's steps: those made on its parent before it, under their
// prompts; the steps after it belong to the next.
func TestVersionSteps(t *testing.T) {
	r := stepsProject(t)
	r.StepTurn("s1", "Warmer light", "")
	write(t, r.Root, "notes.txt", "two")
	r.TakeStep("s1", "Write", "")
	write(t, r.Root, "notes.txt", "three")
	r.TakeStep("s1", "Edit", "")
	time.Sleep(1100 * time.Millisecond) // (times are to the second)
	v := mustSnapshot(t, r, "second")
	write(t, r.Root, "notes.txt", "four")
	r.TakeStep("s1", "Edit", "")
	got, err := r.VersionSteps(v.ID)
	if err != nil || len(got) != 1 || got[0].Steps != 2 || got[0].Turns[0].Prompt != "Warmer light" {
		t.Fatalf("version's steps: %+v %v", got, err)
	}
	first, _ := r.Log()
	if s, _ := r.VersionSteps(first[len(first)-1].ID); len(s) != 0 {
		t.Fatalf("the first version's: %+v", s)
	}
}

// A merge was made of what it brought in: its steps are those of the
// versions on the lane it merged.
func TestMergeVersionSteps(t *testing.T) {
	withSteps(t)
	a, _ := codeTeam(t)
	save := func(msg string) {
		t.Helper()
		time.Sleep(1100 * time.Millisecond) // (times are to the second)
		if _, _, err := a.Save(msg, Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	write(t, a.Root, "main.txt", "one")
	save("main")
	if err := a.CreateBranch("lane"); err != nil {
		t.Fatal(err)
	}
	a.StepTurn("s1", "Lane work", "")
	write(t, a.Root, "lane.txt", "a")
	a.TakeStep("s1", "Write", "")
	write(t, a.Root, "lane.txt", "b")
	a.TakeStep("s1", "Edit", "")
	save("lane: two steps")
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "main.txt", "two")
	save("main moves on")
	if _, err := a.MergeBranch("lane", "", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	log, _ := a.Log()
	if len(log[0].Parents) != 2 {
		t.Fatalf("not a merge: %+v", log[0])
	}
	got, err := a.VersionSteps(log[0].ID)
	if err != nil || len(got) != 1 || got[0].Steps != 2 || got[0].Turns[0].Prompt != "Lane work" {
		t.Fatalf("the merge's steps: %+v %v", got, err)
	}
	if s, _ := a.VersionSteps(log[1].ID); len(s) != 0 { // (main went on from where the lane began)
		t.Fatalf("main's own version: %+v", s)
	}
}
