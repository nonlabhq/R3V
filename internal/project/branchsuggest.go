package project

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/nonlabhq/r3v/internal/remote"
)

// NoBranchError: the team has no branch called Name. Suggestion is the
// closest one's name ("" when none is close): an agent that typed
// "lane/design" for "lane-design" learns which it meant.
type NoBranchError struct {
	Name       string
	Suggestion string
	// Hint, when there is no suggestion: what to do instead.
	Hint string
}

func (e *NoBranchError) Error() string {
	msg := fmt.Sprintf("no branch %q in the team", e.Name)
	switch {
	case e.Suggestion != "":
		msg += fmt.Sprintf(" (did you mean %s?)", e.Suggestion)
	case e.Hint != "":
		msg += " (" + e.Hint + ")"
	}
	return msg
}

// noBranch is the error for a branch name not among heads, with the
// closest branch's name (its label, as people call it) when one is close.
func (r *Repo) noBranch(c remote.Backend, heads map[string]string, name, hint string) error {
	var recs map[string]remote.BranchRecord
	if store, ok := remote.BranchRecordsOf(c); ok {
		recs, _ = store.BranchRecords(r.Config.ProjectID)
	}
	labels := map[string]string{} // key or label: what to suggest for it
	for k := range heads {
		label := branchLabel(recs, k)
		labels[k], labels[label] = label, label
	}
	if b := r.BranchName(); labels[b] == "" {
		labels[b] = r.BranchLabel(b)
	}
	return &NoBranchError{Name: name, Suggestion: closestName(name, labels), Hint: hint}
}

// closestName is the value of the key in names nearest to name: one the
// same but for case and separators (- _ / . and spaces) first, else one a
// few typos away; "" when none is near.
func closestName(name string, names map[string]string) string {
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys) // (ties: the same answer each time)
	want := looseName(name)
	best, bestDist := "", -1
	for _, k := range keys {
		d := editDistance(want, looseName(k))
		if bestDist < 0 || d < bestDist {
			best, bestDist = names[k], d
		}
	}
	n := len([]rune(want))
	limit := max(1, n/4)
	if n >= 4 {
		limit = max(2, limit) // ("mian" for "main")
	}
	if bestDist < 0 || bestDist > limit {
		return ""
	}
	return best
}

// looseName folds what people mix up in a branch name: case, and the
// separators between words.
func looseName(s string) string {
	s = strings.ToLower(norm.NFC.String(strings.TrimSpace(s)))
	return strings.Map(func(r rune) rune {
		switch r {
		case '_', '/', '.', ' ', '\\':
			return '-'
		}
		return r
	}, s)
}

// editDistance is the Levenshtein distance between a and b, in runes.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
