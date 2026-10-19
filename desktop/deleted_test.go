package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A copy left on a team by a move, in the app: its commit stays here
// (action "deleted", saying where the project went, nothing done), then
// pointing it at that team and sharing puts the version on top of the
// team's main there.
func TestCopyLeftByAMoveInApp(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	was := project.MoveProjects
	project.MoveProjects = true
	t.Cleanup(func() { project.MoveProjects = was })
	fake := s3test.New("one", "two")
	t.Cleanup(fake.Close)
	code := func(b string) string {
		return remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/" + b + "/r3v", AccessKey: "key", SecretKey: "secret"})
	}
	dir := filepath.Join(t.TempDir(), "Game")
	writeFile(t, dir, ".r3v.yaml", "presets:\n  ./: none\n")
	writeFile(t, dir, "notes.txt", "hi\n")
	ra, err := project.Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(code("one")); err != nil {
		t.Fatal(err)
	}
	store, _ := teams.Load()
	store.FindByURL(ra.Config.Remote.URL).MemberID = "0000000000000000000000000000000a"
	store.Save()
	if _, _, err := ra.Save("first", project.MergeOptions{Strategy: "fail"}); err != nil {
		t.Fatal(err)
	}
	rb, _, err := project.Clone(code("one"), ra.Config.Name, filepath.Join(t.TempDir(), "B", "Game"), "yi")
	if err != nil {
		t.Fatal(err)
	}
	two, err := project.Connect(code("two"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.MoveToTeam(two, project.MoveOptions{}); err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	t.Cleanup(waitTidy) // (before the folders go: cleanups run last first)
	writeFile(t, rb.Root, "notes.txt", "left behind\n")
	res, err := a.Save(rb.Root, "left behind", false, nil, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "deleted" || res.Deleted == nil || res.Deleted.MovedTo != two.Name || res.Deleted.MovedToTeam != two.ID ||
		!res.Deleted.CanRestore {
		t.Fatalf("commit: %+v %+v", res, res.Deleted)
	}
	if err := a.FollowProject(rb.Root, two.ID); err != nil {
		t.Fatal(err)
	}
	if res, err := a.ShareVersions(rb.Root); err != nil || res.Action != "published" {
		t.Fatalf("share: %+v %v", res, err)
	}
	r, _ := project.Open(rb.Root)
	c, _ := r.Client()
	if bs, _ := c.Branches(r.Config.ProjectID); bs["main"] != r.Head() {
		t.Errorf("the new team's main: %v, here %s", bs, r.Head())
	}
	if got, _ := os.ReadFile(filepath.Join(rb.Root, "notes.txt")); string(got) != "left behind\n" {
		t.Errorf("the file here: %q", got)
	}
	a.stopWatch(rb.Root)
}
