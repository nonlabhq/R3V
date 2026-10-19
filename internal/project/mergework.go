package project

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/remote"
)

// TouchedChangesError: a merge would write or delete files that have
// changes not committed (Paths). It is ErrDirty, saying which.
type TouchedChangesError struct {
	Paths []string
}

func (e *TouchedChangesError) Error() string {
	list := e.Paths
	more := ""
	if len(list) > 10 {
		list, more = list[:10], fmt.Sprintf(" and %d more", len(e.Paths)-10)
	}
	return fmt.Sprintf("%v: the merge changes %s%s; commit or discard them first", ErrDirty, strings.Join(list, ", "), more)
}

func (e *TouchedChangesError) Unwrap() error { return ErrDirty }

// mergeKeepingWork merges target into HEAD while the working files have
// changes not committed (as git does): allowed when what comes in writes
// or deletes none of the files changed (changed, added, deleted, moved),
// refused naming them otherwise (TouchedChangesError). The merge version
// is HEAD's and target's files only. The changes are kept first as a
// version of their own, never shared (as updateKeepingWork keeps them),
// the merge's files are written around them, and they stay uncommitted.
//
// Stopped half-way (switching says "work <id>"), RecoverSwitch puts the
// work back on HEAD, which moves only once the files are in place: the
// project as it was, the changes with it; and ChangedSinceSwitch knows
// them (they are in the work's version), so they don't count as changed
// since.
func (r *Repo) mergeKeepingWork(c remote.Backend, head, target string, opts MergeOptions, message string,
	changes []Change, res *SyncResult) (*SyncResult, error) {
	dirty := map[string]bool{}
	for _, ch := range changes {
		dirty[ch.Path] = true
		if ch.From != "" {
			dirty[ch.From] = true
		}
	}
	base, err := r.Load(head)
	if err != nil {
		return nil, err
	}
	theirs, err := r.Load(target)
	if err != nil {
		return nil, err
	}
	// Before any work (and before asking about conflicts): what the other
	// side changed since the two parted, a little more than the merge will
	// write, refuses at once.
	parted := &Manifest{}
	if id, err := r.mergeBase(head, target); err != nil {
		return nil, err
	} else if id != "" {
		if parted, err = r.Load(id); err != nil {
			return nil, err
		}
	}
	if err := touchedChanges(dirty, parted, theirs); err != nil {
		return nil, err
	}
	merged, log, err := r.mergeWith(c, head, target, opts, message)
	if err != nil {
		return nil, err
	}
	// Exactly what the merge writes or deletes (a file you moved that they
	// changed is written where you put it).
	if err := touchedChanges(dirty, base, merged); err != nil {
		os.Remove(r.snapshotPath(merged.ID))
		return nil, err
	}

	// The work, whatever a commit picks (Repo.Only): the files are written
	// from it.
	only := r.Only
	r.Only = nil
	work, ix, err := r.workingManifest("Uncommitted work, kept while merging")
	r.Only = only
	if err != nil {
		os.Remove(r.snapshotPath(merged.ID))
		return nil, err
	}
	if err := r.save(work); err != nil {
		return nil, err
	}
	if err := ix.save(); err != nil {
		return nil, err
	}
	files, _, err := r.mergeManifests(base, work, merged, opts)
	if err != nil { // (none: they touch different files)
		os.Remove(r.snapshotPath(work.ID))
		os.Remove(r.snapshotPath(merged.ID))
		var mc *MergeConflictError
		if errors.As(err, &mc) {
			mc.Work = true
		}
		return nil, err
	}
	r.knowSizes(files)
	if err := r.fetchObjects(c, files.Objects()); err != nil {
		return nil, err
	}
	notes, err := r.putFiles(files, merged.ID, "work "+work.ID)
	if err != nil {
		return nil, err
	}
	// The files now are kept too, as updateKeepingWork keeps them: until
	// committed, the working files stand for their contents.
	files.Parents, files.Message = []string{merged.ID}, "Uncommitted work with a merge around it"
	if err := r.save(files); err != nil {
		return nil, err
	}
	r.keepWork(work.ID, files.ID)
	res.Action, res.To, res.MergeLog, res.Relinked, res.KeptWork = "merged", merged.ID, log, notes, true
	return res, nil
}

// touchedChanges fails (TouchedChangesError) when going from a to b writes
// or deletes a path in dirty. A change of the rules (.r3v.yaml) changes what
// is tracked at all: it touches every change.
func touchedChanges(dirty map[string]bool, a, b *Manifest) error {
	af, bf := a.FileMap(), b.FileMap()
	rules := af[profile.FileName].Hash != bf[profile.FileName].Hash
	var hit []string
	for p := range dirty {
		if rules || af[p].Hash != bf[p].Hash {
			hit = append(hit, p)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	sort.Strings(hit)
	return &TouchedChangesError{Paths: hit}
}
