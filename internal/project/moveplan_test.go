package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A team's move planned: every key each project needs, there in its
// storage, in a safe order; a sample two projects use counts for both on
// R3V Cloud, once in the team's storage.
func TestPlanTeamMove(t *testing.T) {
	fake := s3test.New("one")
	defer fake.Close()
	code := storageCode(fake, "one")
	a, _ := movingProject(t, code) // a big file, branches, a deleted one, a milestone
	sample := randomBytes(3, 900<<10)
	os.WriteFile(filepath.Join(a.Root, "Samples", "shared.wav"), sample, 0o644)
	if _, _, err := a.Save("a shared sample", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	b, _ := Init(newProject(t), "yi")
	b.SetRemote(code)
	os.MkdirAll(filepath.Join(b.Root, "Samples"), 0o755)
	os.WriteFile(filepath.Join(b.Root, "Samples", "shared.wav"), sample, 0o644)
	if _, _, err := b.Save("the same sample", Strategy("fail")); err != nil {
		t.Fatal(err)
	}

	team, _ := a.Team()
	est, err := EstimateMove(team)
	if err != nil {
		t.Fatal(err)
	}
	if len(est.Plans) != 2 {
		t.Fatalf("estimate: %d plans, %d people", len(est.Plans), est.People)
	}
	if est.Hosted <= est.Storage {
		t.Errorf("hosted %d should be more than the storage's %d (the sample counts twice)", est.Hosted, est.Storage)
	}

	st, _ := remote.Open(mustConfig(t, code))
	sizes, _ := st.(*remote.BucketBackend).KeySizes("")
	for _, plan := range est.Plans {
		marked := map[string]bool{}
		listed := map[string]int{}
		snapshots := false
		for i, it := range plan.Items {
			if n, ok := sizes[it.Src]; !ok || n != it.Size {
				t.Errorf("%s: %s not in storage at size %d (%d, %v)", plan.Project.Name, it.Src, it.Size, n, ok)
			}
			switch {
			case strings.HasPrefix(it.Dst, "chunked/"):
				marked[strings.TrimPrefix(it.Dst, "chunked/")] = true
			case strings.HasPrefix(it.Dst, "objects/"):
				listed[strings.ReplaceAll(strings.TrimPrefix(it.Dst, "objects/"), "/", "")] = i
				if snapshots {
					t.Errorf("%s: a file after a version", plan.Project.Name)
				}
			case strings.HasPrefix(it.Dst, "snapshots/"):
				snapshots = true
			default:
				t.Errorf("unexpected item %+v", it)
			}
		}
		for h := range marked {
			if _, ok := listed[h]; !ok {
				t.Errorf("%s: big file %s marked, its list not planned", plan.Project.Name, h[:8])
			}
		}
		if plan.Versions == 0 {
			t.Errorf("%s: no versions", plan.Project.Name)
		}
	}
	// The project with the big file has its pieces, mark and list.
	var big *MovePlan
	for _, p := range est.Plans {
		if p.Project.ID == a.Config.ProjectID {
			big = p
		}
	}
	marks := 0
	for _, it := range big.Items {
		if strings.HasPrefix(it.Dst, "chunked/") {
			marks++
		}
	}
	if marks == 0 {
		t.Error("the big file's mark isn't planned")
	}
}

// A team moving to R3V Cloud takes no shares (the version stays here);
// moved, it says where to.
func TestNoSharesWhileMoving(t *testing.T) {
	fake := s3test.New("one")
	defer fake.Close()
	code := storageCode(fake, "one")
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(code)
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	if _, _, err := a.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	b, _ := remote.Open(mustConfig(t, code))
	info, _ := b.Info()
	info.Moving = &remote.TeamMove{To: "r3v-cloud+https://api.example/v1/teams/x"}
	if err := b.SetInfo(info); err != nil {
		t.Fatal(err)
	}
	write(t, a.Root, "Notes/lyrics.txt", lyrics+"while moving\n")
	m, _, err := a.Save("while moving", Strategy("fail"))
	if !errors.Is(err, remote.ErrTeamMoving) || m == nil || a.Head() != m.ID {
		t.Fatalf("save while moving: %v (kept here: %v)", err, m != nil)
	}
	c, _ := a.Client()
	if heads, _ := c.Branches(a.Config.ProjectID); heads["main"] == m.ID {
		t.Error("shared while moving")
	}
	info.Moving, info.MovedTo = nil, "r3v-cloud+https://api.example/v1/teams/x"
	b.SetInfo(info)
	var moved *remote.ErrTeamMoved
	if _, err := a.Share(Strategy("fail")); !errors.As(err, &moved) || moved.To == "" {
		t.Errorf("share once moved: %v", err)
	}
}
