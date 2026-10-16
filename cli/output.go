package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/blob"
	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Output for programs and AI agents: with --json, a command that supports
// it prints one JSON object on stdout and nothing else (warnings go to
// stderr):
//
//	{"schema": 1, "ok": true, "command": "status", "result": {...}}
//	{"schema": 1, "ok": false, "command": "commit", "error": {"code": "merge_conflict", ...}}
//
// Errors have a fixed code (see the table in docs/agents.md) and an exit
// status by kind: 2 usage, 3 merge conflict, 4 a set is open in Live, 5 try
// again later, 6 needs another R3V, 1 anything else. Commands never wait
// for an answer: where one is needed they fail with a code.

const jsonSchema = 1

// jsonMode: --json was given.
var jsonMode bool

// jsonCommands support --json.
var jsonCommands = map[string]bool{"status": true, "log": true, "commit": true, "save": true, "snapshot": true, "update": true, "merge": true,
	"version": true, "backup": true, "switch": true, "goto": true, "checkout": true, "parked": true,
	"discard": true, "rules": true, "profile": true} // (rules: only rules ignore)

// stripJSON takes --json (or -json) out of args.
func stripJSON(args []string) ([]string, bool) {
	var out []string
	found := false
	for _, a := range args {
		if a == "--json" || a == "-json" {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}

type envelope struct {
	Schema  int        `json:"schema"`
	OK      bool       `json:"ok"`
	Command string     `json:"command"`
	Result  any        `json:"result,omitempty"`
	Error   *errorJSON `json:"error,omitempty"`
}

func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	os.Stdout.Write(append(data, '\n'))
}

// result prints a command's result: as JSON with --json, else with text.
func result(command string, v any, text func()) {
	if jsonMode {
		v = withFields(v, readBusy)
		v = withFields(v, openedJSON{Recovered: recovered, Unfinished: unfinished})
		printJSON(envelope{Schema: jsonSchema, OK: true, Command: command, Result: v})
		return
	}
	text()
}

// openedJSON is what opening the project found, in any command's result:
// a switch left halfway put back first (recovered), or not (unfinished).
type openedJSON struct {
	Recovered  *recoveredJSON  `json:"recovered,omitempty"`
	Unfinished *unfinishedJSON `json:"unfinished,omitempty"`
}

// withFields adds extra's fields to a result object, after its own: "busy"
// and "holder" (readBusy) to one read without the project's lock, say.
func withFields(v, extra any) any {
	more, err := json.Marshal(extra)
	if err != nil || len(more) <= 2 || more[0] != '{' { // (nil, or nothing to add)
		return v
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) < 2 || data[0] != '{' {
		return v
	}
	out := append([]byte{}, data[:len(data)-1]...)
	if len(data) > 2 { // (not {})
		out = append(out, ',')
	}
	return json.RawMessage(append(out, more[1:]...))
}

// Exit statuses.
const (
	exitError    = 1
	exitUsage    = 2
	exitConflict = 3
	exitLiveOpen = 4
	exitLater    = 5
	exitUpgrade  = 6
)

// cliError is an error with its code (for --json and the exit status).
type cliError struct {
	Code      string
	Exit      int
	Err       error
	Hint      string
	Conflicts []conflictJSON
	Set       string // the set open in Live
	Locks     []lockJSON
	Holder    *holderJSON // project_busy: who holds the project
	// Suggestion: the branch meant, when one asked for isn't there.
	Suggestion string
}

// holderJSON is who holds a project's lock, as it recorded itself (Running
// false: that process has ended, and whoever holds it now didn't say).
type holderJSON struct {
	Program string `json:"program"` // "app" or "cli"
	Command string `json:"command,omitempty"`
	PID     int    `json:"pid"`
	Since   string `json:"since"`
	Running bool   `json:"running"`
}

func holderOf(h *project.Holder) *holderJSON {
	if h == nil {
		return nil
	}
	return &holderJSON{Program: h.Program, Command: h.Command, PID: h.PID, Since: h.Since.UTC().Format(time.RFC3339),
		Running: h.Running}
}

// lockJSON is a path someone else holds: a member of the team (a share
// refused: files_locked), or another workspace on this computer
// (files_locked_by_agent: holder "agent" or "person", its workspace,
// branch and agent tool).
type lockJSON struct {
	Path      string `json:"path"`
	MemberID  string `json:"member_id,omitempty"`
	Holder    string `json:"holder,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Actor     string `json:"actor,omitempty"`
}

func (e *cliError) Error() string { return e.Err.Error() }
func (e *cliError) Unwrap() error { return e.Err }

type errorJSON struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Hint      string         `json:"hint,omitempty"`
	Exit      int            `json:"exit"`
	Conflicts []conflictJSON `json:"conflicts,omitempty"`
	Set       string         `json:"set,omitempty"`
	Locks     []lockJSON     `json:"locks,omitempty"`
	Holder    *holderJSON    `json:"holder,omitempty"`
	// Suggestion: the closest branch to one asked for that isn't there.
	Suggestion string `json:"suggestion,omitempty"`
	// Recovered, Unfinished: as in a result (openedJSON), the command
	// having failed after.
	Recovered  *recoveredJSON  `json:"recovered,omitempty"`
	Unfinished *unfinishedJSON `json:"unfinished,omitempty"`
}

func usageError(format string, a ...any) error {
	return &cliError{Code: "usage", Exit: exitUsage, Err: fmt.Errorf(format, a...)}
}

// codes for errors from the project, by sentinel.
var codes = []struct {
	err  error
	code string
	exit int
	hint string
}{
	{project.ErrNotRepo, "not_a_project", exitUpgrade, "run it in a project folder R3V tracks (r3v init, or r3v clone)"},
	{project.ErrBusy, "project_busy", exitLater, "the app or another command is working on the project: try again in a moment"},
	{manifest.ErrNewerFormat, "newer_version_needed", exitUpgrade, "install the current R3V"},
	{blob.ErrNewerFormat, "newer_version_needed", exitUpgrade, "install the current R3V"},
	{project.ErrNoRemote, "not_connected", exitError, "connect the project to a team: r3v remote <connection-code>"},
	{project.ErrDirty, "unsaved_changes", exitError, "commit them first: r3v commit -m \"...\" (it also brings in the team's versions)"},
	{project.ErrOlderVersion, "on_older_version", exitError, "go back to the latest version: r3v goto latest"},
	{project.ErrUnshared, "unshared_versions", exitError, "share them first: r3v commit -m ..."},
	{project.ErrNotAMerge, "not_a_merge", exitError, "pick a merge version: r3v log --first-parent lists them"},
	{project.ErrNothingToUndo, "nothing_to_undo", exitError, "nothing to do: the branch already is without those changes"},
	{project.ErrParkedHere, "changes_parked_here", exitError, "bring them back or discard them first: r3v parked"},
	{project.ErrNotHere, "files_not_here", exitError, "join the team again to get the files"},
	{project.ErrNotDownloaded, "not_downloaded", exitError, "finish the download first: r3v update (or r3v clone into the same folder)"},
	{project.ErrWorkspaceNotDone, "workspace_not_merged", exitError,
		"merge its branch (r3v merge <branch> in the main copy), commit or discard its changes; or --force to lose them"},
	{project.ErrInWorkspace, "in_workspace", exitError, "run it in the project's main copy"},
	{project.ErrWorkspaceOrphaned, "workspace_orphaned", exitError, "the project's main copy was moved or deleted: make the workspace again"},
	{project.ErrWorkspacesNotInBuild, "needs_nightly", exitUpgrade, "the user must switch R3V to the Nightly channel (Settings → Updates)"},
	{remote.ErrStorageFull, "cloud_quota", exitError, "the team's storage on R3V Cloud is full: delete projects or versions to make room (reading still works); the version stays here, shared once there is room"},
	{cloud.ErrBetaOnly, "cloud_beta_only", exitError, "R3V Cloud is in a closed beta: ask for an invitation, or join the waitlist at " + cloud.Waitlist},
	{cloud.ErrOneTeam, "cloud_one_team", exitError, "the beta includes one team of your own: use it, or delete it to make another"},
}

// classify gives err its code.
func classify(err error) *cliError {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce
	}
	var nn *profile.NeedsNightly
	if errors.As(err, &nn) {
		return &cliError{Code: "needs_nightly", Exit: exitUpgrade, Err: err,
			Hint: "the user must switch R3V to the Nightly channel (Settings → Updates)"}
	}
	var tf *remote.ErrTeamFeatures
	if errors.As(err, &tf) {
		return &cliError{Code: "team_needs_features", Exit: exitUpgrade, Err: err,
			Hint: "the user must update R3V, or switch to the Nightly channel"}
	}
	var locked *remote.ErrLocked
	if errors.As(err, &locked) {
		ce := &cliError{Code: "files_locked", Exit: exitError, Err: err,
			Hint: "someone else holds these files: the versions stay here; save again once they're unlocked"}
		for _, l := range locked.Locks {
			ce.Locks = append(ce.Locks, lockJSON{Path: l.Path, MemberID: l.MemberID})
		}
		return ce
	}
	var byAgent *project.ErrLockedByAgent
	if errors.As(err, &byAgent) {
		ce := &cliError{Code: "files_locked_by_agent", Exit: exitError, Err: err,
			Hint: "another workspace here holds these files: leave them, or ask the user (who can break the lock)"}
		for _, l := range byAgent.Locks {
			ce.Locks = append(ce.Locks, lockJSONOf(l))
		}
		return ce
	}
	var busy *project.BusyError
	if errors.As(err, &busy) {
		return &cliError{Code: "project_busy", Exit: exitLater, Err: err, Holder: holderOf(busy.Holder),
			Hint: "the app or another command is working on the project: try again in a moment (or wait longer: --wait SECONDS)"}
	}
	var noBranch *project.NoBranchError
	if errors.As(err, &noBranch) {
		return &cliError{Code: "error", Exit: exitError, Err: err, Suggestion: noBranch.Suggestion}
	}
	if errors.Is(err, remote.ErrUpdateR3V) {
		return &cliError{Code: "newer_version_needed", Exit: exitUpgrade, Err: err, Hint: "install the current R3V"}
	}
	var mc *project.MergeConflictError
	if errors.As(err, &mc) {
		return &cliError{Code: "merge_conflict", Exit: exitConflict, Err: err, Conflicts: conflictsJSON(mc.Conflicts),
			Hint: "choose how to resolve: --strategy ours | theirs | both, or each by its key: --resolve <key>=<choice>"}
	}
	for _, c := range codes {
		if errors.Is(err, c.err) {
			return &cliError{Code: c.code, Exit: c.exit, Err: err, Hint: c.hint}
		}
	}
	if remote.Transient(err) {
		return &cliError{Code: "team_unreachable", Exit: exitLater, Err: err,
			Hint: "the team's storage can't be reached: try again later"}
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "usage:") || errors.Is(err, flag.ErrHelp) {
		return &cliError{Code: "usage", Exit: exitUsage, Err: err}
	}
	for _, p := range []string{"flag provided but not defined", "flag needs an argument", "invalid value",
		"invalid boolean", "a message is required"} {
		if strings.Contains(msg, p) {
			return &cliError{Code: "usage", Exit: exitUsage, Err: err}
		}
	}
	return &cliError{Code: "error", Exit: exitError, Err: err}
}

// fail reports err (text on stderr, or JSON on stdout) and returns the exit
// status.
func fail(command string, err error) int {
	ce := classify(err)
	if jsonMode {
		printJSON(envelope{Schema: jsonSchema, OK: false, Command: command, Error: &errorJSON{Code: ce.Code,
			Message: ce.Error(), Hint: ce.Hint, Exit: ce.Exit, Conflicts: ce.Conflicts, Set: ce.Set, Locks: ce.Locks,
			Holder: ce.Holder, Suggestion: ce.Suggestion, Recovered: recovered, Unfinished: unfinished}})
		return ce.Exit
	}
	fmt.Fprintln(os.Stderr, "error:", ce.Error())
	if ce.Code == "merge_conflict" { // the conflicts are in the message; how to go on is not
		fmt.Fprintln(os.Stderr, ce.Hint)
	}
	return ce.Exit
}

// --- results ---

type versionJSON struct {
	ID       string   `json:"id"`
	Time     string   `json:"time"`
	Author   string   `json:"author"`
	AuthorID string   `json:"author_id,omitempty"`
	Message  string   `json:"message"`
	Parents  []string `json:"parents"`
	Branches []string `json:"branches,omitempty"` // team branches at this version
	MadeOn   string   `json:"made_on,omitempty"`  // the branch it was made on (its key; older versions: none)
	// Actor: the agent that made it for the author (none: the author did).
	Actor *project.Actor `json:"actor,omitempty"`
	// WIP: its title says it isn't finished (merge and update previews).
	WIP bool `json:"wip,omitempty"`
	// MergedCount: how many versions a merge brought in (log --first-parent).
	MergedCount int `json:"merged_count,omitempty"`
}

func versionOf(m *project.Manifest, names map[string]string) versionJSON {
	parents := m.Parents
	if parents == nil {
		parents = []string{}
	}
	return versionJSON{ID: m.ID, Time: m.Time, Author: project.AuthorName(m, names), AuthorID: m.AuthorID,
		Message: m.Message, Parents: parents, MadeOn: m.Branch, Actor: m.Actor}
}

// byLine is who made m, for a line of text: the author, and the agent
// that worked for them ("Mia (claude-code)").
func byLine(author string, m *project.Manifest) string {
	if a := m.Actor; a != nil {
		tool := a.Tool
		if tool == "" {
			tool = "agent"
		}
		return author + " (" + tool + ")"
	}
	return author
}

type changeJSON struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added, modified, deleted, renamed, untracked
	From   string `json:"from,omitempty"`
	Edited bool   `json:"edited,omitempty"` // renamed and changed
	// SetChanges: what changed in a Live Set (tracks, devices, clips…), one
	// line each, as `r3v diff` prints it.
	SetChanges []string `json:"set_changes,omitempty"`
	// Weight: for a Live Set, its heaviest change: noise (a plugin saving its
	// own state), tidy, mix, sound or arrangement.
	Weight string `json:"weight,omitempty"`
}

type conflictJSON struct {
	Key         string `json:"key"`
	File        string `json:"file"`
	Unit        string `json:"unit"` // e.g. AudioTrack "Bass", or the file itself
	Description string `json:"description"`
	CanKeepBoth bool   `json:"can_keep_both"`
	// A text file's hunk (key text:<path>#<n>): n, and its lines on each
	// side ({start, count, text}; start from 1).
	Hunk   int               `json:"hunk,omitempty"`
	Base   *project.HunkSide `json:"base,omitempty"`
	Ours   *project.HunkSide `json:"ours,omitempty"`
	Theirs *project.HunkSide `json:"theirs,omitempty"`
	// Choices: the words --resolve takes for it, where the command has its
	// own (undo-merge: keep, undo, both); else ours, theirs, both.
	Choices []string `json:"choices,omitempty"`
}

func conflictsJSON(cs []project.ConflictItem) []conflictJSON {
	out := []conflictJSON{}
	for _, c := range cs {
		out = append(out, conflictJSON{Key: c.Key, File: c.File, Unit: c.Unit, Description: c.Description,
			CanKeepBoth: c.CanKeepBoth, Hunk: c.Hunk, Base: c.Base, Ours: c.Ours, Theirs: c.Theirs})
	}
	return out
}

func setLines(render string) []string {
	var out []string
	for _, l := range strings.Split(render, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// nonNil keeps empty lists as [] in JSON.
func nonNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}
