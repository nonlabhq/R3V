package desktop

import (
	"sort"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/project"
)

// branchVersionCounts counts each branch's own versions: those made on it
// (as the history graph puts them on its line, shared/branchGraph.ts), and
// older ones (from before a version said its branch) down its first parents
// from its latest version, until a version another branch has: main's walked
// first, then yours, then the others'. So a branch counts from where it
// started from another one, main from the first version; what a merge
// brought in stays the other branch's. byID: the project's versions.
func branchVersionCounts(byID map[string]*project.Manifest, branches []project.BranchInfo) map[string]int {
	counts := map[string]int{}
	claimed := map[string]bool{}
	for id, m := range byID {
		if m.Branch != "" {
			counts[m.Branch]++
			claimed[id] = true
		}
	}
	mainName, current := "", ""
	for _, b := range branches {
		if b.Name == "main" {
			mainName = b.Name
		}
		if b.Current {
			current = b.Name
		}
	}
	if mainName == "" {
		mainName = current
	}
	walk := func(b project.BranchInfo) {
		for id := b.Head; id != "" && byID[id] != nil; {
			m := byID[id]
			switch {
			case m.Branch == b.Name: // (counted already)
			case m.Branch != "" || claimed[id]:
				return // another branch's
			default:
				claimed[id] = true
				counts[b.Name]++
			}
			ps := m.Parents
			if id = ""; len(ps) > 0 {
				id = ps[0]
			}
		}
	}
	for _, first := range []string{mainName, current} {
		for _, b := range branches {
			if b.Name == first {
				walk(b)
			}
		}
	}
	for _, b := range branches {
		if b.Name != mainName && b.Name != current {
			walk(b)
		}
	}
	return counts
}

// The counts per project, kept while its branches' latest versions stay the
// same (the settings ask again on every visit).
var branchCountCache = struct {
	sync.Mutex
	byRoot map[string]branchCounts
}{byRoot: map[string]branchCounts{}}

type branchCounts struct {
	heads  string
	counts map[string]int
}

// branchVersionCountsOf is branchVersionCounts for project r's branches,
// reading the versions on this computer (those not here aren't counted).
func branchVersionCountsOf(r *project.Repo, branches []project.BranchInfo) map[string]int {
	tips := []string{r.Head(), r.Latest()}
	for _, b := range branches {
		tips = append(tips, b.Name+"="+b.Head)
	}
	sort.Strings(tips)
	heads := strings.Join(tips, " ")
	branchCountCache.Lock()
	c, ok := branchCountCache.byRoot[r.Root]
	branchCountCache.Unlock()
	if ok && c.heads == heads {
		return c.counts
	}
	byID := map[string]*project.Manifest{}
	stack := []string{r.Head(), r.Latest()}
	for _, b := range branches {
		stack = append(stack, b.Head)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == "" || byID[id] != nil {
			continue
		}
		m, err := r.Header(id)
		if err != nil {
			continue // (not on this computer)
		}
		byID[id] = m
		stack = append(stack, m.Parents...)
	}
	counts := branchVersionCounts(byID, branches)
	branchCountCache.Lock()
	branchCountCache.byRoot[r.Root] = branchCounts{heads: heads, counts: counts}
	branchCountCache.Unlock()
	return counts
}
