//go:build nightly

package desktop

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

// File locking as a default (team-settings.md): with the team's off, a
// project turning it on for itself locks, and the others don't; with the
// team's on, a project turning it off doesn't; the team's switch back puts
// every project to it.
func TestProjectFileLocking(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	f := cloudtest.NewFake(t)
	a := NewApp()
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	teams.Update(func(s *teams.Store) error {
		s.Find(hosted.ID).MemberID = cloudtest.Owner
		return nil
	})
	game, other := newSong(t), newSong(t)
	for _, root := range []string{game, other} {
		writeFile(t, root, "Maps/Harbor.umap", "0")
		if _, err := a.AddProjectToTeam(hosted.ID, root); err != nil {
			t.Fatal(err)
		}
		if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
			t.Fatal(err)
		}
	}
	on, off := true, false
	if err := a.SetProjectWorkflow(game, remote.Workflow{Locks: &remote.ProjectLocks{On: &on}}); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.ProjectLocks(game); !v.On {
		t.Error("the project that turned locking on doesn't lock")
	}
	if v, _ := a.ProjectLocks(other); v.On {
		t.Error("a project following the team's (off) locks")
	}
	if tl, _ := a.TeamLocks(hosted.ID); tl.On {
		t.Error("the team's default shown on")
	}
	if _, err := a.LockFiles(game, []string{"Maps/Harbor.umap"}); err != nil {
		t.Errorf("lock in the project that turned it on: %v", err)
	}

	// The team's on: the other locks; the game turning it off doesn't.
	if err := a.SetTeamLocks(hosted.ID, true, profile.DefaultLockKinds()); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProjectWorkflow(game, remote.Workflow{Locks: &remote.ProjectLocks{On: &off}}); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.ProjectLocks(other); !v.On {
		t.Error("a project following the team's (on) doesn't lock")
	}
	if v, _ := a.ProjectLocks(game); v.On {
		t.Error("the project that turned locking off locks")
	}
	if n, err := a.ClearProjectWorkflow(hosted.ID, "locks"); err != nil || n != 1 {
		t.Fatalf("cleared %d, %v", n, err)
	}
	if v, _ := a.ProjectLocks(game); !v.On {
		t.Error("back to the team's: not locking")
	}
}
