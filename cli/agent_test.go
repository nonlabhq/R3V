package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/project"
)

// The agents of the field tests (opencode, Pi) set nothing R3V reads:
// `agent setup --tool` sets their copy up, so the versions they commit
// there say so. No Claude Code settings for them, the guide in AGENTS.md
// beside the project's own lines, and again changes nothing.
func TestAgentSetupAnyTool(t *testing.T) {
	asUnknown(t)
	dir := newFolder(t, map[string]string{"notes.txt": "n",
		"AGENTS.md": "# Our project\r\nRun the tests before you commit.\r\n"})
	if _, err := project.Init(dir, "yi"); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, dir, "agent", "setup"); exit != 2 || rep.Error.Code != "usage" || !strings.Contains(rep.Error.Message, "--tool") {
		t.Fatalf("no tool, none R3V can tell: %d %+v", exit, rep.Error)
	}

	exit, rep := run(t, dir, "agent", "setup", "--tool", "OpenCode", "--model=glm-4.6")
	var out struct {
		Tool, Model  string
		FilesWritten []string `json:"files_written"`
		Changed      []string
		Identity     agentIdentityJSON
	}
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Tool != "opencode" || out.Model != "glm-4.6" ||
		strings.Join(out.FilesWritten, ",") != "AGENTS.md,.r3v/agent.json" || len(out.Changed) != 2 ||
		out.Identity != (agentIdentityJSON{Tool: "opencode", Model: "glm-4.6", File: ".r3v/agent.json"}) {
		t.Fatalf("setup: %d %s %+v", exit, rep.Result, rep.Error)
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.HasPrefix(string(agents), "# Our project\r\nRun the tests before you commit.\r\n\r\n<!-- R3V: ") ||
		!strings.Contains(string(agents), "r3v help agents") {
		t.Fatalf("AGENTS.md: %q", agents)
	}
	for _, f := range []string{"CLAUDE.md", ".claude"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("%s written for opencode", f)
		}
	}

	// Again, without the model: nothing changes (the model is kept).
	exit, rep = run(t, dir, "agent", "setup", "--tool", "opencode")
	if exit != 0 || !strings.Contains(string(rep.Result), `"files_written": []`) || !strings.Contains(string(rep.Result), `"model": "glm-4.6"`) {
		t.Fatalf("again: %s", rep.Result)
	}
	if again, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); string(again) != string(agents) {
		t.Fatal("AGENTS.md changed")
	}

	// Its commits say opencode; the person's own (--as person) don't.
	commit := func(msg string, args ...string) *project.Actor {
		t.Helper()
		writeFile(t, dir, "notes.txt", msg)
		exit, rep := run(t, dir, append([]string{"commit", "-m", msg}, args...)...)
		var sv syncJSON
		json.Unmarshal(rep.Result, &sv)
		if exit != 0 || sv.Saved == nil {
			t.Fatalf("commit %s: %s %+v", msg, rep.Result, rep.Error)
		}
		return sv.Saved.Actor
	}
	if a := commit("by opencode"); a == nil || a.Tool != "opencode" || a.Model != "glm-4.6" {
		t.Fatalf("opencode's commit: %+v", a)
	}
	if a := commit("by hand", "--as", "person"); a != nil {
		t.Fatalf("the person's commit: %+v", a)
	}
	t.Setenv(actor.EnvTool, "codex")
	if a := commit("by codex"); a == nil || a.Tool != "codex" || a.Model != "" {
		t.Fatalf("R3V_ACTOR wins: %+v", a)
	}
	if exit, rep := run(t, dir, "status", "--as", "robot"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("--as robot: %+v", rep.Error)
	}
}
