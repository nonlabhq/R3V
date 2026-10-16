package cli

import (
	"flag"
	"fmt"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// undoMergeUsage is undo-merge's line in the usage (Nightly).
const undoMergeUsage = `
taking back a merge (Nightly):
  undo-merge VERSION [-m MESSAGE] [--strategy ...] [--resolve KEY=CHOICE]
                                         a new version without what that merge brought in (the
                                         history stays); what changed since is merged, conflicts
                                         decided as a merge's
`

// cmdUndoMerge: r3v undo-merge VERSION, a new version on the current
// branch that takes back what a merge version brought in. It rewrites the
// project's files: uncommitted changes and a tool holding them stop it.
func cmdUndoMerge(args []string) error {
	fs := flag.NewFlagSet("undo-merge", flag.ContinueOnError)
	message := fs.String("m", "", `describe the new version (default "Undo merge <id> (<its title>)")`)
	strategy := strategyFlag(fs)
	decided := decisionFlags(fs, strategy)
	force := fs.Bool("force", false, "undo even while a tool has the project's files open")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError("usage: r3v undo-merge <version> [-m message] [--strategy ...] [--resolve <key>=<choice>]")
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
	m, res, err := r.UndoMerge(pos[0], *message, opts)
	if err != nil {
		return err
	}
	out := syncJSON{Action: "local", Merged: []string{}, Relink: []string{}} // (in no team)
	if res != nil {
		out = syncOf(res)
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
			if res.Action == "published" {
				fmt.Println("shared with the team; reopen the set in Live")
			}
		}
	})
	return nil
}
