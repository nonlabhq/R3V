//go:build nightly

package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// The Agents view: a workspace made (set up for Claude Code), its work
// counted against main, its locks, opened as a project, refused removal
// while it has work, then removed.
func TestAppAgentWorkspaces(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)

	st, err := a.State(root)
	if err != nil || !st.AgentWorkspaces || st.WorkspaceCount != 0 || st.WorkspaceOf != "" {
		t.Fatalf("state: %+v %v", st, err)
	}
	if list, err := a.AgentWorkspaces(root); err != nil || len(list) != 0 {
		t.Fatalf("none yet: %+v %v", list, err)
	}

	folder := filepath.Join(t.TempDir(), "Song (renderer)")
	if _, err := a.AddWorkspace(root, "relative", "renderer", "", ""); err == nil {
		t.Error("a relative folder: made")
	}
	added, err := a.AddWorkspace(root, folder, "renderer", "", "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if added.Label != "renderer" || added.Agent != "claude-code" || added.SetupError != "" {
		t.Fatalf("added: %+v", added)
	}
	if _, err := os.Stat(filepath.Join(folder, ".claude", "settings.local.json")); err != nil {
		t.Errorf("not set up for Claude Code: %v", err)
	}
	for _, dir := range []string{folder, root} {
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
			t.Errorf("CLAUDE.md written in %s (the person commits the guide, not the app)", dir)
		}
	}
	if st, _ := a.State(root); st.WorkspaceCount != 1 {
		t.Errorf("count: %d", st.WorkspaceCount)
	}
	if st, _ := a.State(folder); st.AgentWorkspaces || st.WorkspaceOf != root {
		t.Errorf("the workspace's own state: agents %v of %q", st.AgentWorkspaces, st.WorkspaceOf)
	}

	// Its agent commits and shares one version, and is at work on another.
	os.WriteFile(filepath.Join(folder, "Samples", "hat.wav"), []byte("RIFF-hat"), 0o644)
	if _, err := a.Save(folder, "a hat", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(folder, "Samples", "kick.wav"), []byte("RIFF-kick-2"), 0o644)
	ws, err := project.Open(folder)
	if err != nil {
		t.Fatal(err)
	}
	h := agentlock.Holder{Workspace: ws.Config.WorkspaceID, Branch: ws.BranchName(), Actor: "claude-code"}
	if _, err := ws.AgentLockList().Take(h, []string{"Samples/kick.wav"}, agentlock.KindLock, nil); err != nil {
		t.Fatal(err)
	}
	list, err := a.AgentWorkspaces(root)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	w := list[0]
	if w.Folder != folder || w.Label != "renderer" || w.Base != "main" || w.BaseLabel == "" || w.Missing || w.Opened {
		t.Errorf("workspace: %+v", w)
	}
	if w.Ahead == nil || *w.Ahead != 1 || w.Behind == nil || *w.Behind != 0 || w.Changes != 1 {
		t.Errorf("where it is: ahead %v behind %v changes %d", w.Ahead, w.Behind, w.Changes)
	}
	if w.Tool != "claude-code" || len(w.Locks) != 1 || w.Locks[0].Path != "Samples/kick.wav" || w.LastActivity == "" {
		t.Errorf("its agent: tool %q locks %+v last %q", w.Tool, w.Locks, w.LastActivity)
	}

	// Opened: listed beside its project; the app knows the folder.
	p, err := a.OpenWorkspace(root, folder)
	if err != nil || !p.Workspace || p.MainRoot != root || p.Root != folder || p.Branch != w.Branch {
		t.Fatalf("opened: %+v %v", p, err)
	}
	if _, err := a.OpenWorkspace(root, t.TempDir()); err == nil {
		t.Error("opened a folder that isn't a workspace")
	}
	if !knownProject(folder) {
		t.Error("the app doesn't know the workspace's folder")
	}
	ov, err := a.LocalOverview()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ov.Projects); n != 2 || ov.Projects[0].Root != root || !ov.Projects[1].Workspace || ov.Projects[1].Root != folder {
		t.Errorf("overview: %+v", ov.Projects)
	}
	if list, _ := a.AgentWorkspaces(root); !list[0].Opened {
		t.Error("not opened")
	}

	// Removing: refused with what's left, nothing changed.
	rm, err := a.RemoveWorkspace(root, folder, false)
	if err != nil || rm.Removed || rm.Left == nil || rm.Left.Unmerged != 1 || rm.Left.Changes != 1 || rm.Left.Base == "" {
		t.Fatalf("refused: %+v %v", rm, err)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Fatalf("the folder went: %v", err)
	}

	// Merged into main: still its change not committed; forced, it goes.
	if res, err := a.MergeBranch(root, w.Branch, "", nil, true); err != nil || len(res.Conflicts) != 0 {
		t.Fatalf("merge: %+v %v", res, err)
	}
	if list, _ := a.AgentWorkspaces(root); *list[0].Ahead != 0 {
		t.Errorf("merged, still ahead %d", *list[0].Ahead)
	}
	if rm, err := a.RemoveWorkspace(root, folder, false); err != nil || rm.Left == nil || rm.Left.Unmerged != 0 || rm.Left.Changes != 1 {
		t.Fatalf("refused for the change: %+v %v", rm, err)
	}
	if rm, err := a.RemoveWorkspace(root, folder, true); err != nil || !rm.Removed {
		t.Fatalf("forced: %+v %v", rm, err)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Errorf("the folder stays: %v", err)
	}
	if list, _ := a.AgentWorkspaces(root); len(list) != 0 {
		t.Errorf("still listed: %+v", list)
	}
	if s, _ := teams.Load(); len(s.Workspaces) != 0 {
		t.Errorf("still shown as a project: %v", s.Workspaces)
	}
	if locks, _ := ws.AgentLockList().All(); len(locks) != 0 {
		t.Errorf("its locks stay: %+v", locks)
	}
}

// A workspace with nothing left to merge goes without asking, its branch
// archived.
func TestAppRemoveWorkspaceDone(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	folder := filepath.Join(t.TempDir(), "Song (idea)")
	added, err := a.AddWorkspace(root, folder, "idea", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, ".claude")); !os.IsNotExist(err) {
		t.Error("set up for an agent without asking")
	}
	rm, err := a.RemoveWorkspace(root, folder, false)
	if err != nil || !rm.Removed || rm.Left != nil {
		t.Fatalf("removed: %+v %v", rm, err)
	}
	gone, err := a.ArchivedBranches(root)
	if err != nil || len(gone) != 1 || gone[0].Name != added.Branch {
		t.Errorf("its branch archived: %+v %v", gone, err)
	}
}
