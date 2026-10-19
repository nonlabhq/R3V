package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/project"
)

// undoMergeUsage is undo-merge's line in the usage (Nightly).
const undoMergeUsage = `
taking back a merge (Nightly):
  undo-merge VERSION [-m MESSAGE] [--strategy keep|undo|both] [--resolve KEY=keep|undo|both]
                                         a new version without what that merge brought in (the
                                         history stays); what changed since is merged, a conflict
                                         decided keep (as it is now) or undo (as before the merge)
`

// undoChoices are undo-merge's words for a conflict's sides: there "ours"
// (the current version) keeps the change and "theirs" (the merge's first
// parent) undoes it, which reads the wrong way round.
var undoChoices = map[string]string{"keep": "ours", "undo": "theirs"}

// undoConflict is a conflict of undo-merge's with its choices in its words.
func undoConflict(err error) error {
	var mc *project.MergeConflictError
	if !errors.As(err, &mc) {
		return err
	}
	cs := conflictsJSON(mc.Conflicts)
	for i := range cs {
		cs[i].Choices = []string{"keep", "undo"}
		if cs[i].CanKeepBoth {
			cs[i].Choices = append(cs[i].Choices, "both")
		}
	}
	return &cliError{Code: "merge_conflict", Exit: exitConflict, Err: err, Conflicts: cs,
		Hint: "choose for each: keep (as it is now) or undo (as before the merge): --strategy keep|undo|both, or each by its key: --resolve <key>=keep|undo|both"}
}

// cmdUndoMerge: r3v undo-merge VERSION, a new version on the current
// branch that takes back what a merge version brought in. It rewrites the
// project's files: uncommitted changes and a tool holding them stop it.
func cmdUndoMerge(args []string) error {
	fs := flag.NewFlagSet("undo-merge", flag.ContinueOnError)
	message := fs.String("m", "", `describe the new version (default "Undo merge <id> (<its title>)")`)
	strategy := fs.String("strategy", "fail",
		"where the same thing changed again since the merge: fail (ask), keep (as it is now), undo (as before the merge), both (keep both); ours and theirs as in merge")
	decided := decisionFlagsAs(fs, strategy, "keep|undo|both", undoChoices)
	force := fs.Bool("force", false, "undo even while a tool has the project's files open")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError("usage: r3v undo-merge <version> [-m message] [--strategy keep|undo|both] [--resolve <key>=keep|undo|both]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	opts, err := decided.options(r)
	if err != nil {
		return err
	}
	if err := guardLiveAlways(r, *force); err != nil {
		return err
	}
	defer tidy(r)
	before := r.Head()
	m, res, err := r.UndoMerge(pos[0], *message, opts)
	if err != nil {
		return undoConflict(err)
	}
	out := syncJSON{Action: "local", Merged: []string{}, Relink: []string{}} // (in no team)
	if res != nil {
		out = syncOf(res)
		out.ReopenSets = r.SetsChanged(before, r.Head()) // (an undo rewrites what the merge brought in)
	}
	if m != nil {
		v := versionOf(m, nil)
		out.Saved = &v
	}
	result("undo-merge", out, func() {
		if m != nil {
			fmt.Printf("committed version %s  %s\n", short(m.ID), manifest.Title(m.Message))
		}
		if res != nil {
			printMerge(res)
			switch {
			case res.Action == "published" && out.ReopenSets:
				fmt.Println("shared with the team; reopen the set in Live")
			case res.Action == "published":
				fmt.Println("shared with the team")
			}
		}
	})
	return nil
}
