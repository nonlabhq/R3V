package desktop

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Archived projects (docs/design/archive.md) in the app: archiving asks
// first about work not shared here; an archived project is marked in the
// lists (the left panel leaves it out) and isn't watched, here or on a
// teammate's computer once it reads the team; bringing it back watches it
// again. On R3V Cloud, only admins.

func archivingOn(t *testing.T) {
	t.Helper()
	was := remote.ProjectArchive
	remote.ProjectArchive = true
	t.Cleanup(func() { remote.ProjectArchive = was })
}

func watching(a *App, root string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.teamWatches[root]
	return ok
}

func listed(t *testing.T, a *App, id string) TeamProject {
	t.Helper()
	ov, err := a.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if !ov.Archive {
		t.Error("the build archives, the overview says not")
	}
	for _, p := range ov.Projects {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("%s not listed", id)
	return TeamProject{}
}

func TestArchiveProjectInApp(t *testing.T) {
	archivingOn(t)
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	store, _ := teams.Load()
	teamID := store.Teams[0].ID
	r, _ := project.Open(root)
	pid := r.Config.ProjectID
	a.startWatch(root)
	if !watching(a, root) {
		t.Fatal("not watched to begin with")
	}

	// Uncommitted changes: said, and refused unless confirmed.
	writeFile(t, root, "notes.txt", "a change")
	info, err := a.ArchiveInfo(teamID, pid)
	if err != nil || !info.CanArchive || info.Archived || info.Changes != 1 || info.Unshared {
		t.Fatalf("archive info: %+v %v", info, err)
	}
	if err := a.ArchiveProject(teamID, pid, false); !errors.Is(err, project.ErrDirty) {
		t.Fatalf("archiving with changes: %v", err)
	}
	if listed(t, a, pid).Archived || !watching(a, root) {
		t.Fatal("archived all the same")
	}
	// A version not shared: said too.
	r, _ = project.Open(root)
	if _, err := r.Snapshot("only here"); err != nil {
		t.Fatal(err)
	}
	if info, _ := a.ArchiveInfo(teamID, pid); info.Changes != 0 || !info.Unshared {
		t.Fatalf("archive info, a version here: %+v", info)
	}
	if err := a.ArchiveProject(teamID, pid, false); !errors.Is(err, project.ErrUnshared) {
		t.Fatalf("archiving with a version not shared: %v", err)
	}

	// Confirmed: archived for the team, not watched, still listed (marked).
	if err := a.ArchiveProject(teamID, pid, true); err != nil {
		t.Fatal(err)
	}
	if p := listed(t, a, pid); !p.Archived || p.Root != root || p.Status != "downloaded" {
		t.Fatalf("listed as %+v", p)
	}
	if watching(a, root) {
		t.Error("still watched")
	}
	a.startWatch(root) // (as when the app starts)
	if watching(a, root) {
		t.Error("watched again when the app starts")
	}
	if ov, _ := a.LocalOverview(); !ov.Projects[0].Archived {
		t.Error("not marked without asking the team")
	}
	if info, _ := a.ArchiveInfo(teamID, pid); !info.Archived {
		t.Error("archive info: not archived")
	}

	// Brought back: watched again.
	if err := a.UnarchiveProject(teamID, pid); err != nil {
		t.Fatal(err)
	}
	if listed(t, a, pid).Archived || !watching(a, root) {
		t.Error("not back")
	}

	// A teammate archives it: this computer follows once it reads the team.
	b, _ := store.Teams[0].Open()
	if _, err := remote.SetProjectArchived(b, pid, true); err != nil {
		t.Fatal(err)
	}
	if !listed(t, a, pid).Archived || watching(a, root) {
		t.Error("a teammate's archiving not followed")
	}
	remote.SetProjectArchived(b, pid, false)
	if listed(t, a, pid).Archived || !watching(a, root) {
		t.Error("a teammate bringing it back not followed")
	}
	a.stopWatch(root)
}

// An older R3V (or Stable) knows nothing of it: the project shows as before.
func TestArchiveNotInBuildApp(t *testing.T) {
	was := remote.ProjectArchive
	remote.ProjectArchive = false
	defer func() { remote.ProjectArchive = was }()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	store, _ := teams.Load()
	r, _ := project.Open(root)
	if err := a.ArchiveProject(store.Teams[0].ID, r.Config.ProjectID, true); !errors.Is(err, remote.ErrArchiveNotInBuild) {
		t.Errorf("archiving without it: %v", err)
	}
	if ov, _ := a.Overview(); ov.Archive || ov.Projects[0].Archived {
		t.Errorf("overview: %+v", ov)
	}
}

// On an R3V Cloud team a member can't archive; an admin can.
func TestArchiveOnCloudAdminsOnlyApp(t *testing.T) {
	archivingOn(t)
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	f := cloudtest.NewFake(t)
	f.AddMember("kai", kaiID, false)
	a := NewApp()
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	teams.Update(func(s *teams.Store) error {
		s.Find(hosted.ID).MemberID = cloudtest.Owner
		return nil
	})
	root := newSong(t)
	tp, err := a.AddProjectToTeam(hosted.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	forgetRoles := func() {
		roles.Lock()
		roles.byTeam = map[string]cachedRole{}
		roles.Unlock()
	}
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	forgetRoles()
	if info, err := a.ArchiveInfo(hosted.ID, tp.ID); err != nil || info.CanArchive {
		t.Errorf("a member's archive info: %+v %v", info, err)
	}
	if err := a.ArchiveProject(hosted.ID, tp.ID, true); !errors.Is(err, remote.ErrNotAdmin) {
		t.Errorf("a member archiving: %v", err)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	forgetRoles()
	if err := a.ArchiveProject(hosted.ID, tp.ID, false); err != nil {
		t.Fatalf("the owner archiving: %v", err)
	}
	if !listed(t, a, tp.ID).Archived {
		t.Error("not archived")
	}
	forgetRoles()
}
