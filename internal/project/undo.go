package project

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Undo commit: a new version that takes back what one version changed,
// keeping everything done after it (git's revert). It is a 3-way merge
// with the version as base, the latest as ours and the version before it as
// theirs: Live sets go track by track, and where a later version changed
// the same thing, that is a conflict to decide (MergeConflictError).
//
// Uncommitted changes stay as they are, unless they touch a file the undo
// changes too: then the undo stops (ErrUndoTouchesChanges) and asks to
// commit or discard those first.

// ErrUndoTouchesChanges: uncommitted changes are in files the undo would
// change.
type ErrUndoTouchesChanges struct{ Paths []string }

func (e *ErrUndoTouchesChanges) Error() string {
	return "you have uncommitted changes in files this undo changes: commit or discard them first (" +
		strings.Join(e.Paths, ", ") + ")"
}

// ErrNothingToUndo: the latest version already is as before that version.
var ErrNothingToUndo = errors.New("nothing to undo: what that version changed is already gone")

// UndoPlan is what undoing a version does to the latest one: the files it
// changes (added, modified or deleted) and the uncommitted changes in the
// way, if any.
type UndoPlan struct {
	Changed []string
	Blocked []string // uncommitted changes in files the undo changes
	Log     []string // how sets were merged
}

// undoTarget computes the version after the undo: the latest with id's
// changes taken back.
func (r *Repo) undoTarget(c remote.Backend, id string, opts MergeOptions) (head, undone *Manifest, log []string, err error) {
	if r.OnOlderVersion() {
		return nil, nil, nil, errors.New("go back to the latest version first")
	}
	if r.Head() == "" {
		return nil, nil, nil, errors.New("nothing committed yet")
	}
	if ok, err := r.isAncestor(id, r.Head()); err != nil || !ok {
		return nil, nil, nil, errors.New("that version isn't in the history of the version you're on")
	}
	bad, err := r.Load(id)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(bad.Parents) == 0 {
		return nil, nil, nil, errors.New("the first version can't be undone")
	}
	before, err := r.Load(bad.Parents[0])
	if err != nil {
		return nil, nil, nil, err
	}
	if head, err = r.Load(r.Head()); err != nil {
		return nil, nil, nil, err
	}
	if c != nil {
		r.knowSizes(bad, before, head)
		if err := r.fetchObjects(c, setHashes(bad, before, head)); err != nil {
			return nil, nil, nil, err
		}
	}
	undone, log, err = r.mergeManifests(bad, head, before, opts)
	if err != nil {
		return nil, nil, nil, err
	}
	return head, undone, log, nil
}

// changedPaths lists the paths whose file differs between a and b.
func changedPaths(a, b *Manifest) []string {
	af, bf := a.FileMap(), b.FileMap()
	var out []string
	for p, f := range af {
		if g, ok := bf[p]; !ok || g.Hash != f.Hash {
			out = append(out, p)
		}
	}
	for p := range bf {
		if _, ok := af[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// PlanUndo says what undoing version id would change, and what's in the
// way; a conflict comes back as *MergeConflictError.
func (r *Repo) PlanUndo(id string, opts MergeOptions) (*UndoPlan, error) {
	c, _ := r.Client() // a team's version files may only be in its storage
	head, undone, log, err := r.undoTarget(c, id, opts)
	if err != nil {
		return nil, err
	}
	plan := &UndoPlan{Changed: changedPaths(head, undone), Log: log}
	if len(plan.Changed) == 0 {
		return nil, ErrNothingToUndo
	}
	plan.Blocked, err = r.undoBlocked(head, plan.Changed)
	return plan, err
}

// undoBlocked: the uncommitted changes in paths.
func (r *Repo) undoBlocked(head *Manifest, paths []string) ([]string, error) {
	only := r.Only
	r.Only = nil
	work, _, err := r.workingManifest("")
	r.Only = only
	if errors.Is(err, ErrNothingToSnapshot) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pending := map[string]bool{}
	for _, p := range changedPaths(head, work) {
		pending[p] = true
	}
	var blocked []string
	for _, p := range paths {
		if pending[p] {
			blocked = append(blocked, p)
		}
	}
	return blocked, nil
}

// UndoCommit makes a version that takes back version id's changes and
// shares it (with a team): what Save returns. Uncommitted changes in other
// files stay uncommitted.
func (r *Repo) UndoCommit(id, message string, opts MergeOptions) (*Manifest, *SyncResult, error) {
	c, _ := r.Client()
	head, undone, _, err := r.undoTarget(c, id, opts)
	if err != nil {
		return nil, nil, err
	}
	paths := changedPaths(head, undone)
	if len(paths) == 0 {
		return nil, nil, ErrNothingToUndo
	}
	// The work as it is now: the undo goes into it, file by file.
	only := r.Only
	r.Only = nil
	work, ix, err := r.workingManifest("Uncommitted work, kept while undoing a version")
	r.Only = only
	pendingAt := map[string]bool{}
	switch {
	case errors.Is(err, ErrNothingToSnapshot):
		work = head
	case err != nil:
		return nil, nil, err
	default:
		for _, p := range changedPaths(head, work) {
			pendingAt[p] = true
		}
		var blocked []string
		for _, p := range paths {
			if pendingAt[p] {
				blocked = append(blocked, p)
			}
		}
		if len(blocked) > 0 {
			return nil, nil, &ErrUndoTouchesChanges{Paths: blocked}
		}
		if err := r.save(work); err != nil {
			return nil, nil, err
		}
		if err := ix.save(); err != nil {
			return nil, nil, err
		}
	}
	target := *work
	files := map[string]FileEntry{}
	for _, f := range work.Files {
		files[f.Path] = f
	}
	want := undone.FileMap()
	for _, p := range paths {
		if f, ok := want[p]; ok {
			files[p] = f
		} else {
			delete(files, p)
		}
	}
	target.Files = make([]FileEntry, 0, len(files))
	for _, f := range files {
		target.Files = append(target.Files, f)
	}
	sort.Slice(target.Files, func(i, j int) bool { return target.Files[i].Path < target.Files[j].Path })
	target.Tree = "" // listed in Files; folders are made when it's saved
	if c != nil {
		r.knowSizes(&target)
		if err := r.fetchObjects(c, target.Objects()); err != nil {
			return nil, nil, err
		}
	}
	// Stopped halfway, the files go back as they were (the work kept).
	if _, err := r.putFiles(&target, r.Head(), "work "+work.ID); err != nil {
		return nil, nil, err
	}
	if work != head {
		defer os.Remove(r.snapshotPath(work.ID))
	}
	// Committed: the files the undo changed, nothing else.
	r.Only = paths
	defer func() { r.Only = only }()
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("Undo %s", short(id))
	}
	if c == nil {
		m, err := r.Snapshot(message)
		return m, nil, err
	}
	return r.Save(message, opts)
}

// ErrNotAMerge: UndoMerge was given a version that isn't a merge.
var ErrNotAMerge = errors.New("that version isn't a merge (undo it as a commit instead)")

// UndoMerge makes a version that takes back what merge version ref brought
// into its branch (what it changed compared with its first parent), keeping
// the history as it is; what was changed after it is merged as UndoCommit
// does (a conflict comes back as *MergeConflictError). Uncommitted changes
// stop it, unless they are this undo left half-way (R3V stopped after
// putting its files in place): those are committed. Committed and not
// shared (stopped while sharing): shared.
func (r *Repo) UndoMerge(ref, message string, opts MergeOptions) (*Manifest, *SyncResult, error) {
	id, err := r.Resolve(ref)
	if err != nil {
		return nil, nil, err
	}
	m, err := r.Header(id)
	if err != nil {
		return nil, nil, err
	}
	if len(m.Parents) < 2 {
		return nil, nil, ErrNotAMerge
	}
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("Undo merge %s (%q)", short(id), manifest.Title(m.Message))
	}
	c, _ := r.Client()
	changes, err := r.Status()
	if err != nil {
		return nil, nil, err
	}
	if len(changes) > 0 {
		paths, ok, err := r.undoLeftHalfway(c, id, opts)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("%w (%d file(s)); commit or discard them first", ErrDirty, len(changes))
		}
		only := r.Only
		r.Only = paths
		defer func() { r.Only = only }()
		if c == nil {
			s, err := r.Snapshot(message)
			return s, nil, err
		}
		return r.Save(message, opts)
	}
	s, res, err := r.UndoCommit(id, message, opts)
	if errors.Is(err, ErrNothingToUndo) && c != nil {
		if shared, serr := r.shared(c); serr == nil && !shared {
			res, err := r.Share(opts)
			return nil, res, err
		}
	}
	return s, res, err
}

// undoLeftHalfway reports whether the uncommitted changes are exactly
// undoing merge id (its files put in place, not committed), and the paths.
func (r *Repo) undoLeftHalfway(c remote.Backend, id string, opts MergeOptions) ([]string, bool, error) {
	head, undone, _, err := r.undoTarget(c, id, opts)
	var mc *MergeConflictError
	if errors.As(err, &mc) || errors.Is(err, ErrNothingToUndo) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	only := r.Only
	r.Only = nil
	work, _, err := r.workingManifest("")
	r.Only = only
	if err != nil {
		return nil, false, err
	}
	paths := changedPaths(head, undone)
	if len(paths) == 0 || len(changedPaths(work, undone)) > 0 {
		return nil, false, nil
	}
	return paths, true, nil
}
