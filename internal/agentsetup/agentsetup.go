// Package agentsetup sets a project's copy up for an AI agent tool
// (docs/design/agent-workspaces.md, "Setting agents up", and
// docs/design/agent-identity.md): the copy's agent identity, the agents'
// guide in AGENTS.md (which most tools read), and for Claude Code its
// hooks and R3V_ACTOR. Used by the CLI (`r3v agent setup`, `r3v workspace
// add --agent`) and the app.
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

// Options say how Setup sets a copy up.
type Options struct {
	Tool  string // the agent tool's name (actor.ToolName)
	Model string // the model it runs; "": the one set before for this tool, if any
	Hook  string // Claude Code's hook command (HookCommand)
	Guide bool   // the agents' guide too (the main copy: committed once)
	Steps bool   // Claude Code's step hooks (ClaudeCode)
}

// Setup sets the copy in root (its .r3v: dir) up for the tool: its
// identity in dir (versions committed here say this agent made them),
// with Guide the agents' guide in AGENTS.md, and for Claude Code its
// settings (ClaudeCode, CLAUDE.md importing the guide). Running it again
// changes nothing. It returns the files it wrote (relative to root) and
// the identity kept.
func Setup(root, dir string, o Options) ([]string, actor.Identity, error) {
	id := actor.Identity{Tool: o.Tool, Model: o.Model}
	if old := actor.ReadIdentity(dir); id.Model == "" && old != nil && old.Tool == id.Tool {
		id.Model = old.Model
	}
	var changed []string
	var err error
	if o.Tool == ClaudeCodeTool {
		changed, err = ClaudeCode(root, o.Hook, o.Guide, o.Steps)
	} else if o.Guide {
		changed, err = GuideFor(root, false)
	}
	if err != nil {
		return changed, id, err
	}
	wrote, err := actor.WriteIdentity(dir, id)
	if wrote {
		rel, _ := filepath.Rel(root, filepath.Join(dir, actor.IdentityFile))
		changed = append(changed, filepath.ToSlash(rel))
	}
	return changed, id, err
}

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
// copy). With steps, the hooks that make steps too (a turn per prompt, a
// step after each tool call); without, they are taken out. Running it
// again changes nothing. It returns the files it wrote, relative to root.
func ClaudeCode(root, command string, guide, steps bool) ([]string, error) {
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
	setHook(hooks, "PreToolUse", HookMatcher, command)
	base := strings.TrimSuffix(command, "pre-tool")
	if steps {
		setHook(hooks, "PostToolUse", "*", base+"post-tool")
		setHook(hooks, "UserPromptSubmit", "", base+"prompt")
	} else {
		setHook(hooks, "PostToolUse", "", "")
		setHook(hooks, "UserPromptSubmit", "", "")
	}
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
// AGENTS.md, which agent tools read (Codex, opencode, Cursor, Gemini CLI
// and others), and a line importing it in CLAUDE.md (Claude Code reads
// that one). Both are the project's: committed, and so in every workspace.
// R3V's part of AGENTS.md is between two markers; an older one is
// replaced, the rest of the file left as it is. Running it again changes
// nothing. It returns the files it wrote.
func Guide(root string) ([]string, error) { return GuideFor(root, true) }

// GuideFor is Guide, with CLAUDE.md only when claudeMD (set up for
// another tool, a project has no reason to get one).
func GuideFor(root string, claudeMD bool) ([]string, error) {
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

	if !claudeMD {
		return changed, nil
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

// setHook puts our hook for event (command "": takes it out), beside the
// project's own hooks for it.
// StepHooks reports whether Claude Code in the copy in root makes steps:
// its settings have R3V's prompt and post-tool hooks (ClaudeCode with steps).
func StepHooks(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.local.json"))
	if err != nil {
		return false
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return false
	}
	has := func(event, suffix string) bool {
		for _, e := range settings.Hooks[event] {
			for _, h := range e.Hooks {
				if IsOurHook(h.Command) && strings.HasSuffix(h.Command, suffix) {
					return true
				}
			}
		}
		return false
	}
	return has("UserPromptSubmit", "hook prompt") && has("PostToolUse", "hook post-tool")
}

func setHook(hooks map[string]any, event, matcher, command string) {
	list, _ := hooks[event].([]any)
	ours := map[string]any{"type": "command", "command": command}
	found := false
	var kept []any
	for _, e := range list {
		entry, _ := e.(map[string]any)
		inner, _ := entry["hooks"].([]any)
		var keep []any
		for _, hk := range inner {
			if m, _ := hk.(map[string]any); m != nil && IsOurHook(m["command"]) {
				if command == "" || found {
					continue
				}
				hk, found = ours, true
				if matcher != "" {
					entry["matcher"] = matcher
				}
			}
			keep = append(keep, hk)
		}
		if len(keep) == 0 && len(inner) > 0 {
			continue // only ours was there
		}
		entry["hooks"] = keep
		kept = append(kept, entry)
	}
	if !found && command != "" {
		entry := map[string]any{"hooks": []any{ours}}
		if matcher != "" {
			entry["matcher"] = matcher
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 {
		delete(hooks, event)
		return
	}
	hooks[event] = kept
}

// IsOurHook: a hook command that runs one of R3V's hooks (whichever r3v
// it names).
func IsOurHook(c any) bool {
	s, _ := c.(string)
	s = strings.TrimSpace(s)
	return strings.HasSuffix(s, "hook pre-tool") || strings.HasSuffix(s, "hook post-tool") || strings.HasSuffix(s, "hook prompt")
}

// CopyTool is the agent tool the copy in root is set up for: Claude
// Code's settings (ToolOf), else the copy's identity; "" when none.
func CopyTool(root string) string {
	if tool := ToolOf(root); tool != "" {
		return tool
	}
	if id := actor.ReadIdentity(filepath.Join(root, ".r3v")); id != nil {
		return id.Tool
	}
	return ""
}
