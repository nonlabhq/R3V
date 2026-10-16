package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A merge's preview comes with its description written from what comes in
// (unfinished versions marked and left out); merged with it emptied, the
// plain "Merge branch …".
func TestMergePreviewMessage(t *testing.T) {
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
	for i, title := range []string{"Add bloom pass", "WIP glow"} {
		os.WriteFile(filepath.Join(root, "Samples", "fx.wav"), []byte{'R', byte(i)}, 0o644)
		if _, err := a.Save(root, title, true, nil, true, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.SwitchBranch(root, "main", true); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "Samples", "main.wav"), []byte("RIFF-main"), 0o644)
	if _, err := a.Save(root, "on main", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	p, err := a.PreviewMerge(root, key)
	if err != nil || p.Message != "Merge vfx: 2 versions\n- Add bloom pass" || len(p.Versions) != 2 {
		t.Fatalf("preview: %v %+v", err, p)
	}
	for _, v := range p.Versions {
		if v.WIP != (v.Message == "WIP glow") {
			t.Fatalf("wip: %+v", p.Versions)
		}
	}
	if _, err := a.MergeBranch(root, key, " ", nil, true); err != nil {
		t.Fatal(err)
	}
	r, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := r.Load(r.Head()); m.Message != "Merge branch vfx" {
		t.Fatalf("emptied: %q", m.Message)
	}
}
