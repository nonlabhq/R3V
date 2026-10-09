package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// parking turns Parking on for a test (as Nightly has it).
func parking(t *testing.T, on bool) {
	t.Helper()
	was := Parking
	Parking = on
	t.Cleanup(func() { Parking = was })
}

// changesOn makes a change of each kind: one changed, one added, one gone.
func changesOn(t *testing.T, r *Repo) {
	t.Helper()
	write(t, r.Root, "notes.txt", "mine")
	write(t, r.Root, "Samples/new.wav", string(randomBytes(21, 120<<10)))
	os.Remove(filepath.Join(r.Root, "Docs", "old.txt"))
	os.Remove(filepath.Join(r.Root, "Docs"))
}

func twoLocal(t *testing.T) (r *Repo, v1, v2 *Manifest) {
	t.Helper()
	r, _ = Init(newProject(t), "yi")
	write(t, r.Root, "notes.txt", "one")
	write(t, r.Root, "Docs/old.txt", "old")
	v1 = mustSnapshot(t, r, "v1")
	write(t, r.Root, "notes.txt", "two")
	v2 = mustSnapshot(t, r, "v2")
	if _, _, err := r.Checkout(v2.ID, false); err != nil { // (sets relinked, as a switch leaves them)
		t.Fatal(err)
	}
	return r, v1, v2
}

func TestParkedWhileOnAnOlderVersion(t *testing.T) {
	parking(t, true)
	r, v1, v2 := twoLocal(t)
	clean := files(t, r.Root)
	changesOn(t, r)
	mine := files(t, r.Root)

	// Away to v1: the changes wait on main's latest.
	if _, _, err := r.GoTo(v1.ID, false); err != nil {
		t.Fatal(err)
	}
	if r.Head() != v1.ID || fileOf(t, filepath.Join(r.Root, "notes.txt")) != "one" {
		t.Fatal("not on v1")
	}
	assertClean(t, r)
	sets := r.ParkedSets()
	if len(sets) != 1 || sets[0].Branch != "main" || sets[0].At != "" || sets[0].Base != v2.ID || sets[0].Files != 3 {
		t.Fatalf("parked: %+v", sets)
	}
	if r.Park == nil || r.Park.Parked == nil || r.Park.Restored != nil {
		t.Fatalf("outcome: %+v", r.Park)
	}

	// Changes made on v1 wait there when going back.
	write(t, r.Root, "notes.txt", "on one")
	if _, _, err := r.GoTo("latest", false); err != nil {
		t.Fatal(err)
	}
	if got := files(t, r.Root); !maps.Equal(got, mine) || r.Head() != v2.ID || r.OnOlderVersion() {
		t.Fatalf("back on the latest: %v", diffKeys(got, mine))
	}
	if r.Park.Restored == nil || r.Park.Parked == nil || r.Park.Parked.At != v1.ID {
		t.Fatalf("outcome: %+v", r.Park)
	}
	if p := r.ParkedAt("main", v1.ID); p == nil || r.ParkedAt("main", "") != nil {
		t.Fatalf("parked: %+v", r.ParkedSets())
	}

	// And come back with v1.
	write(t, r.Root, "notes.txt", "two") // (changes on the latest again: parked)
	if _, _, err := r.GoTo(v1.ID, false); err != nil {
		t.Fatal(err)
	}
	if fileOf(t, filepath.Join(r.Root, "notes.txt")) != "on one" {
		t.Fatal("v1's changes not back")
	}
	if p := r.ParkedAt("main", v1.ID); p != nil {
		t.Fatalf("brought back but still parked: %+v", p)
	}
	r2, _ := Open(r.Root) // as it is on disk
	if len(r2.ParkedSets()) != 1 {
		t.Fatalf("parked after reopening: %+v", r2.ParkedSets())
	}
	_ = clean
}

func TestParkingOffAsks(t *testing.T) {
	parking(t, false)
	r, v1, _ := twoLocal(t)
	changesOn(t, r)
	if _, _, err := r.GoTo(v1.ID, false); !errors.Is(err, ErrDirty) {
		t.Fatalf("going with changes: %v", err)
	}
	if len(r.ParkedSets()) != 0 {
		t.Fatal("parked")
	}
}

func TestParkedDiscardAndBringHere(t *testing.T) {
	parking(t, true)
	r, v1, _ := twoLocal(t)
	changesOn(t, r)
	mine := files(t, r.Root)
	r.GoTo(v1.ID, false)
	if err := r.DiscardParked("main", ""); err != nil || len(r.ParkedSets()) != 0 {
		t.Fatalf("discard: %v %+v", err, r.ParkedSets())
	}
	r.GoTo("latest", false)
	if fileOf(t, filepath.Join(r.Root, "notes.txt")) != "two" {
		t.Fatal("discarded changes came back")
	}

	// Brought here: onto v1 (as a branch made there would be). v1's notes
	// differ from the set's base: a choice, the set's taken.
	changesOn(t, r)
	r.GoTo(v1.ID, false)
	var mc *MergeConflictError
	if _, err := r.BringParked("main", "", Strategy("fail")); !errors.As(err, &mc) || !mc.Work {
		t.Fatalf("bringing it here: %v", err)
	}
	if fileOf(t, filepath.Join(r.Root, "notes.txt")) != "one" || r.ParkedAt("main", "") == nil {
		t.Fatal("a refused bring changed something")
	}
	if _, err := r.BringParked("main", "", Strategy("theirs")); err != nil {
		t.Fatal(err)
	}
	got := files(t, r.Root)
	if got["Samples/new.wav"] != mine["Samples/new.wav"] || got["notes.txt"] != "mine" || got["Docs/old.txt"] != "" {
		t.Fatalf("brought here: %v", fileNames(got))
	}
	if len(r.ParkedSets()) != 0 || r.Head() != v1.ID {
		t.Fatalf("after: %+v", r.ParkedSets())
	}
}

func TestParkedAcrossBranches(t *testing.T) {
	parking(t, true)
	a, b := team(t)
	if err := a.CreateBranch("idea"); err != nil {
		t.Fatal(err)
	}
	a.Checkout(a.Head(), false) // (sets relinked, as a switch leaves them)
	write(t, a.Root, "idea.txt", "on idea")
	ideaWork := files(t, a.Root)

	// To main: idea's changes wait.
	res, err := a.SwitchBranch("main", false)
	if err != nil || res.Park == nil || res.Park.Parked == nil || res.Park.Parked.Branch != "idea" {
		t.Fatalf("switch: %v %+v", err, res)
	}
	assertClean(t, a)
	write(t, a.Root, "main.txt", "on main")
	mainWork := files(t, a.Root)

	// Back and forth: each branch's changes come back.
	if _, err := a.SwitchBranch("idea", false); err != nil {
		t.Fatal(err)
	}
	if got := files(t, a.Root); !maps.Equal(got, ideaWork) {
		t.Fatalf("on idea: %v", diffKeys(got, ideaWork))
	}
	// A teammate shares on main meanwhile (another file): merged on the way back.
	write(t, b.Root, "theirs.txt", "alex")
	if _, _, err := b.Save("alex", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	res, err = a.SwitchBranch("main", false)
	if err != nil || res.Park.Restored == nil || !res.Park.Merged {
		t.Fatalf("back to main: %v %+v", err, res.Park)
	}
	got := files(t, a.Root)
	if got["main.txt"] != "on main" || got["theirs.txt"] != "alex" || got["idea.txt"] != "" || a.Head() != b.Head() {
		t.Fatalf("on main: %v", fileNames(got))
	}
	if a.ParkedAt("main", "") != nil {
		t.Fatal("main's set still parked")
	}
	_ = mainWork

	// A conflict: the set waits, the branch's files come.
	a.Snapshot("main work") // (local; shared later)
	if _, err := a.SwitchBranch("idea", false); !errors.Is(err, ErrUnshared) {
		t.Fatalf("unshared versions: %v", err)
	}
	if _, _, err := a.Save("main work", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "theirs.txt", "mine")
	if _, err := a.SwitchBranch("idea", false); err != nil {
		t.Fatal(err)
	}
	b.Update(Strategy("fail"))
	write(t, b.Root, "theirs.txt", "alex again")
	if _, _, err := b.Save("alex again", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	a.DiscardParked("idea", "") // (idea's changes: not the point here)
	res, err = a.SwitchBranch("main", false)
	if err != nil || res.Park.Waiting == nil || res.Park.Restored != nil {
		t.Fatalf("conflicting: %v %+v", err, res.Park)
	}
	if fileOf(t, filepath.Join(a.Root, "theirs.txt")) != "alex again" || a.ParkedAt("main", "") == nil {
		t.Fatal("the set should wait, the branch's files come")
	}
	// Leaving with new changes while it waits: refused, nothing changed.
	write(t, a.Root, "more.txt", "x")
	if _, err := a.SwitchBranch("idea", false); !errors.Is(err, ErrParkedHere) {
		t.Fatalf("leaving with a set waiting: %v", err)
	}
	os.Remove(filepath.Join(a.Root, "more.txt"))
	// Brought here, choosing: theirs kept.
	if _, err := a.BringParked("main", "", Strategy("theirs")); err != nil {
		t.Fatal(err)
	}
	if fileOf(t, filepath.Join(a.Root, "theirs.txt")) != "mine" || a.ParkedAt("main", "") != nil {
		t.Fatalf("brought here: %q", fileOf(t, filepath.Join(a.Root, "theirs.txt")))
	}
}

// Parked content stays here even when the team's storage has it.
func TestPruneKeepsParked(t *testing.T) {
	parking(t, true)
	a, _ := team(t)
	write(t, a.Root, "Samples/take.wav", string(randomBytes(31, 200<<10)))
	if err := a.CreateBranch("idea"); err != nil {
		t.Fatal(err)
	}
	a.SwitchBranch("main", false)
	p := a.ParkedAt("idea", "")
	if p == nil {
		t.Fatal("not parked")
	}
	m, _ := a.Load(p.Version)
	h := m.FileMap()["Samples/take.wav"].Hash
	c, _ := a.Client()
	if err := a.uploadObjects(c, []string{h}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PruneObjects(); err != nil {
		t.Fatal(err)
	}
	if !a.Store.Has(h) {
		t.Fatal("a parked file's content was pruned")
	}
}

// Going to a version with changes, killed while the files are rewritten:
// putting them back gives the changes as they were, not parked.
func TestParkedSwitchKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills switches")
	}
	parking(t, true)
	setup := func() (*Repo, *Manifest, map[string]string, map[string]string) {
		r, v1, _ := twoLocal(t)
		after := files(t, r.Root)
		after["notes.txt"], after["Docs/old.txt"] = "one", "old"
		write(t, r.Root, "Samples/new.wav", string(randomBytes(22, 200<<10)))
		write(t, r.Root, "notes.txt", "mine")
		write(t, r.Root, "Mix/bounce.wav", string(randomBytes(23, 200<<10)))
		os.Remove(filepath.Join(r.Root, "Docs", "old.txt"))
		os.Remove(filepath.Join(r.Root, "Docs"))
		delete(after, "Samples/new.wav")
		delete(after, "Mix/bounce.wav")
		return r, v1, files(t, r.Root), after
	}
	r, v1, _, _ := setup()
	steps := writes(func() {
		if _, _, err := r.GoTo(v1.ID, false); err != nil {
			t.Fatal(err)
		}
	})
	if steps < 2 {
		t.Fatalf("the switch wrote %d files", steps)
	}
	for n := 1; n <= steps; n++ {
		what := fmt.Sprintf("killed after %d of %d files", n, steps)
		r, v1, before, after := setup()
		head := r.Head()
		if !runKilled(t, nil, "goto-parking", "", 0, "R3V_KILL_DIR="+r.Root, "R3V_KILL_TO="+v1.ID,
			fmt.Sprintf("R3V_KILL_WRITES=%d", n)) {
			t.Fatalf("%s: it finished", what)
		}
		r, err := Open(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		eitherOf(t, what, files(t, r.Root), before, after)
		if r.UnfinishedSwitch() == "" {
			t.Fatalf("%s: the switch isn't known to have stopped", what)
		}
		if _, err := r.RecoverSwitch(); err != nil {
			t.Fatalf("%s: putting the files back: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, before) || r.Head() != head || r.OnOlderVersion() {
			t.Fatalf("%s: after putting the files back:\n%v", what, fileNames(got))
		}
		if sets := r.ParkedSets(); len(sets) != 0 {
			t.Fatalf("%s: still parked: %+v", what, sets)
		}
		if _, _, err := r.GoTo(v1.ID, false); err != nil {
			t.Fatalf("%s: going again: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, after) || len(r.ParkedSets()) != 1 {
			t.Fatalf("%s: after going again:\n%v", what, fileNames(got))
		}
	}
}

func diffKeys(a, b map[string]string) (out []string) {
	for k, v := range a {
		if b[k] != v {
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

// Stopped after parking, before the files changed: the record goes, the
// changes are simply there.
func TestParkedStoppedBeforeTheFiles(t *testing.T) {
	parking(t, true)
	r, v1, _ := twoLocal(t)
	changesOn(t, r)
	mine := files(t, r.Root)
	if p, err := r.parkChanges(); err != nil || p == nil {
		t.Fatalf("park: %v %v", p, err)
	}
	r, _ = Open(r.Root)
	r.TidyParked()
	if len(r.ParkedSets()) != 0 {
		t.Fatalf("still parked: %+v", r.ParkedSets())
	}
	if _, _, err := r.GoTo(v1.ID, false); err != nil {
		t.Fatal(err)
	}
	r.GoTo("latest", false)
	if got := files(t, r.Root); !maps.Equal(got, mine) {
		t.Fatalf("after going and back: %v", diffKeys(got, mine))
	}
}
