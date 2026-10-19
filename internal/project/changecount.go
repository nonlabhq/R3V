package project

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/store"
)

// Change counts (the history graph's "47 versions · 120 files"): how many
// files a version changed against its first parent, as VersionChanges
// lists them, except that moves are paired by content only (a file moved
// and edited counts twice: pairing those reads the files). Counting reads
// both versions' trees, so each count is worked out once and kept in
// .r3v/changed.json: versions never change, so neither do their counts.

const changedFile = "changed.json"

// counts keeps each project's counts once read or worked out, by the
// store's folder (workspaces share it).
var counts = struct {
	sync.Mutex
	by map[string]*projectCounts
}{by: map[string]*projectCounts{}}

type projectCounts struct {
	m       map[string]int // by version id
	unsaved bool           // some aren't in the file yet
}

func (r *Repo) changedPath() string { return filepath.Join(r.CommonDir(), changedFile) }

func readCounts(path string) map[string]int {
	m := map[string]int{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &m) // (a damaged file: counted again)
	}
	return m
}

// countClock is ChangeCounts' clock: a test's moves on with each reading
// (Windows' clock may not, over a few quick counts).
var countClock = time.Now

// ChangeCounts are the change counts of the versions ids, those not known
// yet worked out in order (give the newest first: the graph shows them
// first) for at most budget (none: all of them). Versions left out, or
// whose records or trees aren't here, are counted by a later call. The
// counts are kept for good, but not by a read-only open.
func (r *Repo) ChangeCounts(ids []string, budget time.Duration) map[string]int {
	counts.Lock()
	defer counts.Unlock()
	pc := counts.by[r.CommonDir()]
	if pc == nil {
		pc = &projectCounts{m: readCounts(r.changedPath())}
		counts.by[r.CommonDir()] = pc
	}
	start := countClock()
	out := make(map[string]int, len(ids))
	for _, id := range ids {
		if n, ok := pc.m[id]; ok {
			out[id] = n
			continue
		}
		if budget > 0 && countClock().Sub(start) > budget {
			continue
		}
		n, err := r.countChanged(id)
		if err != nil {
			continue
		}
		pc.m[id], out[id], pc.unsaved = n, n, true
	}
	if pc.unsaved && !r.ReadOnly {
		// (with what another program kept meanwhile)
		all := readCounts(r.changedPath())
		for id, n := range pc.m {
			all[id] = n
		}
		if data, err := json.Marshal(all); err == nil &&
			store.WriteAtomic(r.changedPath(), bytes.NewReader(data)) == nil {
			pc.m, pc.unsaved = all, false
		}
	}
	return out
}

// countChanged works out how many files version id changed against its
// first parent (none: every file it has).
func (r *Repo) countChanged(id string) (int, error) {
	m, err := r.Header(id)
	if err != nil {
		return 0, err
	}
	var p *Manifest
	if len(m.Parents) > 0 {
		if p, err = r.Header(m.Parents[0]); err != nil {
			return 0, err
		}
	}
	d := &treeDiff{}
	if m.Tree != "" && (p == nil || p.Tree != "") {
		parent := ""
		if p != nil {
			parent = p.Tree
		}
		if err := r.diffTrees(parent, m.Tree, "", d); err != nil {
			return 0, err
		}
	} else { // format 1: the records list the files
		full, err := r.Load(id)
		if err != nil {
			return 0, err
		}
		before := &Manifest{}
		if p != nil {
			if before, err = r.Load(p.ID); err != nil {
				return 0, err
			}
		}
		bf := full.FileMap()
		for _, f := range before.Files {
			if g, ok := bf[f.Path]; !ok {
				d.gone = append(d.gone, f)
			} else if g.Hash != f.Hash {
				d.modified++
			}
		}
		af := before.FileMap()
		for _, f := range full.Files {
			if _, ok := af[f.Path]; !ok {
				d.added = append(d.added, f)
			}
		}
	}
	moved := findRenames(d.gone, d.added, func(FileEntry, bool) ([]byte, bool) { return nil, false })
	return d.modified + len(d.gone) + len(d.added) - len(moved), nil
}

type treeDiff struct {
	gone, added []FileEntry
	modified    int
}

// diffTrees compares two folders' trees ("": no folder), skipping every
// folder whose tree is the same on both sides.
func (r *Repo) diffTrees(a, b, prefix string, d *treeDiff) error {
	if a == b {
		return nil
	}
	var old, now []manifest.TreeEntry
	var err error
	if old, err = r.entries(a); err != nil {
		return err
	}
	if now, err = r.entries(b); err != nil {
		return err
	}
	before := make(map[string]manifest.TreeEntry, len(old))
	for _, e := range old {
		before[e.Name] = e
	}
	for _, e := range now {
		o, ok := before[e.Name]
		delete(before, e.Name)
		switch {
		case ok && o.Dir == e.Dir && o.Hash == e.Hash:
		case ok && o.Dir && e.Dir:
			err = r.diffTrees(o.Hash, e.Hash, prefix+e.Name+"/", d)
		case ok && !o.Dir && !e.Dir:
			d.modified++
		default:
			if ok {
				err = r.goneEntry(o, prefix, d)
			}
			if err == nil {
				err = r.addedEntry(e, prefix, d)
			}
		}
		if err != nil {
			return err
		}
	}
	for _, o := range before {
		if err := r.goneEntry(o, prefix, d); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) entries(h string) ([]manifest.TreeEntry, error) {
	if h == "" {
		return nil, nil
	}
	return r.tree(h)
}

func (r *Repo) goneEntry(e manifest.TreeEntry, prefix string, d *treeDiff) error {
	if e.Dir {
		return r.diffTrees(e.Hash, "", prefix+e.Name+"/", d)
	}
	d.gone = append(d.gone, FileEntry{Path: prefix + e.Name, Hash: e.Hash, Size: e.Size})
	return nil
}

func (r *Repo) addedEntry(e manifest.TreeEntry, prefix string, d *treeDiff) error {
	if e.Dir {
		return r.diffTrees("", e.Hash, prefix+e.Name+"/", d)
	}
	d.added = append(d.added, FileEntry{Path: prefix + e.Name, Hash: e.Hash, Size: e.Size})
	return nil
}
