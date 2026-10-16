package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
)

// A switch that stopped halfway (R3V closed or killed while goto, switch,
// update, merge or undo-merge rewrote the files) leaves them partly one
// version, partly another. The app offers to put them back; an agent with
// only the command line would go on with them as they are. So a command
// that has the project's lock puts them back first, as the app's button
// does, unless that could lose something: a tool has the project open (as
// the app, it waits until it is closed), or a file changed since it stopped
// (only a person decides about that). A command reading without the lock
// writes nothing, so it leaves them too.

// recoveredJSON is what a command did first to a switch an earlier one
// left halfway: Action "restored" (the files back as Version, the version
// the project is on; Changes: with the uncommitted changes they had) or
// "completed" (a project with no version yet: it got Version, as it was
// getting).
type recoveredJSON struct {
	Action   string   `json:"action"`
	Version  string   `json:"version"`
	Changes  bool     `json:"changes,omitempty"`
	Relinked []string `json:"relinked"`
}

// unfinishedJSON is a switch left halfway that this command didn't put
// back, and why (Code): "set_open_in_live" or "files_open_in_tool" (close
// it, then any command puts them back), "changed_since_switch" (Paths:
// files changed since; ask the user), or the error's code. Version is the
// version the files go back to.
type unfinishedJSON struct {
	Version string   `json:"version"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Hint    string   `json:"hint,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

// recovered and unfinished: what this command found of a switch left
// halfway (one of them, or neither), added to its JSON result.
var (
	recovered  *recoveredJSON
	unfinished *unfinishedJSON
)

// recoverSwitch puts back the files of a switch that stopped halfway, when
// that loses nothing (r is held: openRepo, openRead with the lock).
func recoverSwitch(r *project.Repo) {
	id := r.UnfinishedSwitch()
	if id == "" {
		return
	}
	work := strings.HasPrefix(id, "work ")
	out := &recoveredJSON{Action: "restored", Version: r.Head(), Changes: work}
	if out.Version == "" && !work {
		out.Action, out.Version = "completed", id
	}
	held := func(code, msg, hint string, paths []string) {
		unfinished = &unfinishedJSON{Version: out.Version, Code: code, Message: msg, Hint: hint, Paths: paths}
		if !jsonMode {
			fmt.Fprintf(os.Stderr, "warning: %s\n  %s\n", msg, hint)
		}
	}
	stopped := "a switch stopped half-way (R3V was closed): the project's files are partly another version"
	if check, what := openTool(r); check != "" {
		code, tool := "files_open_in_tool", toolNames[check]
		if check == "ableton-live" {
			code = "set_open_in_live"
		}
		if tool == "" {
			tool = check
		}
		open := tool + " has the project open"
		if what != "" && what != "?" {
			open = fmt.Sprintf("%s has %q open", tool, what)
		}
		held(code, stopped+"; "+open, "save and close it there: the next r3v command puts the files back", nil)
		return
	}
	changed, err := r.ChangedSinceSwitch()
	if err == nil && len(changed) > 0 {
		held("changed_since_switch", fmt.Sprintf("%s, and %d file(s) changed since: %s", stopped, len(changed), strings.Join(changed, ", ")),
			fmt.Sprintf("ask the user what to keep of them; then the R3V app puts the files back (or r3v goto --force %s, replacing them)",
				short(out.Version)), changed)
		return
	}
	var notes []string
	if err == nil {
		notes, err = r.RecoverSwitch()
	}
	if err != nil {
		ce := classify(err)
		held(ce.Code, stopped+": "+ce.Error(), "run the command again; if it keeps failing, open the project in the R3V app", nil)
		return
	}
	out.Relinked = nonNil(notes)
	recovered = out
	if jsonMode {
		return
	}
	switch {
	case out.Action == "completed":
		fmt.Fprintf(os.Stderr, "note: getting version %s stopped half-way (R3V was closed): finished now\n", short(out.Version))
	case work:
		fmt.Fprintf(os.Stderr, "note: a switch stopped half-way (R3V was closed): the files are back as they were (version %s and your changes)\n", short(out.Version))
	default:
		fmt.Fprintf(os.Stderr, "note: a switch stopped half-way (R3V was closed): the files are back as version %s\n", short(out.Version))
	}
}
