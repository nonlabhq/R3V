package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

func storageCode(fake *s3test.Server, bucket string) string {
	return remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/" + bucket + "/r3v",
		AccessKey: "key", SecretKey: "secret"})
}

// movingProject is a project on team code with a few versions, a big file
// kept as pieces, a second branch with a name and colour, a deleted branch
// and a milestone.
func movingProject(t *testing.T, code string) (*Repo, []byte) {
	t.Helper()
	withBranchRecords(t)
	was := MoveProjects
	MoveProjects = true
	t.Cleanup(func() { MoveProjects = was })
	a, _ := Init(newProject(t), "yi")
	if err := a.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	big := randomBytes(11, chunk.MinFile+2<<20)
	os.MkdirAll(filepath.Join(a.Root, "Samples"), 0o755)
	os.WriteFile(filepath.Join(a.Root, "Samples", "stem.wav"), big, 0o644)
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddMilestone(a.Head(), "Demo for the label", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateBranchNamed("Mia 的主歌", "b4"); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"verse\n")
	a.Save("verse", Strategy("fail"))
	gone, _ := a.CreateBranchNamed("Old idea", "")
	write(t, a.Root, "Notes/idea.txt", "idea\n")
	a.Save("idea", Strategy("fail"))
	if _, err := a.SwitchBranch("main", false); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteBranch(gone); err != nil {
		t.Fatal(err)
	}
	return a, big
}

func connected(t *testing.T, code string) *teams.Team {
	t.Helper()
	tm, err := Connect(code)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

// A project moved: the other team has all of it (every version, the big
// file whole, branch names and colours, a deleted branch to get back, the
// milestone); the first team lets it go; the folder belongs to the other.
func TestMoveProjectToAnotherTeam(t *testing.T) {
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, big := movingProject(t, storageCode(fake, "one"))
	before, _ := a.Log()
	two := connected(t, storageCode(fake, "two"))

	if err := a.MoveToTeam(two, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	if tm, _ := a.Team(); tm.ID != two.ID {
		t.Errorf("the folder's team: %s", tm.Name)
	}
	one, _ := remote.Open(mustConfig(t, storageCode(fake, "one")))
	if ps, _ := one.Projects(); len(ps) != 0 {
		t.Errorf("the first team still lists it: %+v", ps)
	}

	b, _, err := Clone(storageCode(fake, "two"), a.Config.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("the big file isn't whole")
	}
	recs, _ := b.BranchRecords()
	names := []string{}
	for _, r := range recs {
		if r.Deleted == nil {
			names = append(names, r.Name)
		}
	}
	if !slices.Contains(names, "Mia 的主歌") {
		t.Errorf("branch names: %v", names)
	}
	if gone, _ := b.DeletedBranches(); len(gone) != 1 {
		t.Errorf("deleted branches to get back: %+v", gone)
	} else if err := b.RestoreBranch(gone[0].Key); err != nil {
		t.Errorf("getting the deleted branch back: %v", err)
	}
	if ms, _ := b.Milestones(); len(ms) != 1 {
		t.Errorf("milestones: %+v", ms)
	}
	after, _ := b.Log()
	if len(after) < len(before) {
		t.Errorf("versions: %d, were %d", len(after), len(before))
	}
	// Committing goes to the new team.
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"after the move\n")
	if _, _, err := a.Save("after", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
}

func mustConfig(t *testing.T, code string) remote.Config {
	t.Helper()
	cfg, err := remote.ParseAddress(code)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Copied: both teams have it; the folder stays the first's.
func TestCopyProjectToAnotherTeam(t *testing.T) {
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, _ := movingProject(t, storageCode(fake, "one"))
	from, _ := a.Team()
	two := connected(t, storageCode(fake, "two"))
	if err := a.MoveToTeam(two, MoveOptions{Copy: true}); err != nil {
		t.Fatal(err)
	}
	if tm, _ := a.Team(); tm.ID != from.ID {
		t.Errorf("the folder moved team on a copy")
	}
	for _, bucket := range []string{"one", "two"} {
		b, _ := remote.Open(mustConfig(t, storageCode(fake, bucket)))
		if ps, _ := b.Projects(); len(ps) != 1 {
			t.Errorf("%s lists %d projects", bucket, len(ps))
		}
	}
}

// Versions not shared yet: shared first.
func TestMoveProjectUnshared(t *testing.T) {
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, _ := movingProject(t, storageCode(fake, "one"))
	two := connected(t, storageCode(fake, "two"))
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"here only\n")
	if _, err := a.Snapshot("not shared"); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveToTeam(two, MoveOptions{}); !errors.Is(err, ErrMoveUnshared) {
		t.Errorf("moving with versions not shared: %v", err)
	}
}

// Stopped half-way: the other team shows nothing, the first has it all;
// moving again finishes.
func TestMoveProjectStopped(t *testing.T) {
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, big := movingProject(t, storageCode(fake, "one"))
	two := connected(t, storageCode(fake, "two"))
	var n atomic.Int32
	a.Cancel = func() error {
		if n.Add(1) > 3 {
			return ErrCancelled
		}
		return nil
	}
	if err := a.MoveToTeam(two, MoveOptions{}); !errors.Is(err, ErrCancelled) {
		t.Fatalf("stopped: %v", err)
	}
	b2, _ := remote.Open(mustConfig(t, storageCode(fake, "two")))
	if ps, _ := b2.Projects(); len(ps) != 0 {
		t.Errorf("the other team shows a project half-moved: %+v", ps)
	}
	if tm, _ := a.Team(); tm.ID == two.ID {
		t.Error("the folder moved before the project did")
	}
	a.Cancel = nil
	if err := a.MoveToTeam(two, MoveOptions{}); err != nil {
		t.Fatal(err)
	}
	b, _, err := Clone(storageCode(fake, "two"), a.Config.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("the big file isn't whole after moving again")
	}
}
