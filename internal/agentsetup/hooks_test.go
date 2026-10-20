package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The guide gives every tool one command for working in parallel, and
// says never git worktree.
func TestGuideParallel(t *testing.T) {
	block := guideBlock()
	for _, want := range []string{"`r3v workspace add --branch NAME\n  --agent <your tool> --json`", "Never `git worktree`"} {
		if strings.Count(block, want) != 1 {
			t.Fatalf("the guide lacks %q:\n%s", want, block)
		}
	}
}

// Claude Code's settings get the worktree and model hooks beside the
// user's own (their hooks for the same events, their other settings);
// again, nothing changes; the pre-tool hook is asked about the shells too.
func TestClaudeCodeWorktreeAndModelHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"permissions": {"allow": ["Bash(npm test)"]},
  "hooks": {"SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}]}}`), 0o644)
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
	for event, want := range map[string]string{"WorktreeCreate": "r3v hook worktree-create",
		"WorktreeRemove": "r3v hook worktree-remove", "PostModelSwitch": "r3v hook model-switch"} {
		if es := s.Hooks[event]; len(es) != 1 || es[0].Matcher != "" || len(es[0].Hooks) != 1 ||
			es[0].Hooks[0].Command != want || es[0].Hooks[0].Type != "command" {
			t.Fatalf("%s: %s", event, data)
		}
	}
	if es := s.Hooks["SessionStart"]; len(es) != 2 || es[0].Hooks[0].Command != "echo hi" ||
		es[1].Hooks[0].Command != "r3v hook session-start" {
		t.Fatalf("SessionStart: %s", data)
	}
	if es := s.Hooks["PreToolUse"]; len(es) != 1 || es[0].Matcher != "Edit|Write|MultiEdit|NotebookEdit|Bash|PowerShell" {
		t.Fatalf("PreToolUse: %s", data)
	}
	if changed, err := ClaudeCode(dir, "r3v hook pre-tool", false, false); err != nil || len(changed) != 0 {
		t.Fatalf("again: %v %v", changed, err)
	}

	// Set up before (an older matcher): brought up to date in its place.
	os.WriteFile(path, []byte(`{"hooks": {"PreToolUse": [{"matcher": "Edit|Write|MultiEdit|NotebookEdit", "hooks": [{"type": "command", "command": "r3v hook pre-tool"}]}]}}`), 0o644)
	if changed, _ := ClaudeCode(dir, "r3v hook pre-tool", false, false); len(changed) != 1 {
		t.Fatal("an older setup not brought up to date")
	}
	data, _ = os.ReadFile(path)
	s.Hooks = nil
	json.Unmarshal(data, &s)
	if es := s.Hooks["PreToolUse"]; len(es) != 1 || es[0].Matcher != HookMatcher || len(s.Hooks["WorktreeCreate"]) != 1 {
		t.Fatalf("updated: %s", data)
	}
}
