package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nonlabhq/r3v/internal/actor"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
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
                                         only tells other agents (code, which never locks, is claimed)
  lock --take <paths…> --reason <why>    take over others' locks on paths whose holder is gone (kept
                                         with the reason)
  lock --break <path>                    take anyone's lock off a path (the person's call)
  unlock <paths…>                        free this workspace's locks
  locks                                  every agent lock of the project here (stale ones marked) and the
                                         files the person changed
`

// lockJSONOf is an agent lock as an error's holder.
func lockJSONOf(l agentlock.Lock) lockJSON {
	holder := "agent"
	if l.Person {
		holder = "person"
	}
	return lockJSON{Path: l.Path, Holder: holder, Workspace: l.Workspace, Branch: l.Branch, Actor: l.Actor}
}

// lockJSONIn is lockJSONOf with where its holder is on this computer.
func lockJSONIn(l agentlock.Lock, copies map[string]project.LockCopy) lockJSON {
	out := lockJSONOf(l)
	if c, ok := copies[l.Workspace]; ok {
		out.Folder, out.MainCopy = c.Folder, c.Main
	}
	return out
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
	// ClaimedInstead: files asked to be locked that never lock (code, which
	// merges hunk by hunk): claimed, as a lock on them would stop no one.
	ClaimedInstead []string `json:"claimed_instead"`
}

const lockUsage = "usage: r3v lock <paths…> [--claim] | r3v lock --take <paths…> --reason <why> | r3v lock --break <path>"

// wideFolder: a folder lock over this many files is said to be too wide
// (it stops other agents on all of them).
const wideFolder = 200

func cmdLock(args []string) error {
	kind, brk, take := agentlock.KindLock, false, false
	reason, hasReason := "", false
	var paths []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--claim" || a == "-claim":
			kind = agentlock.KindClaim
		case a == "--break" || a == "-break":
			brk = true
		case a == "--take" || a == "-take":
			take = true
		case (a == "--reason" || a == "-reason") && i+1 < len(args):
			i++
			reason, hasReason = args[i], true
		case strings.HasPrefix(a, "--reason="):
			reason, hasReason = strings.TrimPrefix(a, "--reason="), true
		default:
			if strings.HasPrefix(a, "-") {
				return usageError(lockUsage)
			}
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 || (brk && (len(paths) != 1 || kind == agentlock.KindClaim || take)) ||
		(take && kind == agentlock.KindClaim) || (hasReason && !take) {
		return usageError(lockUsage)
	}
	if take && strings.TrimSpace(reason) == "" {
		return usageError("r3v lock --take needs --reason <why> (kept with the lock, for whoever held it)")
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
	out := lockResultJSON{Warnings: []string{}, ClaimedInstead: []string{}}
	var locks, claims []string
	for _, p := range rels {
		switch {
		case kind == agentlock.KindClaim:
			claims = append(claims, p)
			continue
		case strings.HasSuffix(p, "/"):
			if n := countFiles(r.Abs(p), wideFolder); n >= wideFolder {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s holds %d files or more: a lock that wide stops other agents on all of them", p, n))
			}
		case !r.AgentLocked(p):
			// A lock on it would stop no one (neither the hook nor a commit
			// checks it): a claim is what tells the others.
			claims = append(claims, p)
			out.ClaimedInstead = append(out.ClaimedInstead, p)
			out.Warnings = append(out.Warnings, p+" isn't a file that locks (code merges hunk by hunk): claimed instead, which tells other agents and blocks no one")
			continue
		}
		locks = append(locks, p)
	}
	var res agentlock.Result
	if len(locks) > 0 {
		if take {
			res, err = r.TakeOverAgentLocks(locks, reason, "")
		} else {
			res, err = r.TakeAgentLocks(locks, agentlock.KindLock, "")
		}
		var refused *agentlock.Refused
		if errors.As(err, &refused) {
			return r.LockedByAgent(refused.Locks)
		}
		if err != nil {
			return err
		}
	}
	if len(claims) > 0 { // after the locks: a refused set leaves nothing
		cres, err := r.TakeAgentLocks(claims, agentlock.KindClaim, "")
		if err != nil {
			return err
		}
		res.Taken = append(res.Taken, cres.Taken...)
		res.Others = append(res.Others, cres.Others...)
	}
	out.Taken, out.Others, out.TakenOver = nonNil(res.Taken), nonNil(res.Others), nonNil(res.TakenOver)
	result("lock", out, func() {
		for _, w := range out.Warnings {
			fmt.Fprintln(os.Stderr, "warning: "+w)
		}
		for _, l := range out.TakenOver {
			what := "stale lock"
			if take && !l.Stale {
				what = "lock"
			}
			fmt.Printf("took over %s's %s on %s\n", l.Who(), what, l.Path)
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
			if l.Reason != "" {
				stale += "  (taken over from " + l.TakenFrom + ": " + l.Reason + ")"
			}
			fmt.Printf("%-5s  %s  %s%s\n", l.Kind, l.Path, who, stale)
		}
	})
	return nil
}

// --- the hook -------------------------------------------------------------

// hookTools are Claude Code's tools that change a file.
var hookTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

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

// cmdHook: r3v hook pre-tool, Claude Code's PreToolUse hook (and prompt,
// post-tool: steps). Always exits
// 0: a hook that fails (exit 2) would stop the agent's every change.
func cmdHook(args []string) error {
	if len(args) != 1 {
		return nil
	}
	switch args[0] {
	case "pre-tool":
		hookPreTool(os.Stdin, os.Stdout)
	case "prompt", "post-tool": // steps (steps.go)
		hookSteps(args[0], os.Stdin, os.Stdout)
	}
	return nil // a hook from a newer R3V: nothing to say
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
			return refusedReason(rel, refused.Locks, r.LockCopies()), ""
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

func refusedReason(rel string, holders []agentlock.Lock, copies map[string]project.LockCopy) string {
	var who []string
	for _, l := range holders {
		w := l.Who()
		if !l.Person {
			w = strings.TrimPrefix(project.HolderOf(l, copies), "locked by ")
		}
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
	changed, err := agentsetup.ClaudeCode(r.Root, hookCommand(), !r.IsWorkspace(), r.StepsOn()) // (the guide: in the main copy, committed once)
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

// hookCommand is how Claude Code runs this R3V's hook (see
// agentsetup.HookCommand).
func hookCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return agentsetup.HookCommand("")
	}
	return agentsetup.HookCommand(exe)
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
