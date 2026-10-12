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
// the agents' guide (Guide: the project's, committed, so only in the main
// copy). Running it again changes nothing. It returns the files it
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

	more, err := Guide(root)
	return append(changed, more...), err
}

// Guide puts the agents' guide in the project (the copy in root): in
// AGENTS.md, which agent tools read (Codex, Cursor and others), and a line
// importing it in CLAUDE.md (Claude Code reads that one). Both are the
// project's: committed, and so in every workspace. R3V's part of AGENTS.md
// is between two markers; an older one is replaced, the rest of the file
// left as it is. Running it again changes nothing. It returns the files it
// wrote.
func Guide(root string) ([]string, error) {
	var changed []string
	path := filepath.Join(root, "AGENTS.md")
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	text := string(old)
	block := guideBlock()
	if strings.Contains(text, "\r\n") { // the file's line ends
		block = strings.ReplaceAll(block, "\n", "\r\n")
	}
	if start, end, ok := guidePart(text); ok {
		text = text[:start] + block + text[end:]
	} else {
		text = appendPart(text, block)
	}
	if text != string(old) {
		if err := store.WriteAtomic(path, strings.NewReader(text)); err != nil {
			return nil, err
		}
		changed = append(changed, "AGENTS.md")
	}

	path = filepath.Join(root, "CLAUDE.md")
	old, err = os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return changed, err
	}
	if strings.Contains(string(old), "@AGENTS.md") || strings.Contains(string(old), guideHeading()) {
		return changed, nil
	}
	if err := store.WriteAtomic(path, strings.NewReader(appendPart(string(old), "@AGENTS.md\n"))); err != nil {
		return changed, err
	}
	return append(changed, "CLAUDE.md"), nil
}

// appendPart puts part at the end of text, after a blank line (in text's
// line ends).
func appendPart(text, part string) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += nl
	}
	if text != "" {
		text += nl
	}
	return text + part
}

// The markers around R3V's part of AGENTS.md (HTML comments: not shown).
const (
	guideBegin = "<!-- R3V: this part is R3V's (r3v agent setup updates it) -->"
	guideEnd   = "<!-- R3V: end -->"
)

func guideHeading() string {
	heading, _, _ := strings.Cut(docs.AgentsSnippet, "\n")
	return strings.TrimRight(heading, "\r")
}

// guideBlock is R3V's part of AGENTS.md as this R3V writes it.
func guideBlock() string {
	snippet := strings.ReplaceAll(docs.AgentsSnippet, "\r\n", "\n")
	return guideBegin + "\n" + strings.TrimRight(snippet, "\n") + "\n" + guideEnd + "\n"
}

// guidePart finds R3V's part of text: between the markers, or (written
// before there were markers) from its heading to the next heading of its
// level or above, or the end.
func guidePart(text string) (start, end int, ok bool) {
	if b := strings.Index(text, guideBegin); b >= 0 {
		if e := strings.Index(text[b:], guideEnd); e >= 0 {
			end = b + e + len(guideEnd)
			if strings.HasPrefix(text[end:], "\r\n") {
				end += 2
			} else if strings.HasPrefix(text[end:], "\n") {
				end++
			}
			return b, end, true
		}
	}
	heading := guideHeading()
	at := -1
	for i := 0; ; {
		if strings.HasPrefix(text[i:], heading) {
			at = i
			break
		}
		next := strings.IndexByte(text[i:], '\n')
		if next < 0 {
			break
		}
		i += next + 1
	}
	if at < 0 {
		return 0, 0, false
	}
	end = len(text)
	for i := at; ; {
		next := strings.IndexByte(text[i:], '\n')
		if next < 0 {
			break
		}
		i += next + 1
		if strings.HasPrefix(text[i:], "# ") || strings.HasPrefix(text[i:], "## ") {
			end = i
			break
		}
	}
	return at, end, true
}

// HasGuide: the project in root has the agents' guide (AGENTS.md says it).
func HasGuide(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	return err == nil && strings.Contains(string(data), guideHeading())
}

// GuideOld: the project's AGENTS.md has R3V's part as an older R3V wrote
// it; Guide puts this one's in its place.
func GuideOld(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return false
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	start, end, ok := guidePart(text)
	return ok && strings.TrimRight(text[start:end], "\n") != strings.TrimRight(guideBlock(), "\n")
}

// ToolOf is the agent tool the copy in root is set up for (its
// R3V_ACTOR in Claude Code's settings), "" when none.
func ToolOf(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.local.json"))
	if err != nil {
		return ""
	}
	var s struct {
		Env map[string]any `json:"env"`
	}
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	tool, _ := s.Env[actor.EnvTool].(string)
	if tool == actor.None {
		return ""
	}
	return tool
}

// IsOurHook: a hook command that runs R3V's PreToolUse hook (whichever r3v
// it names).
func IsOurHook(c any) bool {
	s, _ := c.(string)
	return strings.HasSuffix(strings.TrimSpace(s), "hook pre-tool")
}
