package agentsetup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCodeWorkspace(t *testing.T) {
	dir := t.TempDir()
	changed, err := ClaudeCode(dir, HookCommand(""), false, false)
	if err != nil || len(changed) != 1 || changed[0] != ".claude/settings.local.json" {
		t.Fatalf("changed %v, %v", changed, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var s struct {
		Env   map[string]string `json:"env"`
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Env["R3V_ACTOR"] != ClaudeCodeTool {
		t.Fatalf("no actor in env: %s", data)
	}
	if len(s.Hooks.PreToolUse) != 1 || s.Hooks.PreToolUse[0].Matcher != HookMatcher ||
		!strings.HasSuffix(s.Hooks.PreToolUse[0].Hooks[0].Command, "hook pre-tool") {
		t.Fatalf("hook: %s", data)
	}
	// A workspace gets no guide (it is the main copy's, committed).
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("CLAUDE.md written in a workspace: %v", err)
	}
	// Again: nothing to change.
	if changed, err := ClaudeCode(dir, HookCommand(""), false, false); err != nil || len(changed) != 0 {
		t.Fatalf("again: %v, %v", changed, err)
	}
}

func TestHookCommandQuotesAPath(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "bin", "r3v.exe")
	if got := HookCommand(exe); got != `"`+filepath.ToSlash(exe)+`" hook pre-tool` {
		t.Fatalf("got %q", got)
	}
}

// With steps, a prompt hook and a post-tool hook beside the project's own;
// without, ours go and the project's stay.
func TestClaudeCodeStepHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"hooks": {"PostToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "prettier --write"}]}]}}`), 0o644)
	if StepHooks(dir) {
		t.Fatal("hooked before")
	}
	if _, err := ClaudeCode(dir, HookCommand(""), false, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{"r3v hook pre-tool", "r3v hook post-tool", "r3v hook prompt", "UserPromptSubmit", "prettier --write"} {
		if !strings.Contains(text, want) {
			t.Fatalf("no %q: %s", want, text)
		}
	}
	if !StepHooks(dir) {
		t.Fatal("not hooked")
	}
	if changed, _ := ClaudeCode(dir, HookCommand(""), false, true); len(changed) != 0 {
		t.Fatalf("again: %v", changed)
	}
	if _, err := ClaudeCode(dir, HookCommand(""), false, false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	text = string(data)
	if strings.Contains(text, "hook post-tool") || strings.Contains(text, "hook prompt") || strings.Contains(text, "UserPromptSubmit") ||
		!strings.Contains(text, "prettier --write") || !strings.Contains(text, "hook pre-tool") {
		t.Fatalf("steps off: %s", text)
	}
	if StepHooks(dir) {
		t.Fatal("hooked with steps off")
	}
}
