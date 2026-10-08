package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

// With the team's storage down, LocalOverview still lists the projects at
// once (the app starts on it): those here, and the team's others as it last
// listed them. Overview says the team can't be reached.
func TestOverviewWithTheTeamDown(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	defer func(w time.Duration) { remote.RetryWait = w }(remote.RetryWait)
	remote.RetryWait = time.Millisecond
	fake := s3test.New("one")
	t.Cleanup(waitTidy)
	a := NewApp()
	team, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	if _, err := a.AddProjectToTeam(team.ID, root); err != nil {
		t.Fatal(err)
	}
	store, _ := teams.Load()
	c, err := store.Find(team.ID).Open()
	if err != nil {
		t.Fatal(err)
	}
	c.PutProject(remote.Project{ID: teams.NewID(16), Name: "Other"}) // not downloaded here
	if ov, err := a.Overview(); err != nil || len(ov.Projects) != 2 {
		t.Fatalf("up: %+v %v", ov, err)
	}
	waitTidy()
	fake.Close()
	remotes := func(ov *Overview) (names []string) {
		for _, p := range ov.Projects {
			if p.Status == "remote" {
				names = append(names, p.Name)
			}
		}
		return names
	}

	start := time.Now()
	ov, err := a.LocalOverview()
	if err != nil || len(ov.Projects) != 2 || ov.TeamChecked || ov.TeamError != "" {
		t.Fatalf("local: %+v %v", ov, err)
	}
	if r := remotes(ov); len(r) != 1 || r[0] != "Other" {
		t.Fatalf("local, the team's: %v", r)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("LocalOverview took %v", d)
	}
	ov, err = a.Overview()
	if err != nil || !ov.TeamChecked || ov.TeamError == "" || len(remotes(ov)) != 1 {
		t.Fatalf("with the team: %+v %v", ov, err)
	}
}

// The quick launcher looks across teams without asking them: each team's
// projects here, and the rest as it last listed them.
func TestAllProjects(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	a := NewApp()
	one, two := s3test.New("one"), s3test.New("two")
	defer one.Close()
	defer two.Close()
	t1, err := a.CreateStorageTeam(remote.Storage{Endpoint: one.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := a.CreateStorageTeam(remote.Storage{Endpoint: two.URL, Bucket: "two", AccessKey: "k", SecretKey: "s"}, "Two")
	if err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	if _, err := a.AddProjectToTeam(t1.ID, root); err != nil {
		t.Fatal(err)
	}
	store, _ := teams.Load()
	c, _ := store.Find(t2.ID).Open()
	c.PutProject(remote.Project{ID: teams.NewID(16), Name: "Elsewhere"})
	if err := a.SelectTeam(t2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Overview(); err != nil { // (the team's list, remembered)
		t.Fatal(err)
	}
	all, err := a.AllProjects()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, tp := range all {
		for _, p := range tp.Projects {
			got[tp.Team] = append(got[tp.Team], p.Name+":"+p.Status)
		}
	}
	if len(got[t1.ID]) != 1 || !strings.HasSuffix(got[t1.ID][0], ":downloaded") {
		t.Errorf("team one: %v", got[t1.ID])
	}
	if len(got[t2.ID]) != 1 || got[t2.ID][0] != "Elsewhere:remote" {
		t.Errorf("team two: %v", got[t2.ID])
	}
}
