//go:build nightly

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// undo-merge makes a version without what the merge brought in; it stops
// for uncommitted changes and for a version that isn't a merge, and a file
// changed since is a conflict decided as a merge's.
func TestUndoMergeCommand(t *testing.T) {
	a := laneProject(t)
	if exit, rep := run(t, a, "merge", "vfx"); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}
	var lg struct{ Versions []versionJSON }
	_, rep := run(t, a, "log", "-n", "1")
	json.Unmarshal(rep.Result, &lg)
	merge := lg.Versions[0].ID

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("not committed"), 0o644)
	if exit, rep := run(t, a, "undo-merge", merge); exit != exitError || rep.Error.Code != "unsaved_changes" {
		t.Fatalf("with changes: %d %+v", exit, rep.Error)
	}
	os.Remove(filepath.Join(a, "notes.txt"))
	if exit, rep := run(t, a, "undo-merge", "HEAD~1"); exit != exitError || rep.Error.Code != "not_a_merge" {
		t.Fatalf("not a merge: %d %+v", exit, rep.Error)
	}

	// Changed since the merge: a conflict, then decided.
	os.WriteFile(filepath.Join(a, "src", "vfx.ts"), []byte("changed on main"), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "Fix bloom on main"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "undo-merge", merge); exit != exitConflict || rep.Error.Code != "merge_conflict" ||
		len(rep.Error.Conflicts) != 1 {
		t.Fatalf("conflict: %d %+v", exit, rep.Error)
	}
	exit, rep := run(t, a, "undo-merge", merge, "--strategy", "theirs")
	var out syncJSON
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Action != "published" || out.Saved == nil || !strings.HasPrefix(out.Saved.Message, "Undo merge ") {
		t.Fatalf("undo-merge: %d %s %+v", exit, rep.Result, rep.Error)
	}
	if _, err := os.Stat(filepath.Join(a, "src", "vfx.ts")); !os.IsNotExist(err) {
		t.Fatalf("vfx.ts still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a, "src", "main2.ts")); err != nil {
		t.Fatalf("main's own file went: %v", err)
	}
	if exit, rep := run(t, a, "undo-merge", merge); exit != exitError || rep.Error.Code != "nothing_to_undo" {
		t.Fatalf("again: %d %+v", exit, rep.Error)
	}
}
