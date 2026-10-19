package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

// quickScan: the teams of earlier tests (one store for the package, their
// fakes closed) are out of reach: movedTo doesn't wait long for them.
func quickScan(t *testing.T) {
	was := MovedToWait
	MovedToWait = 2 * time.Second
	t.Cleanup(func() { MovedToWait = was })
}

// deletedOnHostedTeam is a project shared with a hosted team, then deleted
// from it from elsewhere (another copy moved it to another team, or it was
// deleted for everyone): the hosted team keeps it a while, out of reach,
// and this copy still points at it, with a version not shared yet.
func deletedOnHostedTeam(t *testing.T) (*cloudtest.Fake, *Repo, remote.Backend) {
	t.Helper()
	quickScan(t)
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

// A share doesn't bring back a project its team deleted (on purpose: a
// stale copy, or an agent committing, mustn't): it says so, the versions
// stay here, the team as it was. Owners and admins included.
func TestShareProjectDeletedFromHostedTeam(t *testing.T) {
	f, a, _ := deletedOnHostedTeam(t)
	head := a.Head()
	_, err := a.Share(Strategy("fail"))
	var gone *DeletedError
	if !errors.Is(err, ErrProjectDeleted) || !errors.As(err, &gone) {
		t.Fatalf("share: %v", err)
	}
	if !strings.Contains(err.Error(), a.Config.Name+" was removed from ") || gone.NotAllowed {
		t.Errorf("message: %v", err)
	}
	if a.Head() != head || !f.Deleted(a.Config.ProjectID) {
		t.Error("nothing should have changed")
	}
	// The next commit too: committed here, not shared.
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"verse\nchorus\n")
	if m, _, err := a.Save("chorus", Strategy("fail")); !errors.Is(err, ErrProjectDeleted) || m == nil {
		t.Errorf("commit: %v %v", m, err)
	}
	if !f.Deleted(a.Config.ProjectID) {
		t.Error("brought back by a commit")
	}
}

// Asked for (RestoreDeleted), an owner's or admin's share brings it back
// whole and shares what's new: nothing of the team's or this copy's lost.
func TestShareRestoresDeletedProjectWhenAsked(t *testing.T) {
	f, a, c := deletedOnHostedTeam(t)
	a.RestoreDeleted = true
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

// Stopped at either point (the service unreachable before it's back;
// brought back and the share stopped there), the next share finishes.
func TestShareRestoringStoppedHalfWay(t *testing.T) {
	f, a, c := deletedOnHostedTeam(t)
	pid := a.Config.ProjectID
	a.RestoreDeleted = true
	f.Down.Store(true)
	if _, err := a.Share(Strategy("fail")); err == nil {
		t.Fatal("shared with the service down")
	}
	f.Down.Store(false)
	if !f.Deleted(pid) {
		t.Fatal("brought back with the service down")
	}
	// Brought back, nothing shared yet: an ordinary share, unasked.
	if err := remote.RestoreProject(c, pid); err != nil {
		t.Fatal(err)
	}
	a.RestoreDeleted = false
	res, err := a.Share(Strategy("fail"))
	if err != nil || res.Action != "published" || res.Restored {
		t.Fatalf("share: %+v %v", res, err)
	}
	if bs, _ := c.Branches(pid); bs["main"] != a.Head() {
		t.Errorf("the team's main: %q, here %q", bs["main"], a.Head())
	}
}

// A member who may not bring it back is told so, with the versions kept.
func TestRestoreDeletedProjectAsMember(t *testing.T) {
	f, a, _ := deletedOnHostedTeam(t)
	f.AddMember("member", "0000000000000000000000000000000b", false)
	t.Setenv("R3V_CLOUD_TOKEN", "member")
	head := a.Head()
	a.RestoreDeleted = true
	_, err := a.Share(Strategy("fail"))
	var gone *DeletedError
	if !errors.As(err, &gone) || !gone.NotAllowed {
		t.Fatalf("share: %v", err)
	}
	if a.Head() != head || !f.Deleted(a.Config.ProjectID) {
		t.Error("nothing should have changed")
	}
}

// staleCopy: a project shared with team A (code), moved to team two from
// copy a (as MoveToTeam does: copied there, deleted from A; old: as an
// older R3V did, its log not saying where), and copy b, still pointing at
// A, with one more version committed.
func staleCopy(t *testing.T, code string, two *teams.Team, old bool) (a, b *Repo) {
	t.Helper()
	quickScan(t)
	was := MoveProjects
	MoveProjects = true
	t.Cleanup(func() { MoveProjects = was })
	a, _ = Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	actAs(t, a, "0000000000000000000000000000000a")
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	b, _, err := Clone(code, a.Config.Name, filepath.Join(t.TempDir(), "B", "Song"), "yi")
	if err != nil {
		t.Fatal(err)
	}
	if old {
		if err := a.MoveToTeam(two, MoveOptions{Copy: true}); err != nil {
			t.Fatal(err)
		}
		c, _ := a.Client()
		if err := c.DeleteProject(a.Config.ProjectID); err != nil {
			t.Fatal(err)
		}
		if err := a.JoinTeam(two); err != nil {
			t.Fatal(err)
		}
	} else if err := a.MoveToTeam(two, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, "Notes/left.txt", "left behind\n")
	if _, err := b.Snapshot("left behind"); err != nil {
		t.Fatal(err)
	}
	return a, b
}

// followAndShare: b's share says where the project went; b follows it
// there and shares; the version arrives on top of the team's main, and
// what a shared there meanwhile (moved on) stays.
func followAndShare(t *testing.T, a, b *Repo, two *teams.Team, movedOn bool) {
	t.Helper()
	if movedOn {
		write(t, a.Root, "Notes/other.txt", "on the new team\n")
		if _, _, err := a.Save("on the new team", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	mine := b.Head()
	_, err := b.Share(Strategy("fail"))
	var gone *DeletedError
	if !errors.As(err, &gone) {
		t.Fatalf("share: %v", err)
	}
	if gone.MovedTo != two.Name || gone.MovedToTeam != two.ID || !strings.Contains(err.Error(), "it moved to "+two.Name) {
		t.Errorf("deleted: %+v (%v)", gone, err)
	}
	if b.Head() != mine {
		t.Fatal("the version here changed")
	}
	if err := b.FollowTo(two); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if tm, _ := b.Team(); tm.ID != two.ID {
		t.Errorf("the copy's team: %s", tm.Name)
	}
	res, err := b.Share(Strategy("fail"))
	if err != nil || res.Restored {
		t.Fatalf("share on the team it moved to: %+v %v", res, err)
	}
	c2, _ := b.Client()
	head := c2.(interface {
		Branches(string) (map[string]string, error)
	})
	bs, _ := head.Branches(b.Config.ProjectID)
	if bs["main"] != b.Head() {
		t.Fatalf("the new team's main: %v, here %s", bs, b.Head())
	}
	// Nothing lost: both copies' work in the team's latest.
	cl, _, err := Clone(storageCodeOf(t, two), b.Config.Name, filepath.Join(t.TempDir(), "C", "Song"), "yi")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Notes/left.txt": "left behind\n", "Notes/lyrics.txt": lyrics}
	if movedOn {
		want["Notes/other.txt"] = "on the new team\n"
	}
	for rel, text := range want {
		if got, _ := os.ReadFile(filepath.Join(cl.Root, filepath.FromSlash(rel))); string(got) != text {
			t.Errorf("%s on the new team: %q", rel, got)
		}
	}
}

// storageCodeOf is a team's address to clone from.
func storageCodeOf(t *testing.T, tm *teams.Team) string {
	t.Helper()
	return remote.EncodeConnectionCode(tm.Remote)
}

// A copy left on a team's own storage by a move: told where the project
// went (the first team's log says), it follows it, its version arrives on
// top of the new team's main; also when the new team moved on.
func TestCopyLeftByAMoveFollows(t *testing.T) {
	for _, movedOn := range []bool{false, true} {
		fake := s3test.New("one", "two")
		two := connected(t, storageCode(fake, "two"))
		a, b := staleCopy(t, storageCode(fake, "one"), two, false)
		followAndShare(t, a, b, two, movedOn)
		c1, _ := remote.Open(mustConfig(t, storageCode(fake, "one")))
		if ps, _ := c1.Projects(); len(ps) != 0 {
			t.Errorf("brought back on the first team: %+v", ps)
		}
		fake.Close()
	}
}

// As the user's: moved from an R3V Cloud team by an older R3V (the log
// doesn't say where): the team this computer knows with the project is
// found all the same.
func TestCopyLeftOnHostedTeamByAnOlderMove(t *testing.T) {
	for _, movedOn := range []bool{false, true} {
		f := cloudtest.NewFake(t)
		fake := s3test.New("two")
		two := connected(t, storageCode(fake, "two"))
		a, b := staleCopy(t, f.Address, two, true)
		followAndShare(t, a, b, two, movedOn)
		if !f.Deleted(b.Config.ProjectID) {
			t.Error("brought back on the first team")
		}
		fake.Close()
	}
}

// Pointing a copy elsewhere is only for a project its team no longer has.
func TestFollowRefusedWhileOnTheTeam(t *testing.T) {
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(storageCode(fake, "one")); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if err := a.FollowTo(connected(t, storageCode(fake, "two"))); err == nil {
		t.Fatal("pointed at another team while still on its own")
	}
}

// Deleted for good from a team's own storage: asked, the share makes it
// anew there, history and all.
func TestRestoreProjectDeletedFromOwnStorage(t *testing.T) {
	quickScan(t)
	fake := s3test.New("one")
	defer fake.Close()
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(storageCode(fake, "one")); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	actAs(t, a, "0000000000000000000000000000000a")
	c, _ := a.Client()
	if err := c.DeleteProject(a.Config.ProjectID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Share(Strategy("fail")); !errors.Is(err, ErrProjectDeleted) {
		t.Fatalf("share: %v", err)
	}
	a.RestoreDeleted = true
	res, err := a.Share(Strategy("fail"))
	if err != nil || !res.Restored {
		t.Fatalf("restore: %+v %v", res, err)
	}
	if bs, _ := c.Branches(a.Config.ProjectID); bs["main"] != a.Head() {
		t.Errorf("the team's main: %v", bs)
	}
}
