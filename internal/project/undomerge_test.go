package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mergedLane: a shared first version; on branch vfx, a version with the
// changes sharedProject makes; on main, another version; then vfx merged
// into main. It returns the project (on main), the merge, and the files
// before and after the merge.
func mergedLane(t *testing.T, code string) (a *Repo, merge string, before, after map[string]string) {
	t.Helper()
	a, _, _ = sharedProject(t, code, "Song")
	if err := a.CreateBranch("vfx"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Save("vfx work", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "main.txt", "on main")
	if _, _, err := a.Save("main moves on", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	before = files(t, a.Root)
	if _, err := a.MergeBranch("vfx", "", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	return a, a.Head(), before, files(t, a.Root)
}

// Undoing a merge makes a version without what it brought in; the history
// stays (the merge is still in it), and again there's nothing to undo.
func TestUndoMerge(t *testing.T) {
	_, code := cutTeam(t)
	a, merge, before, _ := mergedLane(t, code)
	m, res, err := a.UndoMerge(merge, "", Strategy("fail"))
	if err != nil || res.Action != "published" {
		t.Fatalf("undo: %v %+v", err, res)
	}
	if !maps.Equal(files(t, a.Root), before) {
		t.Fatalf("files: %v", fileNames(files(t, a.Root)))
	}
	if len(m.Parents) != 1 || m.Parents[0] != merge || !strings.HasPrefix(m.Message, "Undo merge "+merge[:10]+` ("Merge vfx: 1 version`) {
		t.Fatalf("undo version: %v %q", m.Parents, m.Message)
	}
	assertClean(t, a)
	if got := cloneFiles(t, code, a.Config.Name); !maps.Equal(got, before) {
		t.Fatalf("a teammate gets: %v", fileNames(got))
	}
	if _, _, err := a.UndoMerge(merge, "", Strategy("fail")); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("again: %v", err)
	}
	first, _ := a.Load(merge)
	if _, _, err := a.UndoMerge(first.Parents[0], "", Strategy("fail")); !errors.Is(err, ErrNotAMerge) {
		t.Fatalf("not a merge: %v", err)
	}
}

// Changed again after the merge: another file stays as it is now; the
// same file is a conflict, decided as a merge's.
func TestUndoMergeAfterLaterChanges(t *testing.T) {
	_, code := cutTeam(t)
	a, merge, before, _ := mergedLane(t, code)
	write(t, a.Root, "main.txt", "changed after the merge")
	write(t, a.Root, "notes.txt", "changed after the merge too")
	if _, _, err := a.Save("later", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	var mc *MergeConflictError
	if _, _, err := a.UndoMerge(merge, "", Strategy("fail")); !errors.As(err, &mc) ||
		len(mc.Conflicts) != 1 || mc.Conflicts[0].File != "notes.txt" {
		t.Fatalf("undo: %v", err)
	}
	if _, _, err := a.UndoMerge(merge, "", Strategy("ours")); err != nil {
		t.Fatal(err)
	}
	want := maps.Clone(before)
	want["main.txt"], want["notes.txt"] = "changed after the merge", "changed after the merge too"
	if got := files(t, a.Root); !maps.Equal(got, want) {
		t.Fatalf("files: %v", fileNames(got))
	}
}

// Uncommitted changes stop it, the files untouched.
func TestUndoMergeRefusesUncommitted(t *testing.T) {
	_, code := cutTeam(t)
	a, merge, _, _ := mergedLane(t, code)
	write(t, a.Root, "elsewhere.txt", "not committed")
	now := files(t, a.Root)
	if _, _, err := a.UndoMerge(merge, "", Strategy("fail")); !errors.Is(err, ErrDirty) {
		t.Fatalf("undo: %v", err)
	}
	if !maps.Equal(files(t, a.Root), now) || a.Head() != merge {
		t.Fatal("the files or the version changed")
	}
}

// An undo killed while it puts the files in place or at any point of its
// upload: the team has the merge or the undo, whole; the files are as
// they were (after the app's recovery) or as undone; undoing again
// finishes it without sending again what went up.
func TestUndoMergeKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills undos")
	}
	fake, code := cutTeam(t)
	a, merge, _, _ := mergedLane(t, code)
	put := fake.PutBytes
	steps := requests(fake, "write", func() {
		if _, _, err := a.UndoMerge(merge, "", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	})
	sent := fake.PutBytes - put
	type kill struct {
		writes, n int
		stage     string
	}
	// (while putting the files in place, and once they are, before the
	// commit: the undo uncommitted)
	kills := []kill{{writes: 1}, {writes: 2}, {stage: StageChecking}}
	for _, n := range killPoints(steps) {
		kills = append(kills, kill{n: n})
	}
	for _, k := range kills {
		what := fmt.Sprintf("killed at write %d of %d", k.n, steps)
		if k.writes > 0 {
			what = fmt.Sprintf("killed after putting %d file(s) in place", k.writes)
		} else if k.stage != "" {
			what = "killed while " + k.stage
		}
		fake, code := cutTeam(t)
		a, merge, before, after := mergedLane(t, code)
		put := fake.PutBytes
		if !runKilled(t, fake, "undo-merge", "write", k.n, "R3V_KILL_DIR="+a.Root, "R3V_KILL_TO="+merge,
			"R3V_KILL_WRITES="+strconv.Itoa(k.writes), "R3V_KILL_STAGE="+k.stage) {
			t.Fatalf("%s: it finished", what)
		}
		if got := cloneFiles(t, code, a.Config.Name); !maps.Equal(got, before) && !maps.Equal(got, after) {
			t.Fatalf("%s: a clone gets neither version:\n%v", what, fileNames(got))
		}
		r, err := Open(a.Root)
		if err != nil {
			t.Fatal(err)
		}
		if r.UnfinishedSwitch() != "" { // (the app puts the files back first)
			if changed, err := r.ChangedSinceSwitch(); err != nil || len(changed) != 0 {
				t.Fatalf("%s: changed since: %v %v", what, changed, err)
			}
			if _, err := r.RecoverSwitch(); err != nil {
				t.Fatalf("%s: recovering: %v", what, err)
			}
			if !maps.Equal(files(t, r.Root), after) {
				t.Fatalf("%s: recovered files: %v", what, fileNames(files(t, r.Root)))
			}
		}
		if got := files(t, r.Root); !maps.Equal(got, before) && !maps.Equal(got, after) {
			t.Fatalf("%s: the files are neither: %v", what, fileNames(got))
		}
		// (the last write may be after the undo is shared: then it's done)
		if _, _, err := r.UndoMerge(merge, "", Strategy("fail")); err != nil && !errors.Is(err, ErrNothingToUndo) {
			t.Fatalf("%s: undoing again: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, before) {
			t.Fatalf("%s: after undoing again: %v", what, fileNames(got))
		}
		if got := cloneFiles(t, code, a.Config.Name); !maps.Equal(got, before) {
			t.Fatalf("%s: after undoing again, a clone gets:\n%v", what, fileNames(got))
		}
		if both := fake.PutBytes - put; both > sent+64<<10 {
			t.Errorf("%s: sent %d KB in all, %d KB without being killed", what, both>>10, sent>>10)
		}
		if _, err := os.Stat(filepath.Join(r.Dir, switchingFile)); err == nil {
			t.Errorf("%s: a switch is still marked unfinished", what)
		}
	}
}
