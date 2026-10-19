package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

func when(m *project.Manifest) string {
	if t, err := time.Parse(time.RFC3339, m.Time); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return m.Time
}

func cmdBranch(args []string) error {
	open := openRepo
	if len(args) == 0 || args[0] == "archived" || args[0] == "log" { // (only read)
		open = openRead
	}
	r, err := open()
	if err != nil {
		return err
	}
	if len(args) >= 2 && args[0] == "new" {
		// (the name may be several words: `r3v branch new Mia's verse`)
		var words []string
		agent := false
		for _, a := range args[1:] {
			if a == "--agent" || a == "-agent" {
				agent = true
				continue
			}
			words = append(words, a)
		}
		if len(words) == 0 {
			return usageError("usage: r3v branch new NAME [--agent]")
		}
		name := strings.Join(words, " ")
		if _, err := r.NewBranchWith(name, project.BranchOptions{Switch: true, Agent: agent}); err != nil {
			return err
		}
		what := "branch"
		if agent {
			what = "agent branch"
		}
		fmt.Printf("created %s %q from your current version and switched to it\n", what, name)
		fmt.Printf("versions you commit now go to this branch; merge back with `r3v switch main` + `r3v merge %q`\n", name)
		return nil
	}
	// archivedKey finds an archived branch by its key or its name.
	archivedKey := func(name string) string {
		if gone, err := r.ArchivedBranches(); err == nil {
			recs, _ := r.BranchRecords()
			for _, d := range gone {
				if d.Key == name || remote.SameBranchName(recs[d.Key].Name, name) {
					return d.Key
				}
			}
		}
		return name
	}
	if len(args) >= 2 && args[0] == "archive" {
		key, err := r.ResolveBranch(strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		if err := r.ArchiveBranch(key); err != nil {
			return err
		}
		fmt.Printf("archived branch %q; its versions stay (`r3v branch unarchive %s` brings it back)\n", key, key)
		return nil
	}
	if len(args) >= 2 && args[0] == "unarchive" {
		key := archivedKey(strings.Join(args[1:], " "))
		if err := r.UnarchiveBranch(key); err != nil {
			return err
		}
		fmt.Printf("branch %q is back where it was\n", key)
		return nil
	}
	if len(args) >= 2 && args[0] == "delete" {
		key := archivedKey(strings.Join(args[1:], " "))
		if err := r.DeleteBranch(key); err != nil {
			return err
		}
		fmt.Printf("deleted archived branch %q for good; its versions stay where other branches have them\n", key)
		return nil
	}
	if len(args) == 1 && args[0] == "archived" {
		gone, err := r.ArchivedBranches()
		if err != nil {
			return err
		}
		recs, _ := r.BranchRecords()
		names := r.MemberNames()
		for _, d := range gone {
			label := d.Key
			if n := recs[d.Key].Name; n != "" {
				label = n
			}
			by := names[d.By]
			if by == "" {
				by = "someone"
			}
			fmt.Printf("%-16s %s  archived by %s %s\n", label, short(d.Head), by, d.Time.Local().Format("2006-01-02 15:04"))
		}
		return nil
	}
	if len(args) >= 1 && args[0] == "log" && len(args) <= 2 {
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		return branchLog(r, name)
	}
	if len(args) > 0 {
		return errors.New("usage: r3v branch [new NAME [--agent] | archive NAME | archived | unarchive NAME | delete NAME | log [NAME]]")
	}
	branches, err := r.Branches()
	if err != nil {
		return err
	}
	recs, _ := r.BranchRecords()
	var names map[string]string
	for _, b := range branches {
		mark := "  "
		if b.Current {
			mark = "* "
		}
		latest := "(no versions yet)"
		if b.Latest != nil {
			latest = fmt.Sprintf("%s  %s  %-10s %s", short(b.Head), when(b.Latest), byLine(b.Latest.Author, b.Latest), manifest.Title(b.Latest.Message))
		}
		rec := recs[b.Name]
		name := b.Name
		if rec.Name != "" {
			name = rec.Name
		}
		if rec.Kind == remote.BranchAgent { // whose agent it is
			if names == nil {
				names = r.MemberNames()
			}
			kind := "agent"
			if owner := names[rec.Owner]; owner != "" {
				kind += " for " + owner
			}
			latest += "  (" + kind + ")"
		} else if rec.Kind != "" { // a kind a newer R3V knows
			latest += "  (" + rec.Kind + ")"
		}
		fmt.Printf("%s%-16s %s\n", mark, name, latest)
	}
	return nil
}

func cmdSwitch(args []string) error {
	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	force := fs.Bool("force", false, "discard uncommitted changes and unshared versions; ignore a running Live")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: r3v switch <branch> [--force]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if err := guardLiveAlways(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	key, err := r.ResolveBranch(pos[0])
	if err != nil {
		return err
	}
	res, err := r.SwitchBranch(key, *force)
	if err != nil {
		return err
	}
	result("switch", moveOf(r, res.Park, res.Relinked), func() {
		fmt.Printf("now on branch %q at %s\n", pos[0], short(res.To))
		printMerge(res)
		printPark(res.Park)
	})
	return nil
}

// previewJSON is what update or merge would bring in.
type previewJSON struct {
	// Action: up-to-date, ahead (nothing new), fast-forward (update only:
	// merging another branch always makes a merge version), merge.
	Action    string         `json:"action"`
	Versions  []versionJSON  `json:"versions"` // incoming, newest first
	Changes   []changeJSON   `json:"changes"`
	Conflicts []conflictJSON `json:"conflicts"` // need --strategy
	// Message: a merge's description written for you (what merge says
	// without -m).
	Message string `json:"message,omitempty"`
	// SameFilesAs: the merge version will have exactly this version's
	// files (you are behind it); its results carry over.
	SameFilesAs string `json:"same_files_as,omitempty"`
}

func previewOf(r *project.Repo, p *project.Preview) previewJSON {
	names := r.MemberNames()
	out := previewJSON{Action: p.Action, Versions: []versionJSON{}, Changes: []changeJSON{},
		Conflicts: conflictsJSON(p.Conflicts), Message: p.Message, SameFilesAs: p.SameFilesAs}
	for _, m := range p.Versions {
		v := versionOf(m, names)
		v.WIP = project.LooksWIP(m.Message)
		out.Versions = append(out.Versions, v)
	}
	for _, c := range p.Changes {
		ch := changeJSON{Path: c.Path, Status: c.Status, From: c.From, Edited: c.Edited}
		if c.SetDiff != nil {
			ch.SetChanges, ch.Weight = setLines(c.SetDiff.Render()), c.SetDiff.Weight()
		}
		out.Changes = append(out.Changes, ch)
	}
	return out
}

func printPreview(p *project.Preview, what string) {
	switch p.Action {
	case "up-to-date":
		fmt.Println("nothing new")
		return
	case "ahead":
		fmt.Println("nothing new: you already have all of " + what)
		return
	}
	fmt.Printf("%d new version(s) from %s:\n", len(p.Versions), what)
	for _, m := range p.Versions {
		fmt.Printf("  %s  %s  %-10s %s\n", short(m.ID), when(m), byLine(m.Author, m), manifest.Title(m.Message))
	}
	fmt.Println("\nchanges:")
	sym := map[string]string{"added": "+", "modified": "~", "deleted": "-"}
	for _, c := range p.Changes {
		fmt.Printf("%s %s\n", sym[c.Status], c.Path)
		if c.SetDiff != nil {
			for _, line := range strings.Split(c.SetDiff.Render(), "\n") {
				fmt.Println("    " + line)
			}
		}
	}
	if len(p.Conflicts) > 0 {
		fmt.Printf("\n%d conflict(s) with your versions (you will choose with --strategy ours|theirs|both):\n", len(p.Conflicts))
		for _, c := range p.Conflicts {
			fmt.Println("  ! " + c.String())
		}
	} else if p.Action == "merge" {
		fmt.Println("\nno conflicts: merges automatically")
	}
	if p.SameFilesAs != "" {
		fmt.Printf("the merge version will have exactly the files of %s\n", short(p.SameFilesAs))
	}
}

func cmdMergeBranch(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	preview := fs.Bool("preview", false, "show what would come in, change nothing")
	strategy := strategyFlag(fs)
	decided := decisionFlags(fs, strategy)
	force := fs.Bool("force", false, "merge even while Ableton Live is running")
	message := fs.String("m", "", "describe the merge version (default: what comes in, the versions' titles)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: r3v merge <branch> [-m message] [--preview] [--strategy ...] [--resolve <key>=<choice>]")
	}
	open := openRepo
	if *preview {
		open = openRead
	}
	r, err := open()
	if err != nil {
		return err
	}
	key, err := r.ResolveBranch(pos[0])
	if err != nil {
		return err
	}
	opts, err := decided.options(r)
	if err != nil {
		return err
	}
	if *preview {
		p, err := r.PreviewMerge(key)
		if err != nil {
			return err
		}
		result("merge", previewOf(r, p), func() { printPreview(p, "branch "+pos[0]) })
		return nil
	}
	if err := guardLiveAlways(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	before := r.Head()
	res, err := r.MergeBranch(key, *message, opts)
	if err != nil {
		return err
	}
	out := syncOf(res)
	out.ReopenSets = reopenSets(r, res, before, r.Head())
	result("merge", out, func() {
		switch res.Action {
		case "up-to-date", "ahead":
			fmt.Printf("nothing to merge: you already have everything from %s\n", pos[0])
		default:
			printMerge(res)
			reopen := ""
			if out.ReopenSets {
				reopen = "; reopen the set in Live"
			}
			fmt.Printf("merged %s into %s and shared it (%s)%s\n", pos[0], r.BranchName(), short(res.To), reopen)
		}
	})
	return nil
}

// guardLiveAlways refuses while a tool has the project's files open (a set
// in Live, a project in an editor): the command rewrites them.
func guardLiveAlways(r *project.Repo, force bool) error {
	if force {
		return nil
	}
	if check, what := openTool(r); check != "" {
		return openError(check, what, "")
	}
	return nil
}

// branchLog prints how the team's branches moved (who, when, from which
// version to which): to put back one moved by mistake, `r3v goto`
// the version it was on.
func branchLog(r *project.Repo, name string) error {
	moves, names, err := r.BranchLog(name)
	if err != nil {
		return err
	}
	if len(moves) == 0 {
		fmt.Println("no branch moves recorded")
		return nil
	}
	for _, m := range moves {
		who := names[m.By]
		if who == "" {
			who = "?"
		}
		what := short(m.From) + " -> " + short(m.To)
		switch {
		case m.From == "":
			what = "made at " + short(m.To)
		case m.To == "":
			what = "deleted (was " + short(m.From) + ")"
		}
		fmt.Printf("%s  %-14s %-16s %s\n", m.Time.Local().Format("2006-01-02 15:04:05"), who, m.Branch, what)
	}
	return nil
}
