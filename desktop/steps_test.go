package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
)

// The main copy, where no workspace set Claude Code up: the app says its
// steps aren't made, and sets it up.
func TestAppSetUpSteps(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	info, err := a.Steps(root, "")
	if err != nil || !info.Available || info.Hooked {
		t.Fatalf("before: %+v %v", info, err)
	}
	if err := a.SetUpSteps(root); err != nil {
		t.Fatal(err)
	}
	if info, err := a.Steps(root, ""); err != nil || !info.Hooked || info.Tool != agentsetup.ClaudeCodeTool {
		t.Fatalf("after: %+v %v", info, err)
	}
}

// A tool without hooks makes its steps with step start and end: the app
// lists them as the hooks' (the task's note, its step by that tool), and
// knows the copy is that tool's (no Claude Code hooks to offer).
func TestAppReadsStepTasks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root := teamSong(t, a)
	if _, _, err := agentsetup.Setup(root, filepath.Join(root, ".r3v"), agentsetup.Options{Tool: "codex"}); err != nil {
		t.Fatal(err)
	}
	r, err := project.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	r.Actor = &project.Actor{Kind: "agent", Tool: "codex", Session: "th-1"}
	if _, err := r.StepStart("Brighter chorus"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("brighter"), 0o644)
	if _, err := r.StepEnd("Raised the chorus"); err != nil {
		t.Fatal(err)
	}
	info, err := a.Steps(root, "")
	if err != nil || info.Tool != "codex" || info.Session == nil || info.Session.ID != "th-1" ||
		len(info.Sessions) != 1 || info.Sessions[0].Prompt != "Brighter chorus" {
		t.Fatalf("steps: %+v %v", info, err)
	}
	st := info.Session.Turns[0].Steps
	if len(st) != 1 || st[0].Tool != project.ToolStepEnd || st[0].Actor != "codex" || st[0].Detail != "Raised the chorus" ||
		strings.Join(st[0].Files, ",") != "notes.txt" {
		t.Fatalf("step: %+v", st)
	}
}
