package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// behindLane: a team project on main with branch vfx one version ahead of
// it (main hasn't moved); returns the folder and vfx's head.
func behindLane(t *testing.T) (string, string) {
	t.Helper()
	a := teamProject(t)
	if exit, text := runText(t, a, "branch", "new", "vfx"); exit != 0 {
		t.Fatalf("branch new: %s", text)
	}
	os.WriteFile(filepath.Join(a, "src", "vfx.ts"), []byte("bloom"), 0o644)
	exit, rep := run(t, a, "commit", "-m", "Add bloom pass")
	var c commitJSON
	json.Unmarshal(rep.Result, &c)
	if exit != 0 || c.Saved == nil {
		t.Fatalf("commit: %s %+v", rep.Result, rep.Error)
	}
	if exit, rep := run(t, a, "switch", "main"); exit != 0 {
		t.Fatalf("switch: %+v", rep.Error)
	}
	return a, c.Saved.ID
}

// Merging a branch main is behind: the preview says merge (a merge version
// is made), not fast-forward, and both it and the merge's result say the
// files are exactly the branch head's.
func TestMergeBehindPreviewAndResult(t *testing.T) {
	a, lane := behindLane(t)
	exit, rep := run(t, a, "merge", "vfx", "--preview")
	var pv previewJSON
	json.Unmarshal(rep.Result, &pv)
	if exit != 0 || pv.Action != "merge" || pv.SameFilesAs != lane || len(pv.Versions) != 1 {
		t.Fatalf("preview: %d %s", exit, rep.Result)
	}
	exit, rep = run(t, a, "merge", "vfx")
	var res syncJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || res.Action != "merged" || res.SameFilesAs != lane || res.To == lane {
		t.Fatalf("merge: %d %s %+v", exit, rep.Result, rep.Error)
	}
}

// A merge that changes the files says nothing of the kind.
func TestMergeBothChangedHasNoSameFiles(t *testing.T) {
	a := laneProject(t)
	exit, rep := run(t, a, "merge", "vfx", "--preview")
	if exit != 0 || pvField(rep.Result, "same_files_as") {
		t.Fatalf("preview: %s", rep.Result)
	}
	exit, rep = run(t, a, "merge", "vfx")
	if exit != 0 || pvField(rep.Result, "same_files_as") {
		t.Fatalf("merge: %s", rep.Result)
	}
}

func pvField(data []byte, name string) bool {
	var m map[string]any
	json.Unmarshal(data, &m)
	_, ok := m[name]
	return ok
}
