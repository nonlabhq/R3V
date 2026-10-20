package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The guide gives every tool one command for working in parallel, says
// why not git worktree, and to start the tool in the workspace.
func TestGuideParallel(t *testing.T) {
	block := guideBlock()
	for _, want := range []string{"`r3v workspace add --branch NAME --agent <your tool> --json`",
		"R3V doesn't see a git worktree", "start your tool in that folder"} {
		if strings.Count(strings.ReplaceAll(block, "\n  ", " "), want) != 1 {
			t.Fatalf("the guide lacks %q:\n%s", want, block)
		}
	}
}

// Claude Code's settings get the model hooks beside the user's own (their
// hooks for the same events, their other settings); worktree hooks an
// earlier build wrote are taken out, the user's own for those events kept;
// again, nothing changes.
func TestClaudeCodeModelHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"permissions": {"allow": ["Bash(npm test)"]},
  "hooks": {"SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}],
    "PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit|Bash|PowerShell", "hooks": [{"type": "command", "command": "r3v hook pre-tool"}]}],
    "WorktreeCreate": [{"hooks": [{"type": "command", "command": "r3v hook worktree-create"}]}],
    "WorktreeRemove": [{"hooks": [{"type": "command", "command": "r3v hook worktree-remove"}, {"type": "command", "command": "my-cleanup"}]}]}}`), 0o644)
	if _, err := ClaudeCode(dir, "r3v hook pre-tool", false, false); err != nil {
		t.Fatal(err)
	}
	type hook struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type entry struct {
		Matcher string `json:"matcher"`
		Hooks   []hook `json:"hooks"`
	}
	var s struct {
		Permissions map[string][]string `json:"permissions"`
		Hooks       map[string][]entry  `json:"hooks"`
	}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Permissions["allow"]) != 1 {
		t.Fatalf("the user's settings: %s", data)
	}
	if es := s.Hooks["PostModelSwitch"]; len(es) != 1 || es[0].Matcher != "" || len(es[0].Hooks) != 1 ||
		es[0].Hooks[0].Command != "r3v hook model-switch" || es[0].Hooks[0].Type != "command" {
		t.Fatalf("PostModelSwitch: %s", data)
	}
	if es := s.Hooks["SessionStart"]; len(es) != 2 || es[0].Hooks[0].Command != "echo hi" ||
		es[1].Hooks[0].Command != "r3v hook session-start" {
		t.Fatalf("SessionStart: %s", data)
	}
	if es := s.Hooks["PreToolUse"]; len(es) != 1 || es[0].Matcher != "Edit|Write|MultiEdit|NotebookEdit" {
		t.Fatalf("PreToolUse: %s", data)
	}
	if _, ok := s.Hooks["WorktreeCreate"]; ok {
		t.Fatalf("WorktreeCreate kept: %s", data)
	}
	if es := s.Hooks["WorktreeRemove"]; len(es) != 1 || len(es[0].Hooks) != 1 || es[0].Hooks[0].Command != "my-cleanup" {
		t.Fatalf("WorktreeRemove: %s", data)
	}
	if changed, err := ClaudeCode(dir, "r3v hook pre-tool", false, false); err != nil || len(changed) != 0 {
		t.Fatalf("again: %v %v", changed, err)
	}
}
