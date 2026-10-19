package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

// shortScan: the teams of earlier tests are out of reach; looking for
// where a project went doesn't wait long for them.
func shortScan(t *testing.T) {
	was := project.MovedToWait
	project.MovedToWait = 2 * time.Second
	t.Cleanup(func() { project.MovedToWait = was })
}

type deletedReply struct {
	Error struct {
		Code    string
		Deleted *deletedJSON
	}
}

func deletedOf(t *testing.T, a string, args ...string) (int, deletedReply) {
	t.Helper()
	exit, rep := run(t, a, args...)
	var out deletedReply
	if exit != 0 && rep.Error != nil {
		out.Error.Code = rep.Error.Code
		raw, _ := json.Marshal(rep.Error)
		var e struct{ Deleted *deletedJSON }
		json.Unmarshal(raw, &e)
		out.Error.Deleted = e.Deleted
	}
	return exit, out
}

// A copy of a project its R3V Cloud team deleted: commits stay here
// (project_deleted), the owner's too; r3v project restore brings it back,
// a member's refused.
func TestCommitProjectDeletedFromCloudTeam(t *testing.T) {
	shortScan(t)
	f := cloudtest.NewFake(t)
	f.AddMember("kai", "0000000000000000000000000000000b", false)
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "hi"})
	ra, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(f.Address); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	c, err := ra.Client()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteProject(ra.Config.ProjectID); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("hi again"), 0o644)
	if exit, rep := deletedOf(t, a, "commit", "-m", "second"); exit == 0 || rep.Error.Code != "project_deleted" ||
		rep.Error.Deleted == nil || rep.Error.Deleted.Project == "" {
		t.Fatalf("the owner's commit: %d %+v", exit, rep.Error)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	if exit, rep := deletedOf(t, a, "project", "restore"); exit == 0 || rep.Error.Code != "project_deleted" ||
		rep.Error.Deleted == nil || !rep.Error.Deleted.NotAllowed {
		t.Fatalf("a member's restore: %d %+v", exit, rep.Error)
	}
	if !f.Deleted(ra.Config.ProjectID) {
		t.Fatal("brought back")
	}
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	exit, rep := run(t, a, "project", "restore")
	if exit != 0 {
		t.Fatalf("the owner's restore: %+v", rep.Error)
	}
	var got syncJSON
	if json.Unmarshal(rep.Result, &got) != nil || got.Action != "published" || !got.Restored {
		t.Fatalf("result: %s", rep.Result)
	}
	if f.Deleted(ra.Config.ProjectID) {
		t.Error("still deleted")
	}
}

// A copy left on a team by a move: told where it went, r3v remote --team
// points it there, and its next commit shares there.
func TestCopyLeftByAMoveFollowsByRemote(t *testing.T) {
	shortScan(t)
	wasMove := project.MoveProjects
	project.MoveProjects = true
	t.Cleanup(func() { project.MoveProjects = wasMove })
	fake := s3test.New("one", "two")
	t.Cleanup(fake.Close)
	code := func(b string) string {
		return remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/" + b + "/r3v", AccessKey: "key", SecretKey: "secret"})
	}
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "hi"})
	ra, _ := project.Init(a, "yi")
	if err := ra.SetRemote(code("one")); err != nil {
		t.Fatal(err)
	}
	store, _ := teams.Load()
	one := store.FindByURL(ra.Config.Remote.URL)
	one.MemberID, one.MemberName = "0000000000000000000000000000000a", "yi"
	store.Save()
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	rb, _, err := project.Clone(code("one"), ra.Config.Name, filepath.Join(t.TempDir(), "B", "Song Project"), "yi")
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

	os.WriteFile(filepath.Join(rb.Root, "notes.txt"), []byte("left behind"), 0o644)
	exit, rep := deletedOf(t, rb.Root, "commit", "-m", "left behind")
	if exit == 0 || rep.Error.Code != "project_deleted" || rep.Error.Deleted == nil ||
		rep.Error.Deleted.MovedTo != two.Name || !rep.Error.Deleted.MovedToHere {
		t.Fatalf("commit on the first team: %d %+v %+v", exit, rep.Error, rep.Error.Deleted)
	}
	if exit, text := runText(t, rb.Root, "remote", "--team", two.Name); exit != 0 {
		t.Fatalf("remote --team: %s", text)
	}
	if exit, rep := run(t, rb.Root, "commit", "-m", "again"); exit != 0 {
		t.Fatalf("commit on the team it moved to: %+v", rep.Error)
	}
	rb2, _ := project.Open(rb.Root)
	c, _ := rb2.Client()
	if bs, _ := c.Branches(rb2.Config.ProjectID); bs["main"] != rb2.Head() {
		t.Errorf("the team's main: %v, here %s", bs, rb2.Head())
	}
}
