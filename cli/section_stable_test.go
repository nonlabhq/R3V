//go:build !nightly

package cli

import (
	"strings"
	"testing"
)

// Stable has no sections: no command, not in the usage.
func TestNoSectionsInStable(t *testing.T) {
	a := teamProject(t)
	if exit, rep := run(t, a, "section", "new", "Engine"); exit != exitUsage || rep.Error.Code != "usage" {
		t.Fatalf("section new in Stable: %d %+v", exit, rep.Error)
	}
	if strings.Contains(fullUsage(), "section new") {
		t.Fatal("section new in Stable's usage")
	}
}
