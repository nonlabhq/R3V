package project

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// A folder new in the changes may be a build's output the rules miss: the
// rules leave out target/ but the build wrote target-dx12/. Committed, it
// stays in the team's storage for good (ignoring it later only helps later
// versions), so the user, or the agent committing, hears of it first. A
// warning, never a refusal: the folder may be wanted.

// BuiltFolder is a folder new in the changes that looks like a build's
// output.
type BuiltFolder struct {
	Folder string // slash separated
	Reason string // why, for people
	Files  int    // its new files
	Bytes  int64
	Rule   string // an ignore pattern for .r3v.yaml that leaves it out
}

const (
	builtFilesAtLeast = 50        // files of a compiler's (builtExts)
	builtBytesAtLeast = 200 << 20 // or this much, whatever it is
)

// builtExts are what compilers and linkers write.
var builtExts = map[string]bool{".obj": true, ".o": true, ".pdb": true, ".rlib": true, ".rmeta": true, ".d": true,
	".lib": true, ".dll": true, ".exe": true, ".ilk": true, ".pch": true, ".a": true, ".so": true, ".dylib": true,
	".class": true, ".pyc": true}

// StatusChecked is Status with the folders new in the changes that look
// like a build's output.
func (r *Repo) StatusChecked() ([]Change, []BuiltFolder, error) {
	ix := r.loadIndex()
	files, err := r.workingFiles(ix)
	if err != nil {
		return nil, nil, err
	}
	if !r.ReadOnly {
		defer ix.save()
	}
	changes, head, err := r.compare(files)
	if err != nil {
		return nil, nil, err
	}
	var added []FileEntry
	if len(changes) > 0 {
		isNew := map[string]bool{}
		for _, c := range changes {
			if c.Status == "added" {
				isNew[c.Path] = true
			}
		}
		for _, f := range files {
			if isNew[f.Path] {
				added = append(added, f)
			}
		}
	}
	return changes, r.looksBuilt(added, head), nil
}

// BuiltFoldersIn: the folders version m added that look like a build's
// output (what a commit just took, against the version before it).
func (r *Repo) BuiltFoldersIn(m *Manifest) []BuiltFolder {
	var before map[string]FileEntry
	if len(m.Parents) > 0 {
		p, err := r.Load(m.Parents[0])
		if err != nil {
			return nil
		}
		before = p.FileMap()
	}
	var added []FileEntry
	for _, f := range m.Files {
		if _, ok := before[f.Path]; !ok {
			added = append(added, f)
		}
	}
	return r.looksBuilt(added, before)
}

func (r *Repo) looksBuilt(added []FileEntry, head map[string]FileEntry) []BuiltFolder {
	if len(added) == 0 {
		return nil
	}
	var names []string
	if p := r.rules(); p != nil {
		names = p.IgnoredFolderNames()
	}
	return looksBuilt(added, head, names)
}

type folderStat struct {
	files, built int
	bytes        int64
}

// looksBuilt finds, among the folders of added files that hold nothing of
// head, the topmost ones named like a folder the rules leave out (names,
// lower case) or holding many built files or bytes. A folder named so is
// found first; what it holds doesn't count for the folders above it again.
func looksBuilt(added []FileEntry, head map[string]FileEntry, names []string) []BuiltFolder {
	old := map[string]bool{} // folders holding files of head
	for p := range head {
		for d := path.Dir(p); d != "."; d = path.Dir(d) {
			if old[strings.ToLower(d)] {
				break
			}
			old[strings.ToLower(d)] = true
		}
	}
	// newDirs: a file's folders that are new, topmost first.
	newDirs := func(p string) []string {
		var ds []string
		for d := path.Dir(p); d != "." && !old[strings.ToLower(d)]; d = path.Dir(d) {
			ds = append(ds, d)
		}
		for i, j := 0, len(ds)-1; i < j; i, j = i+1, j-1 {
			ds[i], ds[j] = ds[j], ds[i]
		}
		return ds
	}
	var out []BuiltFolder
	named := map[string]*BuiltFolder{}
	stats := map[string]*folderStat{}
	for _, f := range added {
		ds := newDirs(f.Path)
		var in *BuiltFolder
		for _, d := range ds {
			if b := named[d]; b != nil {
				in = b
				break
			}
			if like, rule := namedLike(path.Base(d), names); like != "" {
				in = &BuiltFolder{Folder: d, Rule: rule,
					Reason: fmt.Sprintf("named like %s/, which the rules leave out", like)}
				named[d] = in
				break
			}
		}
		if in != nil {
			in.Files++
			in.Bytes += f.Size
			continue
		}
		built := builtExts[strings.ToLower(path.Ext(f.Path))]
		for _, d := range ds {
			s := stats[d]
			if s == nil {
				s = &folderStat{}
				stats[d] = s
			}
			s.files++
			s.bytes += f.Size
			if built {
				s.built++
			}
		}
	}
	for _, b := range named {
		out = append(out, *b)
	}
	// The rest: topmost first, so the folders inside a found one are
	// passed over.
	dirs := make([]string, 0, len(stats))
	for d := range stats {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		if a, b := strings.Count(dirs[i], "/"), strings.Count(dirs[j], "/"); a != b {
			return a < b
		}
		return dirs[i] < dirs[j]
	})
	found := map[string]bool{}
	for _, d := range dirs {
		if insideFound(d, found) {
			continue
		}
		s := stats[d]
		var why string
		switch {
		case s.built >= builtFilesAtLeast:
			why = fmt.Sprintf("holds %d built files (.obj, .pdb, .dll and the like)", s.built)
		case s.bytes >= builtBytesAtLeast:
			why = fmt.Sprintf("holds %d MB of new files", s.bytes>>20)
		default:
			continue
		}
		found[d] = true
		out = append(out, BuiltFolder{Folder: d, Reason: why, Files: s.files, Bytes: s.bytes, Rule: folderRule(d)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Folder < out[j].Folder })
	return out
}

func insideFound(d string, found map[string]bool) bool {
	for p := path.Dir(d); p != "."; p = path.Dir(p) {
		if found[p] {
			return true
		}
	}
	return false
}

// namedLike: name is an ignored folder's name and "-", "_" or "." and more
// ("target-dx12", "Library.bak"); like is that name as name has it, rule
// leaves out every folder named so ("target-*/").
func namedLike(name string, names []string) (like, rule string) {
	for _, n := range names {
		if len(name) > len(n)+1 && strings.EqualFold(name[:len(n)], n) && strings.ContainsRune("-_.", rune(name[len(n)])) {
			return name[:len(n)], globQuote(name[:len(n)+1]) + "*/"
		}
	}
	return "", ""
}

// folderRule leaves out that one folder ("/out/": anchored, not every
// folder of that name).
func folderRule(d string) string {
	rule := globQuote(d) + "/"
	if !strings.Contains(d, "/") {
		rule = "/" + rule
	}
	return rule
}

// globQuote makes a name match itself in a rule.
func globQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`*?[\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
