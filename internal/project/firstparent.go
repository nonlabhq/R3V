package project

// FirstParents is the main line of log (newest first, as Log lists it):
// from its newest version back through first parents. merged says, for
// each merge on it, how many versions it brought in (those its other
// parents lead to that the line before it hadn't).
func FirstParents(log []*Manifest) (line []*Manifest, merged map[string]int) {
	merged = map[string]int{}
	if len(log) == 0 {
		return nil, merged
	}
	byID := map[string]*Manifest{}
	for _, m := range log {
		byID[m.ID] = m
	}
	for m := log[0]; m != nil; {
		line = append(line, m)
		if len(m.Parents) == 0 {
			break
		}
		m = byID[m.Parents[0]]
	}
	// Oldest first, keeping what the line has so far: each merge counts
	// only what is new to it.
	seen := map[string]bool{}
	walk := func(id string) int {
		n := 0
		for stack := []string{id}; len(stack) > 0; {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			m := byID[cur]
			if m == nil || seen[cur] {
				continue
			}
			seen[cur] = true
			n++
			stack = append(stack, m.Parents...)
		}
		return n
	}
	for i := len(line) - 1; i >= 0; i-- {
		m := line[i]
		seen[m.ID] = true
		if len(m.Parents) < 2 {
			continue
		}
		n := 0
		for _, p := range m.Parents[1:] {
			n += walk(p)
		}
		merged[m.ID] = n
	}
	return line, merged
}
