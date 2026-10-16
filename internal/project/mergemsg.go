package project

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// A merge's description is written for you from what comes in (the branch,
// how many versions, their titles), still yours to change: with lanes
// merged many times a day, "Merge branch lane/vfx" says nothing. It is
// stored text, in English like the other messages R3V writes.

// wipTitle: words in a title that say the version isn't finished (whole
// words: "wipe" isn't one).
var wipTitle = regexp.MustCompile(`(?i)\b(wip|draft|not for merge|do not merge)\b`)

// LooksWIP reports whether a version's title says it isn't finished: such
// versions are left out of a merge's summary, and marked in its preview.
func LooksWIP(message string) bool { return wipTitle.MatchString(manifest.Title(message)) }

// mergeSummaryLines is how many titles a merge's summary lists.
const mergeSummaryLines = 10

// mergeSummary describes merging incoming (the versions that come in, any
// order) from label: a first line with how many, then each one's title,
// oldest first, merges and unfinished ones left out, the same title once.
// No title to list: plain.
func mergeSummary(label, plain string, incoming []*Manifest) string {
	var titles []string
	seen := map[string]bool{}
	for _, m := range oldestFirst(incoming) {
		title := strings.TrimSpace(manifest.Title(m.Message))
		if len(m.Parents) > 1 || title == "" || LooksWIP(title) || seen[title] {
			continue
		}
		seen[title] = true
		titles = append(titles, title)
	}
	if len(titles) == 0 {
		return plain
	}
	n := "1 version"
	if len(incoming) != 1 {
		n = fmt.Sprintf("%d versions", len(incoming))
	}
	lines := []string{fmt.Sprintf("Merge %s: %s", label, n)}
	for i, title := range titles {
		if i == mergeSummaryLines {
			lines = append(lines, fmt.Sprintf("… and %d more", len(titles)-i))
			break
		}
		lines = append(lines, "- "+title)
	}
	return strings.Join(lines, "\n")
}

// oldestFirst orders versions each after its parents among them, the
// older first where either could come (made in the same second, the times
// alone can't tell).
func oldestFirst(vs []*Manifest) []*Manifest {
	in := map[string]bool{}
	for _, m := range vs {
		in[m.ID] = true
	}
	waiting := map[string]int{} // parents among vs not listed yet
	for _, m := range vs {
		for _, p := range m.Parents {
			if in[p] {
				waiting[m.ID]++
			}
		}
	}
	left := append([]*Manifest(nil), vs...)
	sort.Slice(left, func(i, j int) bool {
		if left[i].Time != left[j].Time {
			return left[i].Time < left[j].Time
		}
		return left[i].ID < left[j].ID
	})
	out := make([]*Manifest, 0, len(vs))
	for len(left) > 0 {
		i := 0
		for i < len(left)-1 && waiting[left[i].ID] > 0 {
			i++
		}
		m := left[i]
		left = append(left[:i], left[i+1:]...)
		out = append(out, m)
		for _, c := range left {
			for _, p := range c.Parents {
				if p == m.ID {
					waiting[c.ID]--
				}
			}
		}
	}
	return out
}

// incoming lists the versions reachable from target and not from head,
// newest first (their snapshots must be here).
func (r *Repo) incoming(head, target string) ([]*Manifest, error) {
	have, err := r.ancestors(head)
	if err != nil {
		return nil, err
	}
	all, err := r.ancestors(target)
	if err != nil {
		return nil, err
	}
	var out []*Manifest
	for id := range all {
		if !have[id] {
			m, err := r.Header(id)
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out, nil
}

// PlainMergeMessage is "Merge branch <name>": what a merge of branch name
// says when its description is left empty.
func (r *Repo) PlainMergeMessage(name string) string {
	return "Merge branch " + r.BranchLabel(name)
}

// mergeLabel names version id for a merge's summary: the branch whose
// latest it is, else the version.
func (r *Repo) mergeLabel(id string) string {
	if c, err := r.Client(); err == nil {
		if heads, err := c.Branches(r.Config.ProjectID); err == nil {
			for name, head := range heads {
				if head == id && name != r.BranchName() {
					return r.BranchLabel(name)
				}
			}
		}
	}
	return "version " + short(id)
}
