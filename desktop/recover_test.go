package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
)

// A switch stopped halfway, then a file made: putting the files back would
// delete it, so the app lists it and asks; only then are they put back.
// Meanwhile no commit takes the half-written files.
func TestRecoverSwitchAsksAboutFilesChangedSince(t *testing.T) {
	root := localSong(t)
	liveHas(t, "")
	a := NewApp()
	r, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Snapshot("a"); err != nil {
		t.Fatal(err)
	}
	head := r.Head()
	writeFile(t, root, "x.wav", "x")
	b, err := r.Snapshot("b")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Checkout(head, false); err != nil {
		t.Fatal(err)
	}
	// Something in the way of x.wav: the switch stops.
	os.MkdirAll(filepath.Join(root, "x.wav", "in the way"), 0o755)
	if _, _, err := r.Checkout(b.ID, true); err == nil {
		t.Fatal("expected the switch to fail")
	}
	os.RemoveAll(filepath.Join(root, "x.wav"))
	writeFile(t, root, "mine.txt", "made after")

	if res, err := a.Save(root, "half", false, nil, false, nil); err != nil || res.Action != "switch-unfinished" {
		t.Fatalf("commit while unfinished: %+v %v", res, err)
	}
	res, err := a.RecoverSwitch(root, false, false)
	if err != nil || res.Action != "changed-since" || !slices.Equal(res.ChangedSince, []string{"mine.txt"}) {
		t.Fatalf("recover: %+v %v", res, err)
	}
	if read(t, filepath.Join(root, "mine.txt")) != "made after" {
		t.Fatal("the file changed since was touched")
	}
	if r, _ := project.Open(root); r.UnfinishedSwitch() != b.ID {
		t.Fatal("the switch isn't unfinished any more")
	}

	// The user agrees: put back, the file replaced.
	if res, err := a.RecoverSwitch(root, false, true); err != nil || res.Action != "moved" {
		t.Fatalf("recover, replacing: %+v %v", res, err)
	}
	r, _ = project.Open(root)
	if r.UnfinishedSwitch() != "" || r.Head() != head {
		t.Fatal("not put back")
	}
	if _, err := os.Stat(filepath.Join(root, "mine.txt")); !os.IsNotExist(err) {
		t.Fatalf("the file made after stays: %v", err)
	}
	writeFile(t, root, "mine.txt", "now")
	if res, err := a.Save(root, "mine", false, nil, false, nil); err != nil || res.Action != "saved-locally" {
		t.Fatalf("commit after: %+v %v", res, err)
	}
}

// Nothing changed since: put back at once, as before.
func TestRecoverSwitchNothingChangedSince(t *testing.T) {
	root := localSong(t)
	liveHas(t, "")
	a := NewApp()
	r, _ := project.Open(root)
	if _, err := r.Snapshot("a"); err != nil {
		t.Fatal(err)
	}
	head := r.Head()
	writeFile(t, root, "x.wav", "x")
	b, err := r.Snapshot("b")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Checkout(head, false); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "x.wav", "in the way"), 0o755)
	if _, _, err := r.Checkout(b.ID, true); err == nil {
		t.Fatal("expected the switch to fail")
	}
	os.RemoveAll(filepath.Join(root, "x.wav"))
	if res, err := a.RecoverSwitch(root, false, false); err != nil || res.Action != "moved" {
		t.Fatalf("recover: %+v %v", res, err)
	}
	if r, _ := project.Open(root); r.UnfinishedSwitch() != "" || r.Head() != head {
		t.Fatal("not put back")
	}
}
