package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// workspace list --against counts against another branch for one call;
// workspace base makes it the one the workspace merges into.
func TestWorkspaceBaseCommand(t *testing.T) {
	a := teamProject(t)
	ws := filepath.Join(t.TempDir(), "lane")
	if exit, rep := run(t, a, "workspace", "add", ws, "--branch", "lane"); exit != 0 {
		t.Fatalf("add: %+v", rep.Error)
	}
	if exit, rep := run(t, a, "branch", "new", "dev"); exit != 0 {
		t.Fatalf("branch new: %+v", rep.Error)
	}
	writeFile(t, a, "src/main.ts", "export const dev = 1;\n")
	if exit, rep := run(t, a, "commit", "-m", "on dev"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	type listed struct {
		Against    string
		Workspaces []workspaceJSON
	}
	list := func(args ...string) listed {
		t.Helper()
		exit, rep := run(t, a, append([]string{"workspace", "list"}, args...)...)
		var got listed
		json.Unmarshal(rep.Result, &got)
		if exit != 0 || len(got.Workspaces) != 1 || got.Workspaces[0].Behind == nil {
			t.Fatalf("list %v: %s %+v", args, rep.Result, rep.Error)
		}
		return got
	}
	if got := list(); got.Against != "" || got.Workspaces[0].Base != "main" || *got.Workspaces[0].Behind != 0 {
		t.Fatalf("list: %+v", got)
	}
	if got := list("--against", "dev"); got.Against != "dev" || got.Workspaces[0].Base != "main" || *got.Workspaces[0].Behind != 1 {
		t.Fatalf("list --against dev: %+v", got)
	}
	exit, rep := run(t, a, "workspace", "base", ws, "dev")
	var set struct{ Workspace workspaceJSON }
	json.Unmarshal(rep.Result, &set)
	if exit != 0 || set.Workspace.Base != "dev" || set.Workspace.Branch != "lane" {
		t.Fatalf("base: %s %+v", rep.Result, rep.Error)
	}
	if got := list(); got.Workspaces[0].Base != "dev" || *got.Workspaces[0].Behind != 1 {
		t.Fatalf("list after base: %+v", got)
	}
	if exit, _ := run(t, a, "workspace", "base", ws, "no-such-branch"); exit == 0 {
		t.Error("a base that isn't there")
	}
	if exit, rep := run(t, a, "workspace", "base", ws); exit == 0 {
		t.Errorf("no branch: %d %+v", exit, rep.Error)
	}
}
