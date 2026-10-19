package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// verify says what is where: the versions in this branch's history (as
// log counts them) and the others, the files kept here and those only in
// the team's storage, so few files here after sharing doesn't read as
// history lost.
func TestVerifySaysWhatIsWhere(t *testing.T) {
	a, _ := behindLane(t) // main, and vfx one version ahead of it
	exit, rep := run(t, a, "verify")
	var v verifyJSON
	json.Unmarshal(rep.Result, &v)
	if exit != 0 || v.Versions != 2 || v.InHistory != 1 || v.OtherVersions != 1 || v.OpenProblems != 0 ||
		len(v.Problems) != 0 || !v.TeamChecked || v.LocalFiles+v.TeamOnlyFiles+v.FolderOnlyFiles == 0 {
		t.Fatalf("verify: %d %s %+v", exit, rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "log")
	var lg struct{ Versions []versionJSON }
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != v.InHistory {
		t.Fatalf("log lists %d, verify says %d in the history", len(lg.Versions), v.InHistory)
	}
	if !strings.Contains(v.Summary, "1 in this branch's history") || !strings.Contains(v.Summary, "1 other") {
		t.Fatalf("summary: %s", v.Summary)
	}
	if v.TeamOnlyFiles > 0 && !strings.Contains(v.Summary, "only in the team's storage") {
		t.Fatalf("summary: %s", v.Summary)
	}
	t.Log(v.Summary)
}
