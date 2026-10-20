package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nonlabhq/r3v/internal/actor"
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
	Head   string `json:"head,omitempty"`
	Ahead  *int   `json:"ahead,omitempty"`
	Behind *int   `json:"behind,omitempty"`
	// Unshared: its versions its branch on the team lacks (commit --local:
	// a merge here can't see them); null when the team can't be reached.
	Unshared     *int   `json:"unshared,omitempty"`
	Changes      int    `json:"changes"` // files changed, not committed
	Parked       int    `json:"parked"`  // sets of parked changes
	LastActivity string `json:"last_activity,omitempty"`
	Missing      bool   `json:"missing,omitempty"` // the folder is gone
	// Results: what the checks said about the version it is on.
	Results map[string]project.Result `json:"results,omitempty"`
	// Merged (list): all its work is in its base branch, nothing changed or
	// parked: it can be removed, nothing lost (agent tools leave such ones).
	Merged *bool `json:"merged,omitempty"`
}

func workspaceOf(e project.WorkspaceEntry) workspaceJSON {
	return workspaceJSON{Folder: e.Folder, Branch: e.Branch, BranchName: e.Branch, Base: e.Base,
		Created: e.Created.Format(time.RFC3339)}
}

func cmdWorkspace(args []string) error {
	usage := errors.New("usage: r3v workspace add [<folder>] --branch NAME [--from BRANCH|VERSION] [--agent TOOL] | list [--against BRANCH] | base <folder> <branch> | remove <folder> [--force]")
	if len(args) == 0 {
		return usage
	}
	fs := flag.NewFlagSet("workspace", flag.ContinueOnError)
	branch := fs.String("branch", "", "the name of the workspace's branch (made for it)")
	from := fs.String("from", "", "a branch or version to start from (default: where this copy is)")
	agent := fs.String("agent", "", "set it up for an agent tool (claude-code, codex, opencode...): its identity, and Claude Code's hook and R3V_ACTOR")
	force := fs.Bool("force", false, "remove even with work not merged or not committed (it is lost)")
	against := fs.String("against", "", "list: ahead and behind this branch instead of each one's base")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return err
	}
	open := openRepo
	if args[0] == "list" { // (only reads)
		open = openRead
	}
	r, err := open()
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if len(pos) > 1 || *branch == "" {
			return usage
		}
		folder := r.DefaultWorkspaceFolder(*branch) // (Song.workspaces beside the project)
		if len(pos) == 1 {
			folder = pos[0]
		}
		if *agent != "" {
			tool, ok := actor.ToolName(*agent)
			if !ok {
				return usageError("--agent %q: an agent tool's name, such as claude-code, codex or opencode", *agent)
			}
			*agent = tool
		}
		e, err := r.AddWorkspace(folder, project.WorkspaceOptions{Branch: *branch, From: *from, Agent: *agent != ""})
		if err != nil {
			return err
		}
		if *agent != "" { // (the workspace is made: set up, or said why not)
			ws := filepath.Join(e.Folder, ".r3v")
			if _, _, err := agentsetup.Setup(e.Folder, ws, agentsetup.Options{Tool: *agent, Hook: hookCommand(), Steps: project.Steps}); err != nil {
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
		list, err := r.WorkspaceStatusesAgainst(*against)
		if err != nil {
			return err
		}
		vs := "" // what ahead and behind count against ("": each one's base)
		if *against != "" {
			if vs, err = r.ResolveBranch(*against); err != nil {
				return err
			}
		}
		out := []workspaceJSON{}
		for _, s := range list {
			w := workspaceOf(s.WorkspaceEntry)
			w.BranchName, w.Head, w.Ahead, w.Behind, w.Unshared = s.BranchName, s.Head, s.Ahead, s.Behind, s.Unshared
			w.Changes, w.Parked, w.Missing, w.Results = s.Changes, s.Parked, s.Missing, s.Results
			w.LastActivity = s.LastActivity.UTC().Format(time.RFC3339)
			if !s.Missing {
				w.Merged = &s.Merged
			}
			out = append(out, w)
		}
		res := map[string]any{"main": r.MainRoot(), "workspaces": out}
		if vs != "" {
			res["against"] = vs
		}
		result("workspace", res, func() {
			if len(out) == 0 {
				fmt.Println("no workspaces (r3v workspace add --branch NAME)")
			}
			for _, w := range out {
				if w.Missing {
					fmt.Printf("%s  %s  (the folder is gone: r3v workspace remove)\n", w.Folder, w.BranchName)
					continue
				}
				where := "ahead/behind not known (team not reached)"
				if w.Ahead != nil {
					of := w.Base
					if vs != "" {
						of = vs
					}
					where = fmt.Sprintf("%d ahead, %d behind %s", *w.Ahead, *w.Behind, r.BranchLabel(of))
				}
				fmt.Printf("%s  %s  %s, %d change(s) not committed  (last %s)\n", w.Folder, w.BranchName, where,
					w.Changes, w.LastActivity)
				if w.Unshared != nil && *w.Unshared > 0 {
					fmt.Printf("  %d version(s) not shared yet (committed --local): merging %s doesn't see them until `r3v commit` there shares them\n",
						*w.Unshared, w.BranchName)
				}
				if w.Merged != nil && *w.Merged {
					fmt.Printf("  all merged into %s, nothing changed: can be removed (r3v workspace remove %q)\n",
						r.BranchLabel(w.Base), w.Folder)
				}
				if line := resultsLine(w.Results); line != "" {
					fmt.Println("  " + line)
				}
			}
		})
	case "base":
		if len(pos) != 2 {
			return usage
		}
		e, err := r.SetWorkspaceBase(pos[0], pos[1])
		if err != nil {
			return err
		}
		out := workspaceOf(*e)
		out.BranchName = r.BranchLabel(e.Branch)
		result("workspace", map[string]any{"workspace": out}, func() {
			fmt.Printf("workspace %s (%s) merges into %s now\n", e.Folder, out.BranchName, r.BranchLabel(e.Base))
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
