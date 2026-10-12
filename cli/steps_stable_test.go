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
