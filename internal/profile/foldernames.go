package profile

import (
	"sort"
	"strings"
)

// IgnoredFolderNames lists, in lower case, the names of folders the rules
// leave out by name ("target", "build", "library"): a new folder named like
// one ("target-dx12", "Library.bak") is likely the same kind of output, made
// where no rule looks. Only literal names count, from the project's rules,
// the presets in use and the .gitignore files read so far.
func (p *Profile) IgnoredFolderNames() []string {
	seen := map[string]bool{}
	add := func(pattern string, dirOnly bool) {
		pattern = strings.TrimSpace(pattern)
		if dirOnly && !strings.HasSuffix(pattern, "/") {
			return
		}
		segs := strings.Split(strings.Trim(pattern, "/"), "/")
		name := strings.ToLower(segs[len(segs)-1])
		if len(name) < 3 || strings.ContainsAny(name, `*?[\`) {
			return // too short to tell anything by, or a pattern
		}
		seen[name] = true
	}
	for _, r := range p.Rules {
		if r.Ignore != "" {
			add(r.Ignore, true)
		}
	}
	for _, a := range p.applied {
		if a.preset != nil {
			for _, pat := range a.preset.Ignore {
				add(pat, true)
			}
		}
	}
	if p.Gitignore {
		p.gitignoreOf("") // (the project folder's, even before a scan)
		p.gi.mu.Lock()
		for _, rules := range p.gi.byDir {
			for _, r := range rules {
				if !r.negate {
					add(r.pattern, false) // git's "/target" is a folder too
				}
			}
		}
		p.gi.mu.Unlock()
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
