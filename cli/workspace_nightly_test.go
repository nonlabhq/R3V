//go:build nightly

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// An agent's workspace from the command line: made, worked in (status,
// commit, log, update), listed from the main copy, refused removal until
// merged, then removed.
func TestWorkspaceCommands(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\nrules:\n  - ignore: \"target/\"\n",
		"src/main.rs": "fn main() {}\n", "target/debug/game": "built"})
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
	ws := filepath.Join(t.TempDir(), "agent")
	exit, rep := run(t, a, "workspace", "add", ws, "--branch", "agent")
	var added struct{ Workspace workspaceJSON }
	json.Unmarshal(rep.Result, &added)
	if exit != 0 || added.Workspace.Branch != "agent" || added.Workspace.Base != "main" {
		t.Fatalf("add: %s %+v", rep.Result, rep.Error)
	}
	if _, err := os.Stat(filepath.Join(ws, "target")); err == nil {
		t.Error("what the rules leave out was copied")
	}

	// In the workspace (a subtest: run keeps the folders it was in open
	// until its test ends, and they couldn't be removed).
	t.Run("in the workspace", func(t *testing.T) {
		os.WriteFile(filepath.Join(ws, "src", "main.rs"), []byte("fn main() { run() }\n"), 0o644)
		exit, rep := run(t, filepath.Join(ws, "src"), "status")
		var st statusJSON
		json.Unmarshal(rep.Result, &st)
		if exit != 0 || len(st.Changes) != 1 || st.Branch != "agent" || st.MainCopy != a {
			t.Fatalf("status: %s %+v", rep.Result, rep.Error)
		}
		if exit, rep := run(t, ws, "commit", "-m", "run"); exit != 0 {
			t.Fatalf("commit: %+v", rep.Error)
		}
		if exit, rep := run(t, ws, "update"); exit != 0 {
			t.Fatalf("update: %+v", rep.Error)
		}
		exit, rep = run(t, ws, "log")
		var lg struct{ Versions []versionJSON }
		json.Unmarshal(rep.Result, &lg)
		if exit != 0 || len(lg.Versions) != 2 || lg.Versions[0].Message != "run" {
			t.Fatalf("log: %s", rep.Result)
		}
	})

	// From the main copy.
	exit, rep = run(t, a, "workspace", "list")
	var ls struct {
		Main       string
		Workspaces []workspaceJSON
	}
	json.Unmarshal(rep.Result, &ls)
	if exit != 0 || len(ls.Workspaces) != 1 || ls.Workspaces[0].Ahead == nil || *ls.Workspaces[0].Ahead != 1 ||
		ls.Workspaces[0].Changes != 0 {
		t.Fatalf("list: %s", rep.Result)
	}
	if exit, rep := run(t, a, "workspace", "remove", ws); exit != 1 || rep.Error.Code != "workspace_not_merged" {
		t.Fatalf("remove before merging: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "merge", "agent"); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "workspace", "remove", ws); exit != 0 {
		t.Fatalf("remove: %+v", rep.Error)
	}
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}
}
