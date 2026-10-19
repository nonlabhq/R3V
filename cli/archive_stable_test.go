//go:build !nightly

package cli

import (
	"strings"
	"testing"
)

// Stable can't archive a project: no command, not in the usage.
func TestNoProjectArchiveInStable(t *testing.T) {
	a := teamProject(t)
	if exit, rep := run(t, a, "project", "archive"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Fatalf("project archive in Stable: %d %+v", exit, rep.Error)
	}
	if strings.Contains(fullUsage(), "project archive") {
		t.Fatal("project archive in Stable's usage")
	}
}
