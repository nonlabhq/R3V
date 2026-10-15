package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
