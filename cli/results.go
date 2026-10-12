package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
)

// Results (project/results.go): what a build or the tests said about a
// version, recorded by whoever ran them; shown by r3v results and in
// workspace list.

const resultsUsage = `
results (what builds and tests said about a version; Nightly):
  result <check> pass|fail|running [-m NOTE] [--version V]
                                         record a check's result for the version you're on
                                         (e.g. result tests pass -m "cargo test: 214 passed")
  results [VERSION]                      the checks' latest results for a version
`

func cmdResult(args []string) error {
	fs := flag.NewFlagSet("result", flag.ContinueOnError)
	note := fs.String("m", "", "what it said (a line)")
	version := fs.String("version", "", "the version (default: the one you're on)")
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) != 2 {
		return usageError("usage: r3v result <check> pass|fail|running [-m NOTE] [--version V]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	id, res, err := r.SetResult(*version, pos[0], pos[1], *note)
	if errors.Is(err, project.ErrBadResult) {
		return usageError("%v", err)
	}
	if err != nil {
		return err
	}
	result("result", map[string]any{"version": id, "check": pos[0], "result": res}, func() {
		fmt.Printf("%s: %s on version %s", pos[0], res.State, short(id))
		if res.Changes > 0 {
			fmt.Printf(" (with %d change(s) not committed)", res.Changes)
		}
		fmt.Println()
	})
	return nil
}

func cmdResults(args []string) error {
	fs := flag.NewFlagSet("results", flag.ContinueOnError)
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) > 1 {
		return usageError("usage: r3v results [VERSION]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	ref := "HEAD"
	if len(pos) == 1 {
		ref = pos[0]
	}
	id, err := r.Resolve(ref)
	if err != nil {
		return err
	}
	all := r.Results(id)
	result("results", map[string]any{"version": id, "results": all}, func() {
		if len(all) == 0 {
			fmt.Printf("no results for version %s (r3v result <check> pass|fail)\n", short(id))
			return
		}
		for _, n := range project.ResultNames(all) {
			res := all[n]
			fmt.Printf("%-12s %-8s %s", n, res.State, localTime(res.Time))
			if res.Actor != "" {
				fmt.Printf("  by %s", res.Actor)
			}
			if res.Changes > 0 {
				fmt.Printf("  (with %d change(s) not committed)", res.Changes)
			}
			if res.Note != "" {
				fmt.Printf("  %s", res.Note)
			}
			fmt.Println()
		}
	})
	return nil
}

// resultsLine is a version's results in a line: "tests ✓ build ✗".
func resultsLine(all map[string]project.Result) string {
	var parts []string
	for _, n := range project.ResultNames(all) {
		mark := map[string]string{"pass": "✓", "fail": "✗", "running": "…"}[all[n].State]
		parts = append(parts, n+" "+mark)
	}
	return strings.Join(parts, "  ")
}
