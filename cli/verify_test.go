package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/store"
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

// verify --json with problems not repaired fails (verify_problems, exit 1,
// as the text output exits), its result in the error's details; repaired,
// it succeeds.
func TestVerifyProblemsFail(t *testing.T) {
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "kick.wav": "kick"})
	r, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	kick, _, _ := store.HashFile(filepath.Join(a, "kick.wav"))
	os.WriteFile(r.Store.Path(kick), []byte("kicx"), 0o644) // a bad sector
	exit, rep := run(t, a, "verify")
	if exit != exitError || rep.Error == nil || rep.Error.Code != "verify_problems" || rep.Result != nil {
		t.Fatalf("verify: %d %+v", exit, rep.Error)
	}
	var v verifyJSON
	if b, _ := json.Marshal(rep.Error.Details); json.Unmarshal(b, &v) != nil || v.OpenProblems != 1 ||
		len(v.Problems) != 1 || v.Problems[0].Path != "kick.wav" || v.Summary == "" {
		t.Fatalf("details: %+v", rep.Error.Details)
	}
	t.Chdir(a)
	if code := Run([]string{"verify"}); code != exitError { // (text: as before)
		t.Fatalf("text: exit %d", code)
	}
	exit, rep = run(t, a, "verify", "--repair")
	json.Unmarshal(rep.Result, &v)
	if exit != 0 || v.OpenProblems != 0 || len(v.Problems) != 1 || !v.Problems[0].Fixed {
		t.Fatalf("repaired: %d %s %+v", exit, rep.Result, rep.Error)
	}
}
