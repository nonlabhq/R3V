package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/nonlabhq/r3v/internal/textmerge"
)

// r3v merge-files: a 3-way merge of three files on disk, each kind the way
// it merges (Live Sets track by track, text place by place, any other file
// whole: the side that changed it). Places both sides changed differently
// stop it, unless --strategy says which side wins (both: ours then theirs,
// for text). merge-sets is its Live Sets' name, as before.

// fileMergeJSON is r3v merge-files --json.
type fileMergeJSON struct {
	Kind      string `json:"kind"` // set | text | binary
	Out       string `json:"out"`
	Conflicts int    `json:"conflicts"`          // places both sides changed differently
	Resolved  string `json:"resolved,omitempty"` // how they were settled (--strategy)
}

func cmdMergeFiles(args []string) error {
	fs := flag.NewFlagSet("merge-files", flag.ContinueOnError)
	out := fs.String("o", "", "where the merged file goes")
	strategy := fs.String("strategy", "fail", "places both sides changed: fail (stop), ours, theirs, or both (text: ours then theirs)")
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) != 3 || *out == "" {
		return usageError("usage: r3v merge-files <base> <ours> <theirs> -o <out> [--strategy fail|ours|theirs|both]")
	}
	switch *strategy {
	case "fail", "ours", "theirs", "both":
	default:
		return usageError("--strategy is fail, ours, theirs or both")
	}
	if isSetFile(pos[0]) && isSetFile(pos[1]) && isSetFile(pos[2]) {
		if jsonMode {
			return usageError("merge-files of Live Sets answers in text (no --json yet)")
		}
		code, err := cmdMerge(append(pos, "-o", *out, "--strategy", *strategy))
		if err == nil && code != 0 {
			err = &cliError{Code: "merge_conflict", Exit: exitConflict, Err: errors.New("tracks both sides changed: use --strategy ours|theirs|both")}
		}
		return err
	}
	var sides [3][]byte
	for i, p := range pos {
		if sides[i], err = os.ReadFile(p); err != nil {
			return err
		}
	}
	base, ours, theirs := sides[0], sides[1], sides[2]
	res := fileMergeJSON{Kind: "binary", Out: *out}
	var merged []byte
	if isText(base) && isText(ours) && isText(theirs) {
		res.Kind = "text"
		m := textmerge.Hunks(base, ours, theirs)
		res.Conflicts = len(m.Hunks())
		choices := make([]string, res.Conflicts)
		if res.Conflicts > 0 {
			if *strategy == "fail" {
				return mergeConflict(res)
			}
			for i := range choices {
				choices[i] = *strategy
			}
			res.Resolved = *strategy
		}
		if merged, _, err = m.Resolve(choices); err != nil {
			return err
		}
	} else {
		switch {
		case bytes.Equal(ours, theirs) || bytes.Equal(theirs, base):
			merged = ours
		case bytes.Equal(ours, base):
			merged = theirs
		default:
			res.Conflicts = 1
			switch *strategy {
			case "ours":
				merged = ours
			case "theirs":
				merged = theirs
			case "both":
				return usageError("a file that isn't text can't keep both sides: --strategy ours or theirs")
			default:
				return mergeConflict(res)
			}
			res.Resolved = *strategy
		}
	}
	if err := os.WriteFile(*out, merged, 0o644); err != nil {
		return err
	}
	result("merge-files", res, func() {
		if res.Conflicts > 0 {
			fmt.Printf("merged (%s): %d place(s) both sides changed, settled as %s\n", res.Kind, res.Conflicts, res.Resolved)
		} else {
			fmt.Printf("merged (%s): no place both sides changed\n", res.Kind)
		}
		fmt.Printf("wrote %s\n", *out)
	})
	return nil
}

func mergeConflict(res fileMergeJSON) error {
	what := "places"
	if res.Kind == "binary" {
		what = "the whole file (it isn't text)"
	}
	return &cliError{Code: "merge_conflict", Exit: exitConflict,
		Err:  fmt.Errorf("both sides changed %s differently (%d): nothing written", what, res.Conflicts),
		Hint: "ask the user which side wins; then --strategy ours|theirs (text: both)"}
}
