package project

import (
	"fmt"

	"github.com/nonlabhq/r3v/internal/textmerge"
)

// Conflict keys of a whole file (file:<path>) and of a text file's hunk
// (text:<path>#<n>).
const (
	fileKeyPrefix = "file:"
	hunkKeyPrefix = "text:"
)

// HunkKey is the conflict key of a text file's nth hunk (from 1, in file
// order).
func HunkKey(path string, n int) string {
	return fmt.Sprintf("%s%s%s%d", hunkKeyPrefix, path, setKeySep, n)
}

// HunkSide is a hunk's lines on one side: Start is the first line's number
// (from 1); with no lines (Count 0), where they would be (before Start).
type HunkSide = textmerge.Side

// mergeHunks merges a text file line by line, deciding each hunk both sides
// changed by its key or the strategy. clean is false while any is undecided
// (returned, nothing merged) or the file is to be chosen whole: a decision
// for file:<path> keeps working as before hunks.
func mergeHunks(path string, base, ours, theirs []byte, opts MergeOptions) (
	merged []byte, log []string, clean bool, undecided []ConflictItem, err error) {
	res := textmerge.Hunks(base, ours, theirs)
	hunks := res.Hunks()
	if len(hunks) == 0 {
		merged, clean, err = res.Resolve(nil)
		return merged, nil, clean, nil, err
	}
	if _, whole := opts.Resolutions[fileKeyPrefix+path]; whole {
		return nil, nil, false, nil, nil
	}
	choices := make([]string, len(hunks))
	for i, h := range hunks {
		key, unit, what := HunkKey(path, i+1), hunkUnit(h), hunkWhat(h)
		switch c := opts.choice(key); c {
		case "ours", "theirs", "both":
			choices[i] = c
			log = append(log, fmt.Sprintf("%s: %s %s -> %s", path, unit, what, hunkTaken[c]))
		default:
			b, o, t := h.Base, h.Ours, h.Theirs
			undecided = append(undecided, ConflictItem{Key: key, File: path, Unit: unit, Description: what,
				CanKeepBoth: true, Hunk: i + 1, Base: &b, Ours: &o, Theirs: &t})
		}
	}
	if len(undecided) > 0 {
		return nil, nil, false, undecided, nil
	}
	merged, clean, err = res.Resolve(choices)
	return merged, log, clean, nil, err
}

var hunkTaken = map[string]string{"ours": "kept yours", "theirs": "took theirs", "both": "kept both (yours first)"}

// hunkUnit says where a hunk is in your file: "lines 40–52".
func hunkUnit(h textmerge.Hunk) string {
	o := h.Ours
	switch {
	case o.Count == 1:
		return fmt.Sprintf("line %d", o.Start)
	case o.Count > 1:
		return fmt.Sprintf("lines %d–%d", o.Start, o.Start+o.Count-1)
	case o.Start > 1:
		return fmt.Sprintf("after line %d", o.Start-1)
	}
	return "at the start"
}

// hunkWhat describes a hunk in the words used for a whole file.
func hunkWhat(h textmerge.Hunk) string {
	switch {
	case h.Base.Count == 0:
		return "added on both sides"
	case h.Ours.Count == 0:
		return "deleted by you, changed by others"
	case h.Theirs.Count == 0:
		return "changed by you, deleted by others"
	}
	return "changed on both sides"
}
