package project

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

func withBranchRecords(t *testing.T) {
	t.Helper()
	was := remote.BranchRecords
	remote.BranchRecords = true
	t.Cleanup(func() { remote.BranchRecords = was })
}

// A branch called anything: its record names it, its key is what storage
// takes; renaming it moves nothing.
func TestBranchNames(t *testing.T) {
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

	key, err := a.CreateBranchNamed("  Mia 的主歌  ", "b4")
	if err != nil || key != "mia" || a.BranchName() != key {
		t.Fatalf("CreateBranchNamed: %q %v (on %q)", key, err, a.BranchName())
	}
	recs, err := a.BranchRecords()
	if err != nil || recs[key].Name != "Mia 的主歌" || recs[key].Color != "b4" {
		t.Fatalf("records: %+v %v", recs, err)
	}
	for _, taken := range []string{"mia 的主歌", "MAIN"} {
		if _, err := a.CreateBranchNamed(taken, ""); err == nil {
			t.Errorf("%q: a name taken was taken again", taken)
		}
	}
	if k, err := a.ResolveBranch("MIA 的主歌"); err != nil || k != key {
		t.Errorf("ResolveBranch: %q %v", k, err)
	}

	// Renamed: the key stays, so does where it is.
	c, _ := a.Client()
	before, _ := c.Branches(a.Config.ProjectID)
	if err := a.SetBranchRecord(key, "Verse, take 2", "b7"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetBranchRecord(key, "Main", ""); err == nil {
		t.Error("renamed to a name another branch has")
	}
	after, _ := c.Branches(a.Config.ProjectID)
	if recs, _ := a.BranchRecords(); recs[key].Name != "Verse, take 2" || recs[key].Color != "b7" || after[key] != before[key] || len(after) != len(before) {
		t.Errorf("after renaming: %+v, %v (was %v)", recs[key], after, before)
	}
	if k, _ := a.ResolveBranch("verse, TAKE 2"); k != key {
		t.Errorf("ResolveBranch after renaming: %q", k)
	}

	// A record left by a branch made half-way (stopped before the branch)
	// takes nothing: the name is free, and shows nowhere.
	store, _ := remote.BranchRecordsOf(c)
	store.PutBranchRecord(a.Config.ProjectID, "Chorus", remote.BranchRecord{Name: "Chorus", Color: "b9"})
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	if k, err := a.CreateBranchNamed("Chorus", ""); err != nil || k != "Chorus" {
		t.Errorf("a name a left record has: %q %v", k, err)
	}
	if recs, _ := a.BranchRecords(); recs["Chorus"].Color != "" {
		t.Errorf("the left record wasn't written anew: %+v", recs["Chorus"])
	}
}

// Without records (Stable), a branch's name is its key, as before.
func TestBranchNamesWithoutRecords(t *testing.T) {
	was := remote.BranchRecords
	remote.BranchRecords = false
	t.Cleanup(func() { remote.BranchRecords = was })
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(code)
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	a.Save("first", Strategy("fail"))
	if _, err := a.CreateBranchNamed("Mia verse", ""); err == nil {
		t.Error("a name storage can't take, without records")
	}
	if k, err := a.CreateBranchNamed("mia-verse", "b2"); err != nil || k != "mia-verse" {
		t.Errorf("%q %v", k, err)
	}
	if recs, _ := a.BranchRecords(); len(recs) != 0 {
		t.Errorf("records: %v", recs)
	}
	if err := a.SetBranchRecord("mia-verse", "Mia", ""); err == nil {
		t.Error("renamed without records")
	}
}
