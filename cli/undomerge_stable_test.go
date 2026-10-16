//go:build !nightly

package cli

import (
	"strings"
	"testing"
)

// Stable can't undo a merge: no command, not in the usage.
func TestNoUndoMergeInStable(t *testing.T) {
	a := laneProject(t)
	if exit, rep := run(t, a, "undo-merge", "HEAD"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Fatalf("undo-merge in Stable: %d %+v", exit, rep.Error)
	}
	if strings.Contains(fullUsage(), "undo-merge") {
		t.Fatal("undo-merge in Stable's usage")
	}
}
