package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team's Activity: the project added and its member joining (the team
// log), its shares (the branch log, with the version's title), newest
// first; a project removed stays in it by its name.
func TestTeamActivity(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a) // (added before anyone was named: not logged)
	store, _ := teams.Load()
	teamID := store.Teams[0].ID
	if _, err := a.SetIdentity(teamID, "", "Robin"); err != nil {
		t.Fatal(err)
	}
	second := newSong(t)
	if _, err := a.AddProjectToTeam(teamID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(second, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("a chorus"), 0o644)
	if _, err := a.Save(root, "Chorus lyrics\n\nfor the second verse", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	items, err := a.TeamActivity(teamID, 7)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds["project-added"] != 1 || kinds["member-joined"] != 1 || kinds["share"] < 1 || kinds["branch-made"] != 2 {
		t.Fatalf("kinds %v in %+v", kinds, items)
	}
	if top := items[0]; top.Kind != "share" || top.Title != "Chorus lyrics" || top.ProjectName == "" || top.ByName == "" {
		t.Errorf("newest: %+v", top)
	}
	for i := 1; i < len(items); i++ {
		if a, b := parseTime(items[i].Time), parseTime(items[i-1].Time); a.After(b) {
			t.Fatalf("not newest first: %+v", items)
		}
	}
}

func parseTime(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }

// An R3V Cloud team's Activity comes ready made from its service (one
// request), with who and which agent for the filters; an older service
// without it, the team's logs are read as for any team.
func TestTeamActivityOfAHostedTeam(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	f := cloudtest.NewFake(t)
	a := NewApp()
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetIdentity(hosted.ID, cloudtest.Owner, "Robin"); err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	if _, err := a.AddProjectToTeam(hosted.ID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	f.SetEvents([]map[string]any{
		{"time": now.Add(-time.Hour).Format(time.RFC3339Nano), "kind": "share", "project": "p1", "projectName": "Song",
			"branch": "Main", "version": "v2", "title": "Chorus", "by": kaiID, "byName": "Kai", "tool": "claude-code", "model": "opus"},
		{"time": now.Format(time.RFC3339Nano), "kind": "merge", "project": "p1", "projectName": "Song",
			"branch": "Main", "version": "v3", "title": "Both verses", "by": cloudtest.Owner, "byName": "Robin"},
	})
	f.ResetRequests()
	items, err := a.TeamActivity(hosted.ID, 400)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Kind != "merge" || items[1].Tool != "claude-code" || items[1].Model != "opus" ||
		items[1].By != kaiID || items[1].ByName != "Kai" || items[0].Title != "Both verses" {
		t.Fatalf("items %+v", items)
	}
	if r := f.Requests(); f.Total() != 1 || r["GET events"] != 1 || f.EventsDays != "366" {
		t.Errorf("requests %v (days %s)", r, f.EventsDays)
	}

	// An older service: the team's logs, as before.
	f.Old.Store(true)
	f.ResetRequests()
	items, err = a.TeamActivity(hosted.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("requests: 1 from /events, %d from the logs (%v)", f.Total(), f.Requests())
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds["project-added"] != 1 || kinds["branch-made"] != 1 || kinds["merge"] != 0 {
		t.Fatalf("kinds %v in %+v", kinds, items)
	}
}
