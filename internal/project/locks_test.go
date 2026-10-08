package project

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// storedVersion stores a version with these files (path -> content) after
// parents, for the changed-paths tests.
func storedVersion(t *testing.T, r *Repo, files map[string]string, parents ...string) string {
	t.Helper()
	m := &Manifest{Version: manifest.Format, Parents: parents, Author: "yi", Time: time.Now().UTC().Format(time.RFC3339),
		Message: "v"}
	for p, c := range files {
		s := sha256.Sum256([]byte(c))
		m.Files = append(m.Files, FileEntry{Path: p, Hash: hex.EncodeToString(s[:]), Size: int64(len(c))})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	if err := r.save(m); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

// merged stores the merge of ours and theirs (base: their common one),
// decided by opts.
func merged(t *testing.T, r *Repo, base, ours, theirs string, opts MergeOptions) string {
	t.Helper()
	b, _ := r.Load(base)
	o, _ := r.Load(ours)
	th, _ := r.Load(theirs)
	m, _, err := r.mergeManifests(b, o, th, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.save(m); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestChangedPaths(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".r3v.yaml", "presets:\n  ./: none\n")
	r, err := Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	changed := func(head string, shared ...string) []string {
		t.Helper()
		got, err := r.changedPaths(head, shared)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	want := func(what string, got []string, w ...string) {
		t.Helper()
		if w == nil {
			w = []string{}
		}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: %v, want %v", what, got, w)
		}
	}

	v0 := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "0", "Hero.uasset": "0", "notes.txt": "0"})
	want("a first share: every path", changed(v0), "Hero.uasset", "Maps/Harbor.umap", "notes.txt")
	want("nothing new", changed(v0, v0))

	// main: a teammate's version (shared); the branch: mine (shared).
	t1 := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "1", "Hero.uasset": "0", "notes.txt": "0"}, v0)
	o1 := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "0", "Hero.uasset": "1", "notes.txt": "0"}, v0)
	want("my version", changed(o1, v0), "Hero.uasset")
	want("creating a branch at a shared version", changed(t1, t1, o1))

	// Merging main into the branch brings the teammate's change: not mine.
	m1 := merged(t, r, v0, o1, t1, Strategy("fail"))
	want("merge from main", changed(m1, t1, o1))

	// My own version not shared yet, then main merged in: only mine counts.
	o2 := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "0", "Hero.uasset": "1", "notes.txt": "2"}, o1)
	m2 := merged(t, r, v0, o2, t1, Strategy("fail"))
	want("mine under a merge from main", changed(m2, t1, o1), "notes.txt")

	// Both changed the map: my change counts whichever side the merge kept;
	// keeping both adds a path of new content.
	o3 := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "3", "Hero.uasset": "1", "notes.txt": "0"}, o1)
	want("conflict kept mine", changed(merged(t, r, v0, o3, t1, Strategy("ours")), t1, o1), "Maps/Harbor.umap")
	want("conflict took theirs", changed(merged(t, r, v0, o3, t1, Strategy("theirs")), t1, o1), "Maps/Harbor.umap")
	want("conflict kept both", changed(merged(t, r, v0, o3, t1, Strategy("both")), t1, o1),
		"Maps/Harbor (theirs).umap", "Maps/Harbor.umap")

	// A merge whose result is neither side's (text merged line by line)
	// gives new content even when neither of my versions is new.
	mx := storedVersion(t, r, map[string]string{"Maps/Harbor.umap": "1", "Hero.uasset": "1", "notes.txt": "merged"}, o1, t1)
	want("a merge's own content", changed(mx, t1, o1), "notes.txt")

	// Deleting and moving count, at both paths.
	o4 := storedVersion(t, r, map[string]string{"Maps/Harbor2.umap": "0", "notes.txt": "0"}, o1)
	want("deleted and moved", changed(o4, o1), "Hero.uasset", "Maps/Harbor.umap", "Maps/Harbor2.umap")

	// A head this copy doesn't have: nothing of it is taken as the team's.
	want("unknown shared head", changed(o2, o1, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), "notes.txt")
}
