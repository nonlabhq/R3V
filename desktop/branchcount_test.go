package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A branch's own versions: those made on it, and older ones (no branch
// said) down its first parents until another branch's.
func TestBranchVersionCounts(t *testing.T) {
	v := func(id, branch string, parents ...string) *project.Manifest {
		return &project.Manifest{ID: id, Branch: branch, Parents: parents}
	}
	// o1 <- o2 (older, no branch said) <- m1 <- m2 (main) <- merge (main, m2 + f2)
	//        \- f1 <- f2 (feature, made from o2); x1: older, made from o2 on "old"
	byID := map[string]*project.Manifest{}
	for _, m := range []*project.Manifest{v("o1", ""), v("o2", "", "o1"), v("m1", "main", "o2"), v("m2", "main", "m1"),
		v("f1", "feature", "o2"), v("f2", "feature", "f1"), v("mg", "main", "m2", "f2"), v("x1", "", "o2")} {
		byID[m.ID] = m
	}
	branches := []project.BranchInfo{{Name: "feature", Head: "f2", Current: true}, {Name: "main", Head: "mg"}, {Name: "old", Head: "x1"}}
	// main: mg, m2, m1 and the older o2, o1 under them; feature from where it
	// started (f1, f2); old: x1 only (o2 is main's).
	if got := branchVersionCounts(byID, branches); got["main"] != 5 || got["feature"] != 2 || got["old"] != 1 {
		t.Errorf("counts: %v", got)
	}

	// All older (none says its branch): main claims its first parents first.
	byID = map[string]*project.Manifest{}
	for _, m := range []*project.Manifest{v("a", ""), v("b", "", "a"), v("c", "", "b"), v("d", "", "b"), v("e", "", "d")} {
		byID[m.ID] = m
	}
	got := branchVersionCounts(byID, []project.BranchInfo{{Name: "lamp", Head: "e", Current: true}, {Name: "main", Head: "c"}})
	if got["main"] != 3 || got["lamp"] != 2 {
		t.Errorf("older versions: %v", got)
	}
}

// The project's branch list says how many versions each has.
func TestBranchListVersions(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("one")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	one, _ := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	root := newSong(t)
	if _, err := a.AddProjectToTeam(one.ID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	key, err := a.CreateBranch(root, "vfx", "", true)
	if err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"bloom", "glow", "haze"} {
		os.WriteFile(filepath.Join(root, "Samples", "fx.wav"), []byte{'R', byte(i)}, 0o644)
		if _, err := a.Save(root, title, true, nil, true, nil); err != nil {
			t.Fatal(err)
		}
	}
	l, err := a.BranchList(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, b := range l.Branches {
		got[b.Name] = b.Versions
	}
	main := got["main"]
	if got[key] != 3 || main < 1 {
		t.Fatalf("versions: %v", got)
	}
	// (asked again with nothing new: the same, from the cache)
	if l, _ := a.BranchList(root); len(l.Branches) != 2 {
		t.Fatalf("again: %+v", l)
	}
	st, err := a.TeamState(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range st.Branches {
		if b.Versions != got[b.Name] {
			t.Errorf("state's %s: %d, list's %d", b.Name, b.Versions, got[b.Name])
		}
	}
}
