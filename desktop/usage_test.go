package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/teams"
)

// A team's Usage on its own storage: nothing until counted; then each
// project's size, what they use together (a file both have counted once)
// and all the storage holds; kept for the next time.
func TestTeamUsage(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	store, _ := teams.Load()
	teamID := store.Teams[0].ID
	other := newSong(t) // (the same sample as the first song)
	os.WriteFile(filepath.Join(other, "notes.txt"), []byte("only in the second song, a few words"), 0o644)
	if _, err := a.AddProjectToTeam(teamID, other); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(other, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	_ = root

	v, err := a.TeamUsage(teamID)
	if err != nil || v.Hosted || v.Counted != "" || v.Used != 0 {
		t.Fatalf("before counting: %+v, %v", v, err)
	}
	v, err = a.CountTeamUsage(teamID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Counted == "" || len(v.Projects) != 2 || v.Versions != 2 {
		t.Fatalf("counted: %+v", v)
	}
	sum := v.Projects[0].Bytes + v.Projects[1].Bytes
	if v.Used <= 0 || v.Used >= sum || v.Stored < v.Used {
		t.Errorf("used %d of %d stored; projects %d + %d (shared files once)", v.Used, v.Stored, v.Projects[0].Bytes, v.Projects[1].Bytes)
	}
	if v.Projects[0].Name == "" || v.Projects[0].Bytes < v.Projects[1].Bytes {
		t.Errorf("projects: %+v", v.Projects)
	}
	if kept, _ := a.TeamUsage(teamID); kept.Counted != v.Counted || kept.Used != v.Used {
		t.Errorf("not kept: %+v", kept)
	}
}
