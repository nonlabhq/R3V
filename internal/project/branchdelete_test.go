package project

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A branch deleted is gone from the team, its versions not: it comes back
// where it was, with its name and colour.
func TestDeleteAndRestoreBranch(t *testing.T) {
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
	if err := a.DeleteBranch(key); !errors.Is(err, ErrOnBranch) {
		t.Errorf("deleting the branch you're on: %v", err)
	}
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteBranch("main"); !errors.Is(err, ErrMainBranch) {
		t.Errorf("deleting main: %v", err)
	}
	if err := a.DeleteBranch(key); err != nil {
		t.Fatal(err)
	}
	if heads, _ := c.Branches(a.Config.ProjectID); heads[key] != "" {
		t.Fatalf("still there: %v", heads)
	}
	gone, err := a.DeletedBranches()
	if err != nil || len(gone) != 1 || gone[0].Key != key || gone[0].Head != ideaHead {
		t.Fatalf("deleted branches: %+v %v", gone, err)
	}

	if err := a.RestoreBranch(key); err != nil {
		t.Fatal(err)
	}
	heads, _ = c.Branches(a.Config.ProjectID)
	recs, _ := a.BranchRecords()
	if heads[key] != ideaHead || recs[key].Name != "Idée à deux" || recs[key].Color != "b8" {
		t.Errorf("restored: %v, %+v", heads, recs[key])
	}
	if gone, _ := a.DeletedBranches(); len(gone) != 0 {
		t.Errorf("still listed as deleted: %+v", gone)
	}
	if err := a.RestoreBranch(key); err == nil {
		t.Error("restored a branch that is there")
	}
}

// Stopped after the delete, before the branch log was written: the branch
// still comes back (its record kept where it was).
func TestRestoreBranchWithoutItsLog(t *testing.T) {
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
	if err := a.DeleteBranch(key); err != nil {
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
	gone, err := a.DeletedBranches()
	if err != nil || len(gone) != 1 || gone[0].Key != key || gone[0].Head != heads[key] {
		t.Fatalf("deleted branches without the log: %+v %v", gone, err)
	}
	if err := a.RestoreBranch(key); err != nil {
		t.Fatal(err)
	}
	if now, _ := c.Branches(a.Config.ProjectID); now[key] != heads[key] {
		t.Errorf("restored: %v", now)
	}
	if recs, _ := a.BranchRecords(); recs[key].Deleted != nil || recs[key].Name != "Bridge" {
		t.Errorf("record after restoring: %+v", recs[key])
	}
}
