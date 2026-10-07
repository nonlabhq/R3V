package project

import (
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A project renamed by one person keeps its name when the others save.
func TestRenameStaysForTheTeam(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	b, _, err := Clone(code, "Song", filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Rename("  "); err == nil {
		t.Error("an empty name is refused")
	}
	if err := a.Rename("Night Drive"); err != nil {
		t.Fatal(err)
	}
	if again, _ := Open(a.Root); again.Config.Name != "Night Drive" {
		t.Errorf("local name: %q", again.Config.Name)
	}
	write(t, b.Root, "Notes/lyrics.txt", lyrics+"line 9\n")
	if _, _, err := b.Save("more", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	c, _ := a.Client()
	ps, err := c.Projects()
	if err != nil || len(ps) != 1 || ps[0].Name != "Night Drive" {
		t.Fatalf("team's name after another save: %+v %v", ps, err)
	}
}

// A project with versions here joins a team: Share uploads them without
// committing the files.
func TestShareVersionsLater(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	first := mustSnapshot(t, a, "here only")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"not committed\n")
	res, err := a.Share(Strategy("fail"))
	if err != nil || res.Action != "published" || a.Head() != first.ID {
		t.Fatalf("share: %+v %v (head %s)", res, err, a.Head())
	}
	c, _ := a.Client()
	if bs, _ := c.Branches(a.Config.ProjectID); bs["main"] != first.ID {
		t.Fatalf("team's main: %v", bs)
	}
	if changes, _ := a.Status(); len(changes) != 1 {
		t.Fatalf("the uncommitted change stays: %+v", changes)
	}
}

// A first share stopped after the project's record was written, then the
// project renamed or given a look meanwhile: sharing again keeps the record.
func TestShareAgainKeepsTheRecord(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	c, _ := a.Client()
	kept := remote.Project{ID: a.Config.ProjectID, Name: "Night Drive", Icon: "drum", Color: "teal"}
	if err := c.PutProject(kept); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	ps, err := c.Projects()
	if err != nil || len(ps) != 1 || ps[0].Name != kept.Name || ps[0].Icon != kept.Icon || ps[0].Color != kept.Color {
		t.Fatalf("the team's record: %+v %v", ps, err)
	}
}
