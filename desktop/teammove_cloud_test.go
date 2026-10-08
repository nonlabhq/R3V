//go:build nightly

package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teammove"
	"github.com/nonlabhq/r3v/internal/teams"
)

// fakeMover plays R3V Cloud's part of a move: it copies the plans' items
// from the old storage to the hosted team, as they are.
type fakeMover struct {
	mu       sync.Mutex
	src, dst remote.Bucket
	prefix   string
	plans    map[string][]cloud.PlanItem // by project
	copied   int
	imported []cloud.ImportedMember
	claimed  []string
	forgot   bool
}

func (f *fakeMover) StartMove(_, _ string, s cloud.MoveSource) (string, error) {
	f.prefix = s.Prefix
	return "m1", nil
}
func (f *fakeMover) AddPlan(_, _, _, p string, items []cloud.PlanItem) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plans[p] = append(f.plans[p], items...)
	return len(items), 0, nil
}
func (f *fakeMover) StartCopy(_, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for p, items := range f.plans {
		for _, it := range items {
			data, _, err := f.src.Get(strings.TrimPrefix(it.Src, f.prefix+"/"))
			if err != nil {
				return err
			}
			if err := f.dst.Put("projects/"+p+"/"+it.Dst, bytes.NewReader(data), int64(len(data)), "", ""); err != nil {
				return err
			}
			f.copied++
		}
	}
	return nil
}
func (f *fakeMover) Move(_, _, _ string) (*cloud.MoveStatus, error) {
	return &cloud.MoveStatus{State: "done"}, nil
}
func (f *fakeMover) ForgetMove(_, _, _ string) error { f.forgot = true; return nil }
func (f *fakeMover) ImportMembers(_, _ string, ms []cloud.ImportedMember) error {
	f.imported = ms
	return nil
}
func (f *fakeMover) Claim(_, _, id string) error { f.claimed = append(f.claimed, id); return nil }

func waitMove(t *testing.T, a *App, teamID, phase string) *TeamMoveState {
	t.Helper()
	for range 200 {
		if _, busy := moving.Load(teamID); !busy {
			st, err := a.TeamMoveState(teamID)
			if err != nil {
				t.Fatal(err)
			}
			if st.Error != "" {
				t.Fatalf("move: %s", st.Error)
			}
			if st.Phase == phase {
				return st
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the move didn't get to %s", phase)
	return nil
}

// A team moves to R3V Cloud: estimated, copied while the team works on,
// finished (frozen, the second pass, records and branches); the old team
// says where it went, this computer's project follows, a teammate gets it
// whole from the hosted team.
func TestTeamMoveToCloud(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("one")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	one, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", Folder: "r3v", AccessKey: "k", SecretKey: "s"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetIdentity(one.ID, "", "Yi"); err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	tp, err := a.AddProjectToTeam(one.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	hostedAddr := cloudtest.New(t)
	hosted, err := a.ConnectTeam(hostedAddr)
	if err != nil {
		t.Fatal(err)
	}

	est, err := a.EstimateTeamMove(one.ID)
	if err != nil || len(est.Projects) != 1 || est.Hosted == 0 || est.Bucket != "one" {
		t.Fatalf("estimate: %+v %v", est, err)
	}

	s, _ := teams.Load()
	src, _ := s.Find(one.ID).Open()
	dst, _ := s.Find(hosted.ID).Open()
	f := &fakeMover{src: src.(*remote.BucketBackend).Bucket(), dst: dst.(*remote.BucketBackend).Bucket(), plans: map[string][]cloud.PlanItem{}}
	was := moves
	moves = f
	t.Cleanup(func() { moves = was })
	teammove.Every = time.Millisecond

	if err := a.StartTeamMove(one.ID, hosted.ID, []string{tp.ID}, "ro-key", "ro-secret", ""); err != nil {
		t.Fatal(err)
	}
	waitMove(t, a, one.ID, "copying")
	if f.copied == 0 || len(f.imported) != 1 || f.imported[0].Name != "Yi" {
		t.Fatalf("copied %d, imported %+v", f.copied, f.imported)
	}
	// The team works on meanwhile: a version shared during the bulk copy.
	writeFile(t, root, "Notes/during.txt", "shared while copying\n")
	if _, err := a.Save(root, "during the copy", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	if err := a.FinishTeamMove(one.ID); err != nil {
		t.Fatal(err)
	}
	waitMove(t, a, one.ID, "done")
	info, _ := src.Info()
	if info.MovedTo != s.Find(hosted.ID).Remote.URL || info.Moving != nil {
		t.Errorf("the old team: moving %v, moved to %q", info.Moving, info.MovedTo)
	}
	if !f.forgot || len(f.claimed) != 1 {
		t.Errorf("forgot the key %v, claimed %v", f.forgot, f.claimed)
	}
	r, _ := project.Open(root)
	if tm, _ := r.Team(); tm.ID != hosted.ID {
		t.Errorf("the project's team: %s", tm.Name)
	}
	b, _, err := project.Clone(hostedAddr, tp.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if log, _ := b.Log(); len(log) != 2 {
		t.Errorf("versions on the hosted team: %d, want 2 (the one shared during the copy too)", len(log))
	}
	// Committing goes to the hosted team now.
	writeFile(t, root, "Notes/after.txt", "after the move\n")
	if _, err := a.Save(root, "after the move", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
