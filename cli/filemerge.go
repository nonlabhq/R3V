package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/handlers"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/textmerge"
)

// r3v merge-files: a 3-way merge of three files on disk, each the way a
// branch merge merges it: Live Sets track by track, a file a handler takes
// with it (text place by place, Unity's scenes with UnityYAMLMerge), any
// other file whole (the side that changed it). The handler is the
// project's when -o is in one (its rules and presets), else the presets'
// by the file's name, and for a file they don't know its content (text
// line by line, anything else whole); --as text or whole overrides it. Places both sides
// changed differently stop it, unless --strategy says which side wins
// (both: ours then theirs, for text). merge-sets is its Live Sets' name,
// as before.

// handlerWhole: no handler takes the file, so it is chosen whole.
const handlerWhole = profile.MergeWhole

// fileMergeJSON is r3v merge-files --json.
type fileMergeJSON struct {
	Kind string `json:"kind"` // set | text | binary: what the files are
	// Handler: how they were merged: text, whole, or a preset's handler
	// (unity-yaml, …).
	Handler   string `json:"handler"`
	Out       string `json:"out"`
	Conflicts int    `json:"conflicts"`          // places both sides changed differently
	Resolved  string `json:"resolved,omitempty"` // how they were settled (--strategy)
}

func cmdMergeFiles(args []string) error {
	fs := flag.NewFlagSet("merge-files", flag.ContinueOnError)
	out := fs.String("o", "", "where the merged file goes")
	strategy := fs.String("strategy", "fail", "places both sides changed: fail (stop), ours, theirs, or both (text: ours then theirs)")
	as := fs.String("as", "", "merge them as text (line by line) or whole, whatever kind they are")
	pos, err := parseArgs(fs, args)
	if err != nil || len(pos) != 3 || *out == "" {
		return usageError("usage: r3v merge-files <base> <ours> <theirs> -o <out> [--strategy fail|ours|theirs|both] [--as text|whole]")
	}
	switch *strategy {
	case "fail", "ours", "theirs", "both":
	default:
		return usageError("--strategy is fail, ours, theirs or both")
	}
	switch *as {
	case "", "text", handlerWhole:
	default:
		return usageError("--as is text or whole")
	}
	if *as == "" && isSetFile(pos[0]) && isSetFile(pos[1]) && isSetFile(pos[2]) {
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
	text := isText(base) && isText(ours) && isText(theirs)
	res := fileMergeJSON{Kind: "binary", Handler: *as, Out: *out}
	if text {
		res.Kind = "text"
	}
	switch {
	case res.Handler == "":
		res.Handler = mergeHandlerFor(*out)
		if res.Handler == "" { // (a file no one knows, outside a project: by its content, as git's mergetool)
			res.Handler = "text"
		}
		if res.Handler == "text" && !text {
			res.Handler = handlerWhole // (as in a branch merge: "text" that isn't is chosen whole)
		}
	case res.Handler == "text" && !text:
		return usageError("--as text: the files aren't all text")
	}
	var merged []byte
	whole := false // changed on both sides, and chosen whole
	switch {
	case bytes.Equal(ours, theirs) || bytes.Equal(theirs, base):
		merged = ours
	case bytes.Equal(ours, base):
		merged = theirs
	case res.Handler == handlerWhole:
		whole = true
	case res.Handler == "text":
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
		if *as == "" && res.Conflicts == 0 && breaksJSON(*out, sides, merged) {
			res.Handler, whole = handlerWhole, true // (as in a branch merge: the whole file instead)
		}
	default:
		clean := false
		if merged, clean, err = handlers.Merge(res.Handler)(base, ours, theirs); err != nil {
			return fmt.Errorf("%s merge: %w", res.Handler, err)
		}
		whole = !clean
	}
	if whole {
		res.Conflicts = 1
		switch *strategy {
		case "ours":
			merged = ours
		case "theirs":
			merged = theirs
		case "both":
			return usageError("a file merged whole can't keep both sides: --strategy ours or theirs")
		default:
			return mergeConflict(res)
		}
		res.Resolved = *strategy
	}
	if err := os.WriteFile(*out, merged, 0o644); err != nil {
		return err
	}
	result("merge-files", res, func() {
		if res.Conflicts > 0 {
			fmt.Printf("merged (%s, %s): %d place(s) both sides changed, settled as %s\n", res.Kind, res.Handler, res.Conflicts, res.Resolved)
		} else {
			fmt.Printf("merged (%s, %s): no place both sides changed\n", res.Kind, res.Handler)
		}
		fmt.Printf("wrote %s\n", *out)
	})
	return nil
}

// mergeHandlerFor is the merge a branch merge would use for the file at
// out: its project's rules when out is in one, else the presets' by its
// name; whole when none applies or this R3V lacks the handler. Outside a
// project a file no preset knows (KnownByName) gets "": its content
// decides (merge-files is git's mergetool in ordinary repositories too).
func mergeHandlerFor(out string) string {
	var h profile.Handler
	byName := true
	if abs, err := filepath.Abs(out); err == nil {
		if r, err := project.Open(filepath.Dir(abs)); err == nil {
			if rel, err := filepath.Rel(r.Root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				if rules, _ := r.Profile(); rules != nil {
					h, byName = rules.Handler(filepath.ToSlash(rel)), false
				}
			}
		}
	}
	if byName {
		if h = profile.HandlerByName(out); h.Merge == "" && !profile.KnownByName(out) {
			return ""
		}
	}
	if h.Merge == "" || handlers.Merge(h.Merge) == nil {
		return handlerWhole
	}
	return h.Merge
}

// breaksJSON: out is a .json file, valid on every side, that merged isn't
// (as a branch merge checks: two changes side by side in a list, say).
func breaksJSON(out string, sides [3][]byte, merged []byte) bool {
	if !strings.EqualFold(filepath.Ext(out), ".json") {
		return false
	}
	for _, d := range sides {
		if !json.Valid(d) {
			return false
		}
	}
	return !json.Valid(merged)
}

func mergeConflict(res fileMergeJSON) error {
	what := "places"
	switch {
	case res.Kind == "binary":
		what = "the whole file (it isn't text)"
	case res.Handler != "text":
		what = "the whole file (" + res.Handler + ", as a branch merge would)"
	}
	hint := "ask the user which side wins; then --strategy ours|theirs (text: both)"
	if res.Kind == "text" && res.Handler != "text" {
		hint = "ask the user which side wins; then --strategy ours|theirs (or --as text, to merge it line by line)"
	}
	return &cliError{Code: "merge_conflict", Exit: exitConflict,
		Err:  fmt.Errorf("both sides changed %s differently (%d): nothing written", what, res.Conflicts),
		Hint: hint}
}
