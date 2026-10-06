package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// A commit writes trees only for folders that changed.
func TestCommitWritesChangedFoldersOnly(t *testing.T) {
	root := newProject(t)
	r, _ := Init(root, "yi")
	for i := range 5 {
		dir := filepath.Join(root, "Stems", string(rune('A'+i)))
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "take.wav"), []byte{byte(i)}, 0o644)
	}
	mustSnapshot(t, r, "first")
	count := func() int {
		n := 0
		filepath.WalkDir(filepath.Join(r.Dir, treesDir), func(_ string, d os.DirEntry, _ error) error {
			if d != nil && !d.IsDir() {
				n++
			}
			return nil
		})
		return n
	}
	before := count()
	os.WriteFile(filepath.Join(root, "Stems", "C", "take.wav"), []byte("changed"), 0o644)
	m := mustSnapshot(t, r, "second")
	if added := count() - before; added != 3 { // Stems/C, Stems, the top
		t.Fatalf("%d new trees", added)
	}
	// Trees are read back as they were written.
	var n int
	if err := r.walkTrees([]string{m.Tree}, func(_ string, es []manifest.TreeEntry) error {
		n += len(es)
		return nil
	}); err != nil || n == 0 {
		t.Fatal(err)
	}
}

// The stat cache (index.bin) reads back what was saved.
func TestIndexBinary(t *testing.T) {
	root := newProject(t)
	r, _ := Init(root, "yi")
	mustSnapshot(t, r, "first")
	ix := r.loadIndex()
	if len(ix.entries) == 0 {
		t.Fatal("empty index")
	}
	want := ix.entries
	ix.dirty = true
	if err := ix.save(); err != nil {
		t.Fatal(err)
	}
	got := r.loadIndex().entries
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: %+v, want %+v", k, got[k], v)
		}
	}
	// A damaged file: start over (files are hashed again), no error.
	os.WriteFile(filepath.Join(r.Dir, indexFile), []byte("R3V-INDEX-1\n\x05ab"), 0o644)
	if n := len(r.loadIndex().entries); n != 0 {
		t.Fatalf("damaged index gave %d entries", n)
	}
	assertClean(t, r)
}
