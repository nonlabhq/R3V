package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Workflow settings (docs/design/team-settings.md) on a team's own
// storage: anyone sets the team's defaults for agents and a project's own;
// this computer's copy reads them, the agents' steps follow them, and the
// team puts every project back to its own in one go.
func TestWorkflowSettings(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	store, _ := teams.Load()
	teamID := store.Teams[0].ID

	v, err := a.TeamWorkflow(teamID)
	if err != nil || !v.CanEdit || !v.Agents.Steps || !v.Agents.Prompts || !v.Agents.AgentLocks || v.Locks {
		t.Fatalf("a team's own storage, as by default: %+v, %v", v, err)
	}

	// The team: no prompts kept; the project: no steps at all.
	if err := a.SetTeamAgents(teamID, AgentWorkflow{Steps: true, Prompts: false, AgentLocks: true}); err != nil {
		t.Fatal(err)
	}
	off := false
	if err := a.SetProjectWorkflow(root, remote.Workflow{Steps: &off}); err != nil {
		t.Fatal(err)
	}
	pw, err := a.ProjectWorkflow(root)
	if err != nil || pw.TeamAgents.Prompts || pw.Steps == nil || *pw.Steps || pw.Prompts != nil {
		t.Fatalf("the project's view: %+v, %v", pw, err)
	}
	r, _ := project.Open(root)
	if w := r.Workflow(); w.StepsByTeam() || w.PromptsByTeam() || !w.AgentLocksOn() {
		t.Errorf("this computer's copy: %+v", w)
	}
	if project.Steps && r.StepsOn() {
		t.Error("steps made where the project turned them off")
	}

	v, _ = a.TeamWorkflow(teamID)
	if len(v.Overrides) != 1 || !v.Overrides[0].Agents {
		t.Fatalf("overrides: %+v", v.Overrides)
	}
	if n, err := a.ClearProjectWorkflow(teamID, "agents"); err != nil || n != 1 {
		t.Fatalf("cleared %d, %v", n, err)
	}
	if pw, _ := a.ProjectWorkflow(root); pw.Steps != nil {
		t.Errorf("still its own: %+v", pw)
	}
}

// A project's .r3v.yaml file_locks: are no longer read; moving them takes
// them out of the file (on a team without file locking, only that).
func TestMoveFileLocksOutOfRules(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	path := filepath.Join(root, ".r3v.yaml")
	os.WriteFile(path, []byte("# what's tracked\nrules:\n  - ignore: \"Backup/\"\n# locks\nfile_locks:\n  enabled: false\n"), 0o644)
	if pw, err := a.ProjectWorkflow(root); err != nil || !pw.RulesLocks {
		t.Fatalf("not offered: %+v, %v", pw, err)
	}
	if err := a.MoveFileLocksToSettings(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "file_locks") || strings.Contains(string(data), "# locks") || !strings.Contains(string(data), "Backup/") {
		t.Errorf(".r3v.yaml now:\n%s", data)
	}
}
