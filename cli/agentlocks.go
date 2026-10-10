package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nonlabhq/r3v/internal/actor"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/docs"
	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/store"
)

// Agent locks between the workspaces of a project on this computer
// (docs/design/agent-workspaces.md, "Locks"): `lock`, `unlock`, `locks`,
// the Claude Code hook that takes them on first touch, and `agent setup`
// that installs it. On the Nightly channel while they're new; the hook is
// in every build, so a project set up by Nightly and opened by Stable
// isn't stopped (it allows everything there).

// agentCommands adds the agent lock commands (the Nightly channel's).
func agentCommands() {
	extraCommands["lock"] = cmdLock
	extraCommands["unlock"] = cmdUnlock
	extraCommands["locks"] = cmdLocks
	extraCommands["agent"] = cmdAgent
	for _, c := range []string{"lock", "unlock", "locks", "agent"} {
		jsonCommands[c] = true
	}
}

const agentUsage = `
AI agents on one computer (Nightly):
  lock <paths…> [--claim]                lock files (or folders) for this workspace, all or none; --claim
                                         only tells other agents (for code)
  lock --break <path>                    take anyone's lock off a path (the person's call)
  unlock <paths…>                        free this workspace's locks
  locks                                  every agent lock of the project here (stale ones marked) and the
                                         files the person changed
  agent setup [--tool claude-code]       set an agent tool up in this folder: its hook, R3V_ACTOR, the guide
`

// lockJSONOf is an agent lock as an error's holder.
func lockJSONOf(l agentlock.Lock) lockJSON {
	holder := "agent"
	if l.Person {
		holder = "person"
	}
	return lockJSON{Path: l.Path, Holder: holder, Workspace: l.Workspace, Branch: l.Branch, Actor: l.Actor}
}

// lockArgs turns paths on the command line into lock paths of the project.
func lockArgs(r *project.Repo, args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		rel := r.RelPath(a)
		if rel == "" {
			return nil, usageError("%s is not in the project", a)
		}
		folder := strings.HasSuffix(a, "/") || strings.HasSuffix(a, `\`)
		if fi, err := os.Stat(r.Abs(rel)); err == nil && fi.IsDir() {
			folder = true
		}
		out = append(out, agentlock.Clean(rel, folder))
	}
	return out, nil
}

type lockResultJSON struct {
	Taken     []agentlock.Lock `json:"taken"`
	Others    []agentlock.Lock `json:"others"`
	TakenOver []agentlock.Lock `json:"taken_over"`
	Warnings  []string         `json:"warnings"`
}

// wideFolder: a folder lock over this many files is said to be too wide
// (it stops other agents on all of them).
const wideFolder = 200

func cmdLock(args []string) error {
	kind, brk := agentlock.KindLock, false
	var paths []string
	for _, a := range args {
		switch a {
		case "--claim", "-claim":
			kind = agentlock.KindClaim
		case "--break", "-break":
			brk = true
		default:
			if strings.HasPrefix(a, "-") {
				return usageError("usage: r3v lock <paths…> [--claim] | r3v lock --break <path>")
			}
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 || (brk && (len(paths) != 1 || kind == agentlock.KindClaim)) {
		return usageError("usage: r3v lock <paths…> [--claim] | r3v lock --break <path>")
	}
	r, err := openAgent(".")
	if err != nil {
		return err
	}
	rels, err := lockArgs(r, paths)
	if err != nil {
		return err
	}
	if brk {
		broken, err := r.BreakAgentLock(rels[0])
		if err != nil {
			return err
		}
		result("lock", map[string]any{"broken": nonNil(broken)}, func() {
			if len(broken) == 0 {
				fmt.Println("no lock on " + rels[0])
			}
			for _, l := range broken {
				fmt.Printf("broke %s's %s on %s\n", l.Who(), l.Kind, l.Path)
			}
		})
		return nil
	}
	out := lockResultJSON{Warnings: []string{}}
	for _, p := range rels {
		switch {
		case strings.HasSuffix(p, "/"):
			if n := countFiles(r.Abs(p), wideFolder); n >= wideFolder {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s holds %d files or more: a lock that wide stops other agents on all of them", p, n))
			}
		case kind == agentlock.KindLock && !r.AgentLocked(p):
			out.Warnings = append(out.Warnings, p+" isn't a file that locks (code merges hunk by hunk): a lock on it stops no one; --claim says you're on it")
		}
	}
	res, err := r.TakeAgentLocks(rels, kind, "")
	var refused *agentlock.Refused
	if errors.As(err, &refused) {
		return &project.ErrLockedByAgent{Locks: refused.Locks}
	}
	if err != nil {
		return err
	}
	out.Taken, out.Others, out.TakenOver = nonNil(res.Taken), nonNil(res.Others), nonNil(res.TakenOver)
	result("lock", out, func() {
		for _, w := range out.Warnings {
			fmt.Fprintln(os.Stderr, "warning: "+w)
		}
		for _, l := range out.TakenOver {
			fmt.Printf("took over %s's stale lock on %s\n", l.Who(), l.Path)
		}
		for _, l := range out.Taken {
			fmt.Printf("%s %s\n", map[string]string{agentlock.KindLock: "locked", agentlock.KindClaim: "claimed"}[l.Kind], l.Path)
		}
		for _, l := range out.Others {
			fmt.Printf("note: %s has a %s on %s\n", l.Who(), l.Kind, l.Path)
		}
	})
	return nil
}

// countFiles counts the files in dir, up to max.
func countFiles(dir string, max int) int {
	n := 0
	errEnough := errors.New("enough")
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if n++; n >= max {
				return errEnough
			}
		}
		return nil
	})
	return n
}

func cmdUnlock(args []string) error {
	if len(args) == 0 {
		return usageError("usage: r3v unlock <paths…>")
	}
	r, err := openAgent(".")
	if err != nil {
		return err
	}
	rels, err := lockArgs(r, args)
	if err != nil {
		return err
	}
	freed, err := r.UnlockAgent(rels)
	if err != nil {
		return err
	}
	result("unlock", map[string]any{"unlocked": nonNil(freed)}, func() {
		if len(freed) == 0 {
			fmt.Println("this workspace holds none of them")
		}
		for _, l := range freed {
			fmt.Printf("unlocked %s\n", l.Path)
		}
	})
	return nil
}

func cmdLocks(args []string) error {
	if len(args) > 0 {
		return usageError("usage: r3v locks [--json]")
	}
	r, err := openAgent(".")
	if err != nil {
		return err
	}
	r.RefreshAgentTeamLocks()
	locks, err := r.AllAgentLocks()
	if err != nil {
		return err
	}
	me := r.AgentHolder("").Workspace
	type lockOut struct {
		agentlock.Lock
		Mine bool `json:"mine"`
	}
	out := []lockOut{}
	for _, l := range locks {
		out = append(out, lockOut{l, l.Workspace == me && !l.Person})
	}
	result("locks", map[string]any{"workspace": me, "branch": r.BranchName(), "locks": out}, func() {
		if len(out) == 0 {
			fmt.Println("no agent locks")
		}
		for _, l := range out {
			who := l.Who()
			if l.Mine {
				who = "this workspace"
			}
			stale := ""
			if l.Stale {
				stale = "  (stale: no heartbeat since " + l.Heartbeat.Local().Format("2006-01-02 15:04") + ")"
			}
			fmt.Printf("%-5s  %s  %s%s\n", l.Kind, l.Path, who, stale)
		}
	})
	return nil
}

// --- the hook -------------------------------------------------------------

// hookTools are Claude Code's tools that change a file.
var hookTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

const hookMatcher = "Edit|Write|MultiEdit|NotebookEdit"

// hookInput is what Claude Code sends a PreToolUse hook (only what's used).
type hookInput struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath     string          `json:"file_path"`
		NotebookPath string          `json:"notebook_path"`
		Edits        json.RawMessage `json:"edits"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookDecision `json:"hookSpecificOutput"`
}

type hookDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

// cmdHook: r3v hook pre-tool, Claude Code's PreToolUse hook. Always exits
// 0: a hook that fails (exit 2) would stop the agent's every change.
func cmdHook(args []string) error {
	if len(args) != 1 || args[0] != "pre-tool" {
		return nil // a hook from a newer R3V: nothing to say
	}
	hookPreTool(os.Stdin, os.Stdout)
	return nil
}

// hookPreTool reads a tool call and answers: a deny (with who holds the
// file) when another workspace holds it, a note when another claims it,
// else nothing (Claude Code goes on as it would). Anything unexpected
// says nothing: the hook never stands in the way by mistake.
func hookPreTool(in io.Reader, out io.Writer) {
	if !project.AgentLocks {
		return
	}
	var h hookInput
	if json.NewDecoder(io.LimitReader(in, 16<<20)).Decode(&h) != nil || !hookTools[h.ToolName] {
		return
	}
	paths := []string{h.ToolInput.FilePath, h.ToolInput.NotebookPath}
	var edits []struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(h.ToolInput.Edits, &edits) == nil {
		for _, e := range edits {
			paths = append(paths, e.FilePath)
		}
	}
	var denied, notes []string
	repos := map[string]*project.Repo{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			if h.Cwd == "" {
				continue
			}
			p = filepath.Join(h.Cwd, p)
		}
		r := repos[filepath.Dir(p)]
		if r == nil {
			var err error
			if r, err = openAgent(filepath.Dir(p)); err != nil {
				continue // not a project
			}
			repos[filepath.Dir(p)] = r
		}
		rel := r.RelPath(p)
		if rel == "" {
			continue
		}
		d, n := hookPath(r, rel, h.SessionID)
		if d != "" {
			denied = append(denied, d)
		}
		if n != "" {
			notes = append(notes, n)
		}
	}
	dec := hookDecision{HookEventName: "PreToolUse"}
	switch {
	case len(denied) > 0:
		dec.PermissionDecision = "deny"
		dec.PermissionDecisionReason = strings.Join(denied, " ")
	case len(notes) > 0:
		dec.AdditionalContext = strings.Join(notes, " ")
	default:
		return
	}
	data, _ := json.Marshal(hookOutput{dec})
	out.Write(append(data, '\n'))
}

// hookPath takes rel for this workspace when it is a file that locks: the
// reason it's refused, or a note about others' claims ("" for none).
func hookPath(r *project.Repo, rel, session string) (deny, note string) {
	me := r.AgentHolder(session)
	list := r.AgentLockList()
	list.Heartbeat(me.Workspace)
	var others []agentlock.Lock
	if r.AgentLocked(rel) {
		res, err := r.TakeAgentLocks([]string{rel}, agentlock.KindLock, session)
		var refused *agentlock.Refused
		if errors.As(err, &refused) {
			return refusedReason(rel, refused.Locks), ""
		}
		if err != nil {
			return "", ""
		}
		others = res.Others
	} else {
		all, _ := list.All()
		for _, l := range agentlock.Over(all, rel) {
			if l.Workspace != me.Workspace && !l.Stale {
				others = append(others, l)
			}
		}
	}
	var keys []string
	byKey := map[string]agentlock.Lock{}
	for _, l := range others {
		k := l.Kind + "\x00" + l.Path + "\x00" + l.Workspace + "\x00" + l.Since.String()
		keys = append(keys, k)
		byKey[k] = l
	}
	var said []string
	for _, k := range list.Untold(me.Workspace, keys) {
		l := byKey[k]
		said = append(said, fmt.Sprintf("R3V: %s is working on %s (a %s): it can be merged, but check you're not doing the same work.", l.Who(), l.Path, l.Kind))
	}
	return "", strings.Join(said, " ")
}

func refusedReason(rel string, holders []agentlock.Lock) string {
	var who []string
	for _, l := range holders {
		w := l.Who()
		if !l.Person && l.Path != rel {
			w += " (" + l.Path + ")"
		}
		who = append(who, w)
	}
	return fmt.Sprintf("R3V: %s is locked by %s, and it can't be merged. Leave it and go on with other work; "+
		"if it must change now, ask the user (who can break the lock).", rel, strings.Join(who, ", "))
}

// --- agent setup ----------------------------------------------------------

func cmdAgent(args []string) error {
	if len(args) == 0 || args[0] != "setup" {
		return usageError("usage: r3v agent setup [--tool claude-code]")
	}
	tool := "claude-code"
	rest := args[1:]
	for len(rest) > 0 {
		switch {
		case (rest[0] == "--tool" || rest[0] == "-tool") && len(rest) > 1:
			tool, rest = rest[1], rest[2:]
		case strings.HasPrefix(rest[0], "--tool="):
			tool, rest = strings.TrimPrefix(rest[0], "--tool="), rest[1:]
		default:
			return usageError("usage: r3v agent setup [--tool claude-code]")
		}
	}
	if tool != "claude-code" {
		return usageError("r3v agent setup knows claude-code only (for now)")
	}
	r, err := openAgent(".")
	if err != nil {
		return err
	}
	r.RefreshAgentTeamLocks()
	changed, err := setupClaudeCode(r.Root, hookCommand(), !r.IsWorkspace()) // (the guide: in the main copy, committed once)
	if err != nil {
		return err
	}
	result("agent", map[string]any{"tool": tool, "folder": r.Root, "changed": nonNil(changed)}, func() {
		if len(changed) == 0 {
			fmt.Println("already set up")
		}
		for _, c := range changed {
			fmt.Println("wrote " + c)
		}
	})
	return nil
}

// hookCommand is how Claude Code runs this R3V's hook: `r3v` when that is
// this program, else its full path (another r3v on PATH may be a build
// without agent locks, or none).
func hookCommand() string {
	exe, err := os.Executable()
	if err != nil {
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

// setupClaudeCode writes Claude Code's settings for this copy in root
// (.claude/settings.local.json, which R3V never tracks: the hook's path and
// R3V_ACTOR are this computer's; merged with what's there) and, with guide,
// the agents' guide in CLAUDE.md (the project's: committed). Running it
// again changes nothing.
func setupClaudeCode(root, command string, guide bool) ([]string, error) {
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
	env[actor.EnvTool] = "claude-code"
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
			if m, _ := hk.(map[string]any); m != nil && isOurHook(m["command"]) {
				list[i], found = ours, true
				entry["matcher"] = hookMatcher
			}
		}
	}
	if !found {
		pre = append(pre, map[string]any{"matcher": hookMatcher, "hooks": []any{ours}})
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

// isOurHook: a hook command that runs R3V's PreToolUse hook (whichever
// r3v it names).
func isOurHook(c any) bool {
	s, _ := c.(string)
	return strings.HasSuffix(strings.TrimSpace(s), "hook pre-tool")
}

// openAgent opens the project in dir without taking its lock (the hook runs
// on every tool call), as whoever runs it: an agent by its actor.
func openAgent(dir string) (*project.Repo, error) {
	r, err := project.Open(dir)
	if err != nil {
		return nil, err
	}
	r.Actor = actor.Detect()
	return r, nil
}
