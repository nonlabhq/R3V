//go:build !nightly

package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Stable makes no workspaces: nothing is written, nothing on the team.
func TestStableMakesNoWorkspace(t *testing.T) {
	if AgentWorkspaces {
		t.Fatal("workspaces are on in Stable")
	}
	_, code := cutTeam(t)
	r := codeProject(t, code)
	dir := filepath.Join(t.TempDir(), "agent")
	if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); !errors.Is(err, ErrWorkspacesNotInBuild) {
		t.Fatalf("AddWorkspace: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("a folder was made")
	}
	if bs, _ := r.Branches(); len(bs) != 1 {
		t.Errorf("branches: %+v", bs)
	}
}
