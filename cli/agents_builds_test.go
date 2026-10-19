package cli

import (
	"strings"
	"testing"
)

// The agents' commands are in every build since 0.3 (Stable too): each is
// registered, takes --json and has its lines in the usage, none of them
// marked Nightly.
func TestAgentCommandsInEveryBuild(t *testing.T) {
	u := fullUsage()
	for c, line := range map[string]string{
		"steps": "\n  steps [--session ID]", "step": "\n  step start -m", "result": "\n  result <check>",
		"results": "\n  results [VERSION]", "lock": "\n  lock <paths…>", "unlock": "\n  unlock <paths…>",
		"locks": "\n  locks ", "workspace": "\n  workspace add", "undo-merge": "\n  undo-merge VERSION",
		"agent": "\n  agent setup [--tool NAME]",
	} {
		if _, ok := extraCommands[c]; !ok || !jsonCommands[c] {
			t.Errorf("r3v %s isn't in this build", c)
		}
		if !strings.Contains(u, line) {
			t.Errorf("r3v -h lacks %q", line)
		}
	}
	for _, head := range []string{"AI agents on one computer:", "steps (an agent's moves, on this computer):",
		"results (what builds and tests said about a version):", "taking back a merge:", "workspaces (several agents on one project):"} {
		if !strings.Contains(u, "\n"+head+"\n") {
			t.Errorf("r3v -h lacks %q", head)
		}
	}
}

// Teams by name are in every build since 0.2 (R3V Cloud teams are): --team
// looks the team up (TestTeamByName), as agent setup is
// (TestAgentSetupAnyTool).
func TestTeamByNameInEveryBuild(t *testing.T) {
	noTeams(t)
	if _, err := joinedTeam("Band"); err == nil || !strings.Contains(err.Error(), "hasn't joined") {
		t.Fatalf("--team: %v", err)
	}
	if !strings.Contains(fullUsage(), "\n  init --team NAME") {
		t.Fatal("r3v -h lacks --team")
	}
}
