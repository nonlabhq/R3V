// Package textmerge merges text files line by line (a three-way merge, like
// diff3): changes made on one side are taken; where both sides changed the
// same lines differently, the merge isn't clean.
package textmerge

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/nonlabhq/r3v/internal/textdiff"
)

// Merge merges ours and theirs, both changed from base. clean is false when
// the changes touch the same lines differently.
func Merge(base, ours, theirs []byte) (merged []byte, clean bool, err error) {
	res := Hunks(base, ours, theirs)
	if len(res.Hunks()) > 0 {
		return nil, false, nil
	}
	return res.Resolve(nil)
}

// Side is a hunk's lines on one side: Start is the first line's number
// (from 1); with no lines (Count 0), where they would be (before Start).
type Side struct {
	Start int    `json:"start"`
	Count int    `json:"count"`
	Text  string `json:"text"`
}

// Hunk is one place both sides changed differently.
type Hunk struct {
	Base, Ours, Theirs Side
}

// Result is a merge as parts in file order: the text merged on its own, and
// the hunks between it that need a choice, so one can be finished hunk by
// hunk.
type Result struct {
	parts []part
	eol   string // the files' line ending
}

type part struct {
	text string
	hunk *Hunk
}

// Hunks merges ours and theirs, both changed from base, keeping each place
// that collides as a hunk.
func Hunks(base, ours, theirs []byte) *Result {
	b, o, t := lines(base), lines(ours), lines(theirs)
	mo, mt := match(b, o), match(b, t)
	res := &Result{eol: newline(ours, theirs, base)}
	var clean strings.Builder
	flush := func() {
		if clean.Len() > 0 {
			res.parts = append(res.parts, part{text: clean.String()})
			clean.Reset()
		}
	}
	i, j, k := 0, 0, 0
	for i < len(b) || j < len(o) || k < len(t) {
		// The same base line on both sides, where both are now: unchanged.
		if i < len(b) && mo[i] == j && mt[i] == k {
			clean.WriteString(b[i])
			i, j, k = i+1, j+1, k+1
			continue
		}
		// The next base line both sides still have ends this chunk.
		i2, j2, k2 := len(b), len(o), len(t)
		for x := i; x < len(b); x++ {
			if mo[x] >= j && mt[x] >= k {
				i2, j2, k2 = x, mo[x], mt[x]
				break
			}
		}
		bc, oc, tc := b[i:i2], o[j:j2], t[k:k2]
		switch {
		case equal(oc, bc):
			clean.WriteString(strings.Join(tc, ""))
		case equal(tc, bc), equal(oc, tc):
			clean.WriteString(strings.Join(oc, ""))
		default:
			flush()
			res.parts = append(res.parts, part{hunk: &Hunk{side(bc, i), side(oc, j), side(tc, k)}})
		}
		i, j, k = i2, j2, k2
	}
	flush()
	return res
}

func side(ls []string, at int) Side {
	n := len(ls)
	if n > 0 && ls[n-1] == "" { // (what follows a file's last newline)
		n--
	}
	return Side{Start: at + 1, Count: n, Text: strings.Join(ls, "")}
}

// Hunks lists the places that need a choice, in file order.
func (r *Result) Hunks() []Hunk {
	var out []Hunk
	for _, p := range r.parts {
		if p.hunk != nil {
			out = append(out, *p.hunk)
		}
	}
	return out
}

// Resolve puts the merged text together with a choice for each hunk, in
// order: ours, theirs or both (ours then theirs).
func (r *Result) Resolve(choices []string) ([]byte, bool, error) {
	var out strings.Builder
	n := 0
	for _, p := range r.parts {
		if p.hunk == nil {
			out.WriteString(p.text)
			continue
		}
		if n >= len(choices) {
			return nil, false, fmt.Errorf("hunk %d has no choice", n+1)
		}
		h := p.hunk
		switch choices[n] {
		case "ours":
			out.WriteString(h.Ours.Text)
		case "theirs":
			out.WriteString(h.Theirs.Text)
		case "both":
			out.WriteString(h.Ours.Text)
			// Ours ending the file without a newline would run into theirs.
			if h.Ours.Text != "" && h.Theirs.Text != "" && !strings.HasSuffix(h.Ours.Text, "\n") {
				out.WriteString(r.eol)
			}
			out.WriteString(h.Theirs.Text)
		default:
			return nil, false, fmt.Errorf("hunk %d: unknown choice %q", n+1, choices[n])
		}
		n++
	}
	return []byte(out.String()), true, nil
}

// newline is the line ending the files use: their first line's.
func newline(files ...[]byte) string {
	for _, f := range files {
		if i := bytes.IndexByte(f, '\n'); i >= 0 {
			if i > 0 && f[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}
	return "\n"
}

func lines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.SplitAfter(string(data), "\n")
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// match maps each line of a to its line in b along a shortest diff (-1 when
// unmatched).
func match(a, b []string) []int {
	out := make([]int, len(a))
	for i := range out {
		out[i] = -1
	}
	for _, op := range textdiff.Diff(a, b) {
		if op.Kind == '=' {
			out[op.A] = op.B
		}
	}
	return out
}
