package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
)

// workspaceJSON is a workspace of the project (docs/design/agent-workspaces.md).
type workspaceJSON struct {
	Folder     string `json:"folder"`
	Branch     string `json:"branch"`      // its branch's key
	BranchName string `json:"branch_name"` // what the branch is called
	Base       string `json:"base"`        // the branch it was made from, and merges into
	Created    string `json:"created"`     // RFC 3339
	// With list: where it is. Ahead and behind (of its base branch) are
	// null when the team can't be reached.
	Head         string `json:"head,omitempty"`
	Ahead        *int   `json:"ahead,omitempty"`
	Behind       *int   `json:"behind,omitempty"`
	Changes      int    `json:"changes"` // files changed, not committed
	Parked       int    `json:"parked"`  // sets of parked changes
	LastActivity string `json:"last_activity,omitempty"`
	Missing      bool   `json:"missing,omitempty"` // the folder is gone
}

func workspaceOf(e project.WorkspaceEntry) workspaceJSON {
	return workspaceJSON{Folder: e.Folder, Branch: e.Branch, BranchName: e.Branch, Base: e.Base,
		Created: e.Created.Format(time.RFC3339)}
}

func cmdWorkspace(args []string) error {
	usage := errors.New("usage: r3v workspace add <folder> --branch NAME [--from BRANCH|VERSION] [--agent claude-code] | list | remove <folder> [--force]")
	if len(args) == 0 {
		return usage
	}
	fs := flag.NewFlagSet("workspace", flag.ContinueOnError)
	branch := fs.String("branch", "", "the name of the workspace's branch (made for it)")
	from := fs.String("from", "", "a branch or version to start from (default: where this copy is)")
	agent := fs.String("agent", "", "set it up for an agent tool (claude-code): its hook and R3V_ACTOR")
	force := fs.Bool("force", false, "remove even with work not merged or not committed (it is lost)")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return err
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if len(pos) != 1 || *branch == "" {
			return usage
		}
		if *agent != "" && *agent != "claude-code" {
			return usageError("agent tools set up for now: claude-code")
		}
		e, err := r.AddWorkspace(pos[0], project.WorkspaceOptions{Branch: *branch, From: *from})
		if err != nil {
			return err
		}
		if *agent != "" { // (the workspace is made: set up, or said why not)
			if _, err := agentsetup.ClaudeCode(e.Folder, hookCommand(), false, project.Steps); err != nil {
				return fmt.Errorf("the workspace is made, but setting %s up failed: %w (run r3v agent setup there)", *agent, err)
			}
		}
		out := workspaceOf(*e)
		out.BranchName = r.BranchLabel(e.Branch)
		result("workspace", map[string]any{"workspace": out}, func() {
			fmt.Printf("workspace %s on branch %s (from %s)\n", e.Folder, out.BranchName, r.BranchLabel(e.Base))
			fmt.Println("  what the rules leave out (node_modules/, target/) isn't there: build it there")
			if *agent != "" {
				fmt.Printf("  set up for %s (its hook and R3V_ACTOR, in .claude/settings.local.json)\n", *agent)
			}
		})
	case "list":
		if len(pos) != 0 {
			return usage
		}
		list, err := r.WorkspaceStatuses()
		if err != nil {
			return err
		}
		out := []workspaceJSON{}
		for _, s := range list {
			w := workspaceOf(s.WorkspaceEntry)
			w.BranchName, w.Head, w.Ahead, w.Behind = s.BranchName, s.Head, s.Ahead, s.Behind
			w.Changes, w.Parked, w.Missing = s.Changes, s.Parked, s.Missing
			w.LastActivity = s.LastActivity.UTC().Format(time.RFC3339)
			out = append(out, w)
		}
		result("workspace", map[string]any{"main": r.MainRoot(), "workspaces": out}, func() {
			if len(out) == 0 {
				fmt.Println("no workspaces (r3v workspace add <folder> --branch NAME)")
			}
			for _, w := range out {
				if w.Missing {
					fmt.Printf("%s  %s  (the folder is gone: r3v workspace remove)\n", w.Folder, w.BranchName)
					continue
				}
				where := "ahead/behind not known (team not reached)"
				if w.Ahead != nil {
					where = fmt.Sprintf("%d ahead, %d behind %s", *w.Ahead, *w.Behind, w.Base)
				}
				fmt.Printf("%s  %s  %s, %d change(s) not committed  (last %s)\n", w.Folder, w.BranchName, where,
					w.Changes, w.LastActivity)
			}
		})
	case "remove":
		if len(pos) != 1 {
			return usage
		}
		folder, err := filepath.Abs(pos[0])
		if err != nil {
			return err
		}
		if err := r.RemoveWorkspace(folder, *force); err != nil {
			return err
		}
		result("workspace", map[string]any{"removed": folder}, func() {
			fmt.Printf("removed %s (its branch stays on the team)\n", folder)
		})
	default:
		return usage
	}
	return nil
}
