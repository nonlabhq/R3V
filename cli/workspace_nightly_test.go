//go:build nightly

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
		// The agent says what its tests said (r3v result), seen from the main copy below.
		if exit, rep := run(t, ws, "result", "tests", "pass", "-m", "cargo test: 3 passed"); exit != 0 {
			t.Fatalf("result: %+v", rep.Error)
		}
		if exit, rep := run(t, ws, "results"); exit != 0 || !strings.Contains(string(rep.Result), "cargo test: 3 passed") {
			t.Fatalf("results: %s %+v", rep.Result, rep.Error)
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
		ls.Workspaces[0].Changes != 0 || ls.Workspaces[0].Results["tests"].State != "pass" {
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

// A workspace made for an agent: its branch is an agent's, the agent's tool
// is set up in it; removed once merged, its branch is archived.
func TestWorkspaceForAnAgent(t *testing.T) {
	a := teamProject(t)
	ws := filepath.Join(t.TempDir(), "renderer")
	exit, rep := run(t, a, "workspace", "add", ws, "--branch", "renderer", "--agent", "claude-code")
	if exit != 0 {
		t.Fatalf("add: %+v", rep.Error)
	}
	if _, err := os.Stat(filepath.Join(ws, ".claude", "settings.local.json")); err != nil {
		t.Fatalf("not set up: %v", err)
	}
	r, _ := project.Open(a)
	recs, _ := r.BranchRecords()
	if recs["renderer"].Kind != remote.BranchAgent {
		t.Fatalf("branch record: %+v", recs["renderer"])
	}
	// main moves on: the workspace hears that main has a version its
	// branch hasn't, and that merging it brings it in (update doesn't).
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("main moved on"), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "main moved on"); exit != 0 {
		t.Fatalf("commit on main: %+v", rep.Error)
	}
	t.Run("in the workspace", func(t *testing.T) {
		for _, cmd := range []string{"status", "update"} {
			exit, rep := run(t, ws, cmd)
			var got struct{ Base *baseJSON }
			json.Unmarshal(rep.Result, &got)
			if exit != 0 || got.Base == nil || got.Base.Branch != "main" || got.Base.Behind != 1 {
				t.Fatalf("%s: %s %+v", cmd, rep.Result, rep.Error)
			}
		}
	})
	if exit, rep := run(t, a, "workspace", "remove", ws); exit != 0 {
		t.Fatalf("remove: %+v", rep.Error)
	}
	if gone, _ := r.ArchivedBranches(); len(gone) != 1 || gone[0].Key != "renderer" {
		t.Fatalf("archived: %+v", gone)
	}
}

// Without a folder, a workspace goes in the project's own holder beside it
// (Song Project.workspaces), named after its branch; the holder goes with
// its last workspace.
func TestWorkspaceDefaultFolder(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "one\n"})
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
	exit, rep := run(t, a, "workspace", "add", "--branch", "try/mix")
	var added struct{ Workspace workspaceJSON }
	json.Unmarshal(rep.Result, &added)
	holder := filepath.Join(filepath.Dir(a), "Song Project.workspaces")
	want := filepath.Join(holder, "try-mix")
	if exit != 0 || !strings.EqualFold(added.Workspace.Folder, want) {
		t.Fatalf("add: %s %+v (want %s)", rep.Result, rep.Error, want)
	}
	if _, err := os.Stat(filepath.Join(want, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "workspace", "remove", want, "--force"); exit != 0 {
		t.Fatalf("remove: %+v", rep.Error)
	}
	if _, err := os.Stat(holder); !os.IsNotExist(err) {
		t.Errorf("the holder is still there: %v", err)
	}
}
