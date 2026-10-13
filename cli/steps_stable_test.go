//go:build !nightly

package cli

import (
	"bytes"
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

// Nor milestones (tag, tags).
func TestNoMilestonesInStable(t *testing.T) {
	for _, c := range []string{"milestone", "milestones", "tag", "tags"} {
		if _, ok := extraCommands[c]; ok {
			t.Fatalf("r3v %s in Stable", c)
		}
	}
}
