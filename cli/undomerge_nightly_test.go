//go:build nightly

package cli

import (
	"encoding/json"
	"flag"
	"maps"
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
	exit, rep := run(t, a, "undo-merge", merge)
	if exit != exitConflict || rep.Error.Code != "merge_conflict" || len(rep.Error.Conflicts) != 1 ||
		!strings.Contains(rep.Error.Hint, "--resolve <key>=keep|undo") {
		t.Fatalf("conflict: %d %+v", exit, rep.Error)
	}
	c := rep.Error.Conflicts[0]
	if len(c.Choices) < 2 || c.Choices[0] != "keep" || c.Choices[1] != "undo" {
		t.Fatalf("choices named %v", c.Choices)
	}
	if exit, rep := run(t, a, "undo-merge", merge, "--resolve", c.Key+"=back"); exit != exitUsage ||
		!strings.Contains(rep.Error.Message, "keep, undo or both") {
		t.Fatalf("a choice that isn't: %d %+v", exit, rep.Error)
	}
	exit, rep = run(t, a, "undo-merge", merge, "--resolve", c.Key+"=undo")
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

// undo-merge's keep and undo are ours and theirs; the old words still work,
// and merge's own choices have no aliases.
func TestUndoChoices(t *testing.T) {
	strategy := "keep"
	fs := flag.NewFlagSet("undo-merge", flag.ContinueOnError)
	d := decisionFlagsAs(fs, &strategy, "keep|undo|both", undoChoices)
	if err := fs.Parse([]string{"--resolve", "file:a.txt=undo", "--resolve", "file:b.txt=keep",
		"--resolve", "file:c.txt=theirs", "--resolve", "file:d.txt=both"}); err != nil {
		t.Fatal(err)
	}
	opts, err := d.options(nil)
	want := map[string]string{"file:a.txt": "theirs", "file:b.txt": "ours", "file:c.txt": "theirs", "file:d.txt": "both"}
	if err != nil || opts.Strategy != "ours" || !maps.Equal(opts.Resolutions, want) {
		t.Fatalf("%v %q %v", err, opts.Strategy, opts.Resolutions)
	}
	strategy = "undo"
	if opts, _ := d.options(nil); opts.Strategy != "theirs" {
		t.Fatalf("--strategy undo: %q", opts.Strategy)
	}
	mergeStrategy := "keep"
	m := decisionFlags(flag.NewFlagSet("merge", flag.ContinueOnError), &mergeStrategy)
	if opts, _ := m.options(nil); opts.Strategy != "keep" { // (refused by the merge as any unknown word)
		t.Fatalf("merge took keep as %q", opts.Strategy)
	}
	m.resolve = stringList{"file:a.txt=undo"}
	if _, err := m.options(nil); err == nil || !strings.Contains(err.Error(), "ours, theirs or both") {
		t.Fatalf("merge --resolve undo: %v", err)
	}
}
