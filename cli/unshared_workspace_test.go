package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A version committed --local in a workspace isn't on the team's branch,
// so merging that branch from the main copy finds nothing: it says so
// (unshared_elsewhere, warning unshared_versions) instead of a bare
// up-to-date; status and workspace list count it. Once shared, the merge
// brings it in, and a merge with truly nothing left is up-to-date alone.
func TestUnsharedInAnotherCopy(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "src/main.ts": "export {};\n"})
	ra, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	ws := filepath.Join(t.TempDir(), "player")
	if exit, rep := run(t, a, "workspace", "add", ws, "--branch", "player"); exit != 0 {
		t.Fatalf("add: %+v", rep.Error)
	}

	t.Run("in the workspace", func(t *testing.T) {
		os.WriteFile(filepath.Join(ws, "src", "player.ts"), []byte("move"), 0o644)
		if exit, rep := run(t, ws, "commit", "--local", "-m", "Player moves"); exit != 0 {
			t.Fatalf("commit --local: %+v", rep.Error)
		}
		exit, rep := run(t, ws, "status")
		var st statusJSON
		json.Unmarshal(rep.Result, &st)
		if exit != 0 || st.Unshared == nil || *st.Unshared != 1 {
			t.Fatalf("status in the workspace: %s", rep.Result)
		}
	})

	exit, rep := run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || st.Unshared == nil || *st.Unshared != 0 {
		t.Fatalf("status in the main copy: %s", rep.Result)
	}
	for _, args := range [][]string{{"merge", "player", "--preview"}, {"merge", "player"}} {
		exit, rep := run(t, a, args...)
		var res struct {
			Action            string
			UnsharedElsewhere []unsharedJSON `json:"unshared_elsewhere"`
			Warnings          []struct {
				Code, Reason, Workspace, Branch string
				Count                           int
			}
		}
		json.Unmarshal(rep.Result, &res)
		if exit != 0 || len(res.UnsharedElsewhere) != 1 || res.UnsharedElsewhere[0].Count != 1 ||
			res.UnsharedElsewhere[0].Branch != "player" || res.UnsharedElsewhere[0].MainCopy ||
			!strings.EqualFold(filepath.Clean(res.UnsharedElsewhere[0].Workspace), ws) ||
			len(res.Warnings) != 1 || res.Warnings[0].Code != "unshared_versions" || res.Warnings[0].Count != 1 ||
			strings.Contains(string(rep.Result), `"folder"`) { // (not a looks_built warning's fields)
			t.Fatalf("%s: %s", strings.Join(args, " "), rep.Result)
		}
		if res.Action == "merged" || res.Action == "merge" {
			t.Fatalf("%s: nothing the team has to merge: %s", strings.Join(args, " "), rep.Result)
		}
	}
	exit, text := runText(t, a, "merge", "player")
	if exit != 0 || !strings.Contains(text, "1 version(s) on player not shared yet") {
		t.Fatalf("merge, text: %s", text)
	}
	exit, rep = run(t, a, "workspace", "list")
	var ls struct{ Workspaces []workspaceJSON }
	json.Unmarshal(rep.Result, &ls)
	if exit != 0 || len(ls.Workspaces) != 1 || ls.Workspaces[0].Unshared == nil || *ls.Workspaces[0].Unshared != 1 {
		t.Fatalf("workspace list: %s", rep.Result)
	}

	t.Run("shared", func(t *testing.T) {
		os.WriteFile(filepath.Join(ws, "src", "player.ts"), []byte("move and jump"), 0o644)
		if exit, rep := run(t, ws, "commit", "-m", "Player jumps"); exit != 0 {
			t.Fatalf("commit: %+v", rep.Error)
		}
	})
	exit, rep = run(t, a, "merge", "player")
	var res syncJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || res.Action != "merged" || len(res.UnsharedElsewhere) != 0 || len(res.Warnings) != 0 {
		t.Fatalf("merge once shared: %s %+v", rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "merge", "player")
	if exit != 0 || strings.Contains(string(rep.Result), "unshared") || !strings.Contains(string(rep.Result), `"action": "ahead"`) &&
		!strings.Contains(string(rep.Result), `"action": "up-to-date"`) {
		t.Fatalf("nothing left: %s", rep.Result)
	}
}
