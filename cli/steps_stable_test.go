//go:build !nightly

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Stable has no steps: no commands, and the hooks say and do nothing.
func TestNoStepsInStable(t *testing.T) {
	if _, ok := extraCommands["steps"]; ok {
		t.Fatal("r3v steps in Stable")
	}
	var out bytes.Buffer
	hookSteps("post-tool", bytes.NewReader([]byte(`{"session_id":"s1","tool_name":"Write","cwd":"."}`)), &out)
	if out.Len() != 0 {
		t.Fatalf("said %q", out.String())
	}
}

// Stable has no results commands either.
func TestNoResultsInStable(t *testing.T) {
	if _, ok := extraCommands["result"]; ok {
		t.Fatal("r3v result in Stable")
	}
}

// Teams by name are Stable's since 0.2 (R3V Cloud teams are): --team
// looks the team up (TestTeamByName runs in every build), as agent setup
// is (TestAgentSetupAnyTool).
func TestTeamByNameAndAgentSetupInStable(t *testing.T) {
	noTeams(t)
	if _, err := joinedTeam("Band"); err == nil || !strings.Contains(err.Error(), "hasn't joined") {
		t.Fatalf("--team in Stable: %v", err)
	}
	if _, ok := extraCommands["agent"]; !ok || !jsonCommands["agent"] {
		t.Fatal("no r3v agent setup in Stable")
	}
	if u := fullUsage(); !strings.Contains(u, "\n  init --team NAME") || !strings.Contains(u, "\n  agent setup [--tool NAME]") {
		t.Fatal("r3v -h in Stable lacks --team or agent setup")
	}
}
