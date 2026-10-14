package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
)

// Reading a project another program holds (an agent committing): at once,
// as it is, nothing written, the page told; read with its lock free, it's
// kept for the next start (LastState).
func TestStateBusyAndLastState(t *testing.T) {
	root := localSong(t)
	a := NewApp()
	if st, err := a.LastState(root); err != nil || st != nil {
		t.Fatalf("never read: %+v %v", st, err)
	}
	st, err := a.State(root)
	if err != nil || st.Busy {
		t.Fatalf("free: %+v %v", st, err)
	}
	last, _ := a.LastState(root)
	if last == nil || !last.Stale || last.Root != st.Root || last.Name != st.Name {
		t.Fatalf("kept: %+v", last)
	}

	other, _ := project.Open(root)
	release, err := other.Lock(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("changed while held"), 0o644)
	index := filepath.Join(root, ".r3v", "index.bin")
	before, _ := os.Stat(index)
	start := time.Now()
	st, err = a.State(root)
	if err != nil || !st.Busy || time.Since(start) > 3*time.Second {
		t.Fatalf("held: %+v %v in %v", st, err, time.Since(start))
	}
	if len(st.Changes) == 0 {
		t.Error("the change isn't seen")
	}
	if after, _ := os.Stat(index); before != nil && after != nil && !after.ModTime().Equal(before.ModTime()) {
		t.Error("the index was written while another program held the project")
	}
}
