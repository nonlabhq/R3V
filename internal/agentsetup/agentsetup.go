// Package agentsetup sets a project's copy up for an AI agent tool
// (docs/design/agent-workspaces.md, "Setting agents up"): the hook that
// takes agent locks, R3V_ACTOR, and the agents' guide. Used by the CLI
// (`r3v agent setup`, `r3v workspace add --agent`) and the app.
package agentsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/docs"
	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/store"
)

// ClaudeCodeTool is the name of Claude Code, as R3V_ACTOR and --agent say it.
const ClaudeCodeTool = "claude-code"

// HookMatcher is the Claude Code tools the hook is asked about: those that
// change a file.
const HookMatcher = "Edit|Write|MultiEdit|NotebookEdit"

// HookCommand is how Claude Code runs the hook of the r3v program at exe:
// `r3v` when that is the r3v on PATH, else its full path (another r3v on
// PATH may be a build without agent locks, or none). exe "": `r3v`.
func HookCommand(exe string) string {
	if exe == "" {
		return "r3v hook pre-tool"
	}
	if p, err := exec.LookPath("r3v"); err == nil {
		a, _ := filepath.EvalSymlinks(p)
		b, _ := filepath.EvalSymlinks(exe)
		if a != "" && strings.EqualFold(a, b) {
			return "r3v hook pre-tool"
		}
	}
	return `"` + filepath.ToSlash(exe) + `" hook pre-tool`
}

// ClaudeCode writes Claude Code's settings for the copy in root
// (.claude/settings.local.json, which R3V never tracks: the hook's path and
// R3V_ACTOR are this computer's; merged with what's there) and, with guide,
// the agents' guide in CLAUDE.md (the project's: committed, so only in the
// main copy). Running it again changes nothing. It returns the files it
// wrote, relative to root.
func ClaudeCode(root, command string, guide bool) ([]string, error) {
	var changed []string
	path := filepath.Join(root, ".claude", "settings.local.json")
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &settings); err != nil {
			return nil, fmt.Errorf("%s isn't JSON R3V can read (%v): fix it, then run this again", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	before, _ := json.Marshal(settings)
	env, _ := settings["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	env[actor.EnvTool] = ClaudeCodeTool
	settings["env"] = env
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	pre, _ := hooks["PreToolUse"].([]any)
	ours := map[string]any{"type": "command", "command": command}
	found := false
	for _, e := range pre {
		entry, _ := e.(map[string]any)
		list, _ := entry["hooks"].([]any)
		for i, hk := range list {
			if m, _ := hk.(map[string]any); m != nil && IsOurHook(m["command"]) {
				list[i], found = ours, true
				entry["matcher"] = HookMatcher
			}
		}
	}
	if !found {
		pre = append(pre, map[string]any{"matcher": HookMatcher, "hooks": []any{ours}})
	}
	hooks["PreToolUse"] = pre
	settings["hooks"] = hooks
	if after, _ := json.Marshal(settings); string(after) != string(before) {
		out, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := store.WriteAtomic(path, strings.NewReader(string(out)+"\n")); err != nil {
			return nil, err
		}
		changed = append(changed, filepath.ToSlash(filepath.Join(".claude", "settings.local.json")))
	}
	if !guide {
		return changed, nil
	}

	guidePath := filepath.Join(root, "CLAUDE.md")
	old, err := os.ReadFile(guidePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return changed, err
	}
	heading, _, _ := strings.Cut(docs.AgentsSnippet, "\n")
	if !strings.Contains(string(old), heading) {
		text := string(old)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if text != "" {
			text += "\n"
		}
		if err := store.WriteAtomic(guidePath, strings.NewReader(text+docs.AgentsSnippet)); err != nil {
			return changed, err
		}
		changed = append(changed, "CLAUDE.md")
	}
	return changed, nil
}

// IsOurHook: a hook command that runs R3V's PreToolUse hook (whichever r3v
// it names).
func IsOurHook(c any) bool {
	s, _ := c.(string)
	return strings.HasSuffix(strings.TrimSpace(s), "hook pre-tool")
}
