package project

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

// deletedOnHostedTeam is a project shared with a hosted team, then deleted
// from it from elsewhere (another copy moved to another team, or deleted
// from the team's page): the hosted team keeps it a while, out of reach,
// and this copy still points at it, with a version not shared yet.
func deletedOnHostedTeam(t *testing.T) (*cloudtest.Fake, *Repo, remote.Backend) {
	t.Helper()
	f := cloudtest.NewFake(t)
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(f.Address); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	c, err := a.Client()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteProject(a.Config.ProjectID); err != nil {
		t.Fatal(err)
	}
	if !f.Deleted(a.Config.ProjectID) {
		t.Fatal("the fake should keep the deleted project")
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"verse\n")
	if _, err := a.Snapshot("verse"); err != nil {
		t.Fatal(err)
	}
	return f, a, c
}

// Sharing a copy of a project its hosted team deleted brings it back (an
// admin's, while the team keeps it) and shares what's new: nothing of the
// team's or of this copy is lost.
func TestShareRestoresProjectDeletedFromHostedTeam(t *testing.T) {
	f, a, c := deletedOnHostedTeam(t)
	res, err := a.Share(Strategy("fail"))
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if !res.Restored || res.Action != "published" {
		t.Errorf("result: %+v", res)
	}
	pid := a.Config.ProjectID
	if f.Deleted(pid) {
		t.Error("still deleted on the team")
	}
	if ps, _ := c.Projects(); len(ps) != 1 || ps[0].ID != pid {
		t.Errorf("the team's projects: %+v", ps)
	}
	if bs, _ := c.Branches(pid); bs["main"] != a.Head() {
		t.Errorf("the team's main: %q, here %q", bs["main"], a.Head())
	}
	if hs, _ := a.Log(); len(hs) != 2 {
		t.Errorf("history here: %d versions", len(hs))
	}
}

// The next commit does the same.
func TestSaveRestoresProjectDeletedFromHostedTeam(t *testing.T) {
	f, a, c := deletedOnHostedTeam(t)
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"verse\nchorus\n")
	m, res, err := a.Save("chorus", Strategy("fail"))
	if err != nil || m == nil {
		t.Fatalf("commit: %v", err)
	}
	if !res.Restored || f.Deleted(a.Config.ProjectID) {
		t.Errorf("not brought back: %+v", res)
	}
	if bs, _ := c.Branches(a.Config.ProjectID); bs["main"] != m.ID {
		t.Errorf("the team's main: %q, the commit %q", bs["main"], m.ID)
	}
}

// Stopped at either point (the service unreachable before it's back;
// brought back and the share stopped there), the next share finishes.
func TestShareRestoringStoppedHalfWay(t *testing.T) {
	f, a, c := deletedOnHostedTeam(t)
	pid := a.Config.ProjectID
	f.Down.Store(true)
	if _, err := a.Share(Strategy("fail")); err == nil {
		t.Fatal("shared with the service down")
	}
	f.Down.Store(false)
	if !f.Deleted(pid) {
		t.Fatal("brought back with the service down")
	}
	// Brought back, nothing shared yet.
	if err := remote.RestoreProject(c, pid); err != nil {
		t.Fatal(err)
	}
	res, err := a.Share(Strategy("fail"))
	if err != nil || res.Action != "published" || res.Restored {
		t.Fatalf("share: %+v %v", res, err)
	}
	if bs, _ := c.Branches(pid); bs["main"] != a.Head() {
		t.Errorf("the team's main: %q, here %q", bs["main"], a.Head())
	}
}

// A member who may not bring it back is told what happened, with the
// versions kept here.
func TestShareProjectDeletedFromHostedTeamAsMember(t *testing.T) {
	f, a, _ := deletedOnHostedTeam(t)
	f.AddMember("member", "0000000000000000000000000000000b", false)
	t.Setenv("R3V_CLOUD_TOKEN", "member")
	head := a.Head()
	_, err := a.Share(Strategy("fail"))
	if !errors.Is(err, ErrProjectDeleted) {
		t.Fatalf("share: %v", err)
	}
	if a.Head() != head || !f.Deleted(a.Config.ProjectID) {
		t.Error("nothing should have changed")
	}
}
