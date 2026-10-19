package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// r3v merge with a change not committed: one the merge doesn't touch stays
// (kept_work, not in the merge version); one it writes refuses it with
// unsaved_changes, naming the file.
func TestMergeWithUncommittedChanges(t *testing.T) {
	a, _ := behindLane(t)
	os.WriteFile(filepath.Join(a, "src", "vfx.ts"), []byte("mine"), 0o644)
	exit, rep := run(t, a, "merge", "vfx")
	var details struct{ Files []string }
	if rep.Error != nil {
		b, _ := json.Marshal(rep.Error.Details)
		json.Unmarshal(b, &details)
	}
	if exit != exitError || rep.Error.Code != "unsaved_changes" || len(details.Files) != 1 || details.Files[0] != "src/vfx.ts" {
		t.Fatalf("a change the merge writes: %d %+v", exit, rep.Error)
	}
	os.Remove(filepath.Join(a, "src", "vfx.ts"))
	os.WriteFile(filepath.Join(a, "src", "main.ts"), []byte("export const mine = 1;\n"), 0o644)
	exit, rep = run(t, a, "merge", "vfx")
	var res syncJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || res.Action != "merged" || !res.KeptWork {
		t.Fatalf("merge: %d %s %+v", exit, rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || len(st.Changes) != 1 || st.Changes[0].Path != "src/main.ts" || st.Version != res.To {
		t.Fatalf("status after: %s", rep.Result)
	}
}
