package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// A version's change count is what its changes list (a move one change),
// worked out once: kept in the project, read from there by a later open
// (with the trees gone, nothing could be worked out again); a little time
// counts some, a later call the rest.
func TestChangeCounts(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	write(t, r.Root, "notes/a.txt", "one")
	write(t, r.Root, "notes/b.txt", "two")
	first := mustSnapshot(t, r, "first")
	write(t, r.Root, "notes/a.txt", "one, changed")
	os.Remove(filepath.Join(r.Root, "notes", "b.txt"))
	write(t, r.Root, "notes/deep/c.txt", "three")
	second := mustSnapshot(t, r, "second")
	move(t, r.Root, "notes/deep/c.txt", "moved/c.txt")
	third := mustSnapshot(t, r, "third")
	ids := []string{third.ID, second.ID, first.ID}

	at := time.Now()
	countClock = func() time.Time { at = at.Add(time.Millisecond); return at }
	got := r.ChangeCounts(ids, time.Millisecond)
	countClock = time.Now
	if len(got) == len(ids) {
		t.Fatalf("all counted in no time: %v", got)
	}
	got = r.ChangeCounts(ids, 0)
	for _, id := range ids {
		changes, err := r.VersionChanges(id)
		if err != nil {
			t.Fatal(err)
		}
		if got[id] != len(changes) {
			t.Errorf("%s: counted %d, lists %d", short(id), got[id], len(changes))
		}
	}
	if got[second.ID] != 3 || got[third.ID] != 1 {
		t.Fatalf("counts: %v", got)
	}

	forgetCounts()
	os.RemoveAll(filepath.Join(r.CommonDir(), treesDir))
	trees.mu.Lock()
	trees.m, trees.entries = map[string][]manifest.TreeEntry{}, 0
	trees.mu.Unlock()
	again, _ := Open(r.Root)
	if kept := again.ChangeCounts(ids, 0); len(kept) != 3 || kept[third.ID] != 1 {
		t.Fatalf("counted again, not read: %v", kept)
	}
}

// A read-only open (another program holds the project) counts, and keeps
// nothing; the next open that may write keeps what it counted.
func TestChangeCountsReadOnly(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	m := mustSnapshot(t, r, "first")
	ro, _ := Open(r.Root)
	ro.ReadOnly = true
	if got := ro.ChangeCounts([]string{m.ID}, 0); got[m.ID] == 0 {
		t.Fatalf("not counted: %v", got)
	}
	if _, err := os.Stat(r.changedPath()); !os.IsNotExist(err) {
		t.Fatalf("a read-only open kept the counts: %v", err)
	}
	r.ChangeCounts([]string{m.ID}, 0)
	if kept := readCounts(r.changedPath()); kept[m.ID] == 0 {
		t.Fatalf("not kept: %v", kept)
	}
}

// forgetCounts drops the counts kept in memory (a new run of the app).
func forgetCounts() {
	counts.Lock()
	counts.by = map[string]*projectCounts{}
	counts.Unlock()
}
