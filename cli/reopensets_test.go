package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Merging a branch that changed no Live Set says nothing of Live (a Unity
// project's scenes, say); one that changed a set says to reopen it.
func TestReopenSetsOnlyForLiveSets(t *testing.T) {
	set, err := os.ReadFile(filepath.Join("..", "testdata", "live", "SampleAbletonProject.als"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := behindLane(t)
	exit, text := runText(t, a, "merge", "vfx")
	if exit != 0 || strings.Contains(text, "Live") || !strings.Contains(text, "merged vfx into main") {
		t.Fatalf("merge of code: %d %s", exit, text)
	}

	if exit, text := runText(t, a, "branch", "new", "mix"); exit != 0 {
		t.Fatalf("branch new: %s", text)
	}
	os.WriteFile(filepath.Join(a, "Song.als"), set, 0o644)
	if exit, rep := run(t, a, "commit", "-m", "Add the set"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "switch", "main"); exit != 0 {
		t.Fatalf("switch: %+v", rep.Error)
	}
	exit, rep := run(t, a, "merge", "mix")
	var res syncJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || res.Action != "merged" || !res.ReopenSets {
		t.Fatalf("merge of a set: %d %s %+v", exit, rep.Result, rep.Error)
	}
}
