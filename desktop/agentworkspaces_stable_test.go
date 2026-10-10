//go:build !nightly

package desktop

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
)

// Stable offers no Agents view and makes no workspaces.
func TestAgentWorkspacesStable(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	if st, err := a.State(root); err != nil || st.AgentWorkspaces {
		t.Fatalf("state: agents %v %v", st.AgentWorkspaces, err)
	}
	_, err := a.AddWorkspace(root, filepath.Join(t.TempDir(), "ws"), "renderer", "", "")
	if !errors.Is(err, project.ErrWorkspacesNotInBuild) {
		t.Fatalf("made one: %v", err)
	}
}
