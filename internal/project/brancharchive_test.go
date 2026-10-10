package project

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A branch archived is gone from the team, its versions not: it comes back
// where it was, with its name and colour.
func TestArchiveAndUnarchiveBranch(t *testing.T) {
	withBranchRecords(t)
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
	key, err := a.CreateBranchNamed("Idée à deux", "b8")
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range []string{"line 9\n", "line 10\n"} {
		write(t, a.Root, "Notes/lyrics.txt", lyrics+text)
		if _, _, err := a.Save("idea "+string(rune('1'+i)), Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	c, _ := a.Client()
	heads, _ := c.Branches(a.Config.ProjectID)
	ideaHead := heads[key]

	if n, err := a.OnlyOnBranch(key); err != nil || n != 2 {
		t.Errorf("versions only on the branch: %d %v", n, err)
	}
	if err := a.ArchiveBranch(key); !errors.Is(err, ErrOnBranch) {
		t.Errorf("archiving the branch you're on: %v", err)
	}
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	if err := a.ArchiveBranch("main"); !errors.Is(err, ErrMainBranch) {
		t.Errorf("archiving main: %v", err)
	}
	if err := a.ArchiveBranch(key); err != nil {
		t.Fatal(err)
	}
	if heads, _ := c.Branches(a.Config.ProjectID); heads[key] != "" {
		t.Fatalf("still there: %v", heads)
	}
	gone, err := a.ArchivedBranches()
	if err != nil || len(gone) != 1 || gone[0].Key != key || gone[0].Head != ideaHead {
		t.Fatalf("deleted branches: %+v %v", gone, err)
	}

	if err := a.UnarchiveBranch(key); err != nil {
		t.Fatal(err)
	}
	heads, _ = c.Branches(a.Config.ProjectID)
	recs, _ := a.BranchRecords()
	if heads[key] != ideaHead || recs[key].Name != "Idée à deux" || recs[key].Color != "b8" {
		t.Errorf("restored: %v, %+v", heads, recs[key])
	}
	if gone, _ := a.ArchivedBranches(); len(gone) != 0 {
		t.Errorf("still listed as deleted: %+v", gone)
	}
	if err := a.UnarchiveBranch(key); err == nil {
		t.Error("restored a branch that is there")
	}
}

// Stopped after the delete, before the branch log was written: the branch
// still comes back (its record kept where it was).
func TestUnarchiveBranchWithoutItsLog(t *testing.T) {
	withBranchRecords(t)
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(code)
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	a.Save("first", Strategy("fail"))
	key, _ := a.CreateBranchNamed("Bridge", "")
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"bridge\n")
	a.Save("bridge", Strategy("fail"))
	a.SwitchBranch("main", false)
	c, _ := a.Client()
	heads, _ := c.Branches(a.Config.ProjectID)
	if err := a.ArchiveBranch(key); err != nil {
		t.Fatal(err)
	}
	// The log as if it never got the delete.
	b := c.(*remote.BucketBackend).Bucket()
	var logKeys []string
	b.List("projects/"+a.Config.ProjectID+"/branchlog/", "", func(items []remote.Item) bool {
		for _, it := range items {
			logKeys = append(logKeys, it.Key)
		}
		return true
	})
	for _, k := range logKeys {
		b.Delete(k, "")
	}
	gone, err := a.ArchivedBranches()
	if err != nil || len(gone) != 1 || gone[0].Key != key || gone[0].Head != heads[key] {
		t.Fatalf("deleted branches without the log: %+v %v", gone, err)
	}
	if err := a.UnarchiveBranch(key); err != nil {
		t.Fatal(err)
	}
	if now, _ := c.Branches(a.Config.ProjectID); now[key] != heads[key] {
		t.Errorf("restored: %v", now)
	}
	if recs, _ := a.BranchRecords(); recs[key].Deleted != nil || recs[key].Name != "Bridge" {
		t.Errorf("record after restoring: %+v", recs[key])
	}
}

// A branch archived with those made from it, one of them kept (now coming
// from main); only an archived branch is deleted, for good, and its key
// isn't taken while it is archived.
func TestArchiveSubtreeAndDelete(t *testing.T) {
	withBranchRecords(t)
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
	for _, name := range []string{"ideal", "test", "hello"} {
		if name != "ideal" {
			a.SwitchBranch("ideal", false)
		}
		if _, err := a.CreateBranchNamed(name, ""); err != nil {
			t.Fatal(err)
		}
		write(t, a.Root, "Notes/"+name+".txt", name)
		if _, _, err := a.Save("on "+name, Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	recs, _ := a.BranchRecords()
	if recs["test"].Parent != "ideal" || recs["hello"].Parent != "ideal" || recs["ideal"].Parent != "main" {
		t.Fatalf("parents: %+v", recs)
	}
	a.SwitchBranch("main", false)
	if err := a.ArchiveBranches([]string{"ideal", "test"}, map[string]string{"hello": "main"}); err != nil {
		t.Fatal(err)
	}
	c, _ := a.Client()
	heads, _ := c.Branches(a.Config.ProjectID)
	if heads["ideal"] != "" || heads["test"] != "" || heads["hello"] == "" {
		t.Fatalf("heads: %v", heads)
	}
	if recs, _ := a.BranchRecords(); recs["hello"].Parent != "main" {
		t.Fatalf("hello kept: %+v", recs["hello"])
	}
	if gone, _ := a.ArchivedBranches(); len(gone) != 2 {
		t.Fatalf("archived: %+v", gone)
	}

	// Its key stays its own while archived.
	key, err := a.CreateBranchNamed("ideal", "")
	if err != nil || key == "ideal" {
		t.Fatalf("a new branch called ideal: %q %v", key, err)
	}
	if err := a.DeleteBranch("hello"); err == nil {
		t.Error("deleted a branch not archived")
	}
	a.SwitchBranch("main", false)
	if err := a.DeleteBranch("test"); err != nil {
		t.Fatal(err)
	}
	if gone, _ := a.ArchivedBranches(); len(gone) != 1 || gone[0].Key != "ideal" {
		t.Fatalf("archived after deleting test: %+v", gone)
	}
	if err := a.UnarchiveBranch("test"); err == nil {
		t.Error("a deleted branch came back")
	}
	if err := a.UnarchiveBranch("ideal"); err != nil {
		t.Fatal(err)
	}
}
