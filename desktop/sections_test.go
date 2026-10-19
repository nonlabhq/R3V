package desktop

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Sections (docs/design/sections.md) in the app: the overview gives the
// team's sections in order and each project's; making, filling, renaming,
// moving and deleting them; kept as last read (LocalOverview); a
// teammate's change followed when the team is read; Activity has them. On
// R3V Cloud, only admins.

func sectionsOnApp(t *testing.T) {
	t.Helper()
	was := remote.ProjectSections
	remote.ProjectSections = true
	t.Cleanup(func() { remote.ProjectSections = was })
}

func overviewOf(t *testing.T, a *App, local bool) *Overview {
	t.Helper()
	get := a.Overview
	if local {
		get = a.LocalOverview
	}
	ov, err := get()
	if err != nil {
		t.Fatal(err)
	}
	return ov
}

func TestSectionsInApp(t *testing.T) {
	sectionsOnApp(t)
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	store, _ := teams.Load()
	teamID := store.Teams[0].ID
	r, _ := project.Open(root)
	pid := r.Config.ProjectID
	if _, err := a.SetIdentity(teamID, "", "Robin"); err != nil { // (named: the team log says who)
		t.Fatal(err)
	}
	store, _ = teams.Load()

	ov := overviewOf(t, a, false)
	if !ov.SectionsOn || !ov.Teams[0].CanEditSections || len(ov.Sections) != 0 || ov.Projects[0].Section != "" {
		t.Fatalf("before: %+v", ov)
	}
	game, err := a.NewSection(teamID, "Game")
	if err != nil {
		t.Fatal(err)
	}
	tools, _ := a.NewSection(teamID, "Tools")
	if err := a.SetProjectSection(teamID, pid, game.ID); err != nil {
		t.Fatal(err)
	}
	ov = overviewOf(t, a, false)
	if len(ov.Sections) != 2 || ov.Sections[0].Name != "Game" || ov.Projects[0].Section != game.ID || ov.Projects[0].SectionName != "Game" {
		t.Fatalf("grouped: %+v %+v", ov.Sections, ov.Projects[0])
	}
	// Moved to another, the sections' order changed, one renamed.
	if err := a.SetProjectSection(teamID, pid, tools.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveSection(teamID, tools.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.RenameSection(teamID, tools.ID, "Engine"); err != nil {
		t.Fatal(err)
	}
	// Without asking the team: as last read.
	ov = overviewOf(t, a, true)
	if len(ov.Sections) != 2 || ov.Sections[0].Name != "Engine" || ov.Projects[0].SectionName != "Engine" {
		t.Fatalf("local: %+v %+v", ov.Sections, ov.Projects[0])
	}
	if all, _ := a.AllProjects(); all[0].Projects[0].Section != tools.ID {
		t.Errorf("all projects: %+v", all[0].Projects[0])
	}
	// Deleted: ungrouped.
	if err := a.DeleteSection(teamID, tools.ID); err != nil {
		t.Fatal(err)
	}
	ov = overviewOf(t, a, true)
	if len(ov.Sections) != 1 || ov.Projects[0].Section != "" {
		t.Fatalf("after deleting: %+v %+v", ov.Sections, ov.Projects[0])
	}
	// A teammate's change: followed once the team is read.
	b, _ := store.Teams[0].Open()
	remote.SetProjectSection(b, pid, game.ID)
	if ov := overviewOf(t, a, true); ov.Projects[0].Section != "" {
		t.Error("followed before the team was read")
	}
	if ov := overviewOf(t, a, false); ov.Projects[0].Section != game.ID {
		t.Error("a teammate's change not followed")
	}
	if ov := overviewOf(t, a, true); ov.Projects[0].Section != game.ID {
		t.Error("not kept as last read")
	}
	// Activity says what changed.
	items, err := a.TeamActivity(teamID, 1)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]int{}
	for _, it := range items {
		if it.Kind == remote.EventSectionsChanged {
			changes[it.Change]++
			if it.Change == remote.SectionRenamed && (it.Was != "Tools" || it.Section != "Engine") {
				t.Errorf("renamed: %+v", it)
			}
			if it.Change == remote.SectionProjectIn && it.ProjectName == "" {
				t.Errorf("project in: %+v", it)
			}
		}
	}
	if changes[remote.SectionMade] != 2 || changes[remote.SectionProjectIn] != 3 || changes[remote.SectionDeleted] != 1 {
		t.Errorf("activity: %v", changes)
	}
	if _, err := a.NewSection(teamID, "game"); !errors.Is(err, remote.ErrSectionExists) {
		t.Errorf("a second Game: %v", err)
	}
}

// An older R3V has none: the projects show as before.
func TestSectionsNotInBuildApp(t *testing.T) {
	was := remote.ProjectSections
	remote.ProjectSections = false
	defer func() { remote.ProjectSections = was }()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	teamSong(t, a)
	store, _ := teams.Load()
	if _, err := a.NewSection(store.Teams[0].ID, "Game"); !errors.Is(err, remote.ErrSectionsNotInBuild) {
		t.Errorf("a section without them: %v", err)
	}
	if ov := overviewOf(t, a, false); ov.SectionsOn || ov.Teams[0].CanEditSections || len(ov.Sections) != 0 {
		t.Errorf("overview: %+v", ov)
	}
}

// On an R3V Cloud team a member sees the sections but can't change them.
func TestSectionsOnCloudAdminsOnlyApp(t *testing.T) {
	sectionsOnApp(t)
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
	defer forgetRoles()
	game, err := a.NewSection(hosted.ID, "Game")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetProjectSection(hosted.ID, tp.ID, game.ID); err != nil {
		t.Fatal(err)
	}
	if ov := overviewOf(t, a, false); !ov.Teams[0].CanEditSections {
		t.Error("not offered to the owner")
	}

	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	forgetRoles()
	if ov := overviewOf(t, a, true); ov.Teams[0].CanEditSections {
		t.Error("offered before the team was asked")
	}
	ov := overviewOf(t, a, false)
	if ov.Teams[0].CanEditSections || len(ov.Sections) != 1 || ov.Projects[0].Section != game.ID {
		t.Errorf("a member's overview: %+v %+v", ov.Teams[0], ov.Sections)
	}
	if _, err := a.NewSection(hosted.ID, "Tools"); !errors.Is(err, remote.ErrNotAdmin) {
		t.Errorf("a member making a section: %v", err)
	}
	if err := a.SetProjectSection(hosted.ID, tp.ID, ""); !errors.Is(err, remote.ErrNotAdmin) {
		t.Errorf("a member ungrouping: %v", err)
	}
}
