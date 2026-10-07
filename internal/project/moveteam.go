package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	"github.com/nonlabhq/r3v/internal/blob"
	"github.com/nonlabhq/r3v/internal/chunk"
	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Moving a project to another team (docs/design/moving.md). Its whole
// history goes from one team's storage to the other's, straight from one to
// the other (nothing kept on this computer on the way), whatever each keeps
// its contents like (one pool for a team's projects, or each project's
// apart on a hosted team):
//
//  1. its versions here (they're small): every branch's, every deleted
//     branch's and milestone's, with their ancestors;
//  2. the files they use (folder lists, files, the pieces of big files: the
//     pieces before their list), only those the other team lacks;
//  3. the versions, then the branches' names and colours, the milestones;
//  4. the branches;
//  5. the project's record: only now does the other team list it;
//  6. moving (not copying): the first team lets it go, and this folder
//     belongs to the other team.
//
// Stopped anywhere before 5, the other team has files and versions it
// doesn't show (its cleanup takes them in time), and moving again goes on
// from there. Stopped after 5, both teams have it: moving again finishes.

// MoveProjects: this build moves projects between teams (the Nightly
// channel, while it's new: move_nightly.go).
var MoveProjects = false

// ErrMoveNotInBuild: this build doesn't move projects (Stable).
var ErrMoveNotInBuild = errors.New("moving a project to another team needs R3V's Nightly channel")

// ErrMoveUnshared: versions here the team doesn't have yet.
var ErrMoveUnshared = errors.New("share your versions first: a project moves with what its team has")

// MoveOptions: Copy keeps the project in its team too (the other gets a copy).
type MoveOptions struct {
	Copy bool
}

// MoveToTeam moves (or copies) the project to team t.
func (r *Repo) MoveToTeam(t *teams.Team, opts MoveOptions) error {
	if !MoveProjects {
		return ErrMoveNotInBuild
	}
	if r.Config.Remote == nil {
		return errors.New("the project isn't a team's: add it to the team instead")
	}
	from, err := r.Team()
	if err != nil {
		return err
	}
	if from.ID == t.ID {
		return errors.New("the project is in that team already")
	}
	c, err := r.Client()
	if err != nil {
		return err
	}
	if ok, err := r.shared(c); err != nil {
		return err
	} else if !ok {
		return ErrMoveUnshared
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	pid := r.Config.ProjectID
	d := remote.ForProject(b, pid)

	// Copied until the team's branches stay as they were while copying
	// (someone may share meanwhile).
	var copied map[string]string
	for range 3 {
		heads, err := c.Branches(pid)
		if err != nil {
			return err
		}
		if sameHeads(heads, copied) {
			break
		}
		if err := r.copyTo(c, d, heads, copied); err != nil {
			return err
		}
		copied = heads
	}

	// Its record last: from now on the other team lists it.
	ps, err := c.Projects()
	if err != nil {
		return err
	}
	rec := remote.Project{ID: pid, Name: r.Config.Name}
	if i := slices.IndexFunc(ps, func(p remote.Project) bool { return p.ID == pid }); i >= 0 {
		rec = ps[i]
	}
	if err := d.PutProject(rec); err != nil {
		return err
	}
	if opts.Copy {
		return nil
	}
	if err := c.DeleteProject(pid); err != nil {
		return fmt.Errorf("the other team has it, but this one couldn't let it go (move it again): %w", err)
	}
	if _, err := teams.Update(func(s *teams.Store) error {
		s.ForgetProject(from.ID, pid)
		return nil
	}); err != nil {
		return err
	}
	return r.JoinTeam(t)
}

func sameHeads(a, b map[string]string) bool {
	if b == nil || len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// copyTo puts on d what the branches heads need (steps 1-4), d's branches
// moving from before (nil the first time) to heads.
func (r *Repo) copyTo(c, d remote.Backend, heads, before map[string]string) error {
	pid := r.Config.ProjectID
	tips := map[string]bool{}
	for _, h := range heads {
		tips[h] = true
	}
	if gone, err := r.DeletedBranches(); err == nil {
		for _, g := range gone {
			tips[g.Head] = true
		}
	}
	marks, _ := r.Milestones()
	for _, m := range marks {
		tips[m.Version] = true
	}
	// 1. The versions, here.
	versions := map[string]bool{}
	for h := range tips {
		if h == "" {
			continue
		}
		if err := r.fetchSnapshots(c, h); err != nil {
			return err
		}
		anc, err := r.ancestors(h)
		if err != nil {
			return err
		}
		for v := range anc {
			versions[v] = true
		}
	}
	// 2. The files they use, the other team lacks.
	var ids, objects, roots []string
	for v := range versions {
		ids = append(ids, v)
		m, err := r.Load(v)
		if err != nil {
			return err
		}
		objects = append(objects, m.Objects()...)
		if m.Tree != "" {
			roots = append(roots, m.Tree)
		}
	}
	if err := r.walkTrees(roots, func(h string, _ []manifest.TreeEntry) error {
		objects = append(objects, h)
		return nil
	}); err != nil {
		return err
	}
	objects = dedupe(objects)
	if l, ok := d.(remote.Leaser); ok && len(objects) > 0 {
		release, err := l.Lease(objects)
		if err != nil {
			return err
		}
		defer release()
	}
	missing, err := d.MissingObjects(objects)
	if err != nil {
		return err
	}
	if p, ok := c.(remote.Preparer); ok {
		p.PrepareObjects(missing)
	}
	t := r.newTransfer(StageUploading, len(missing), 0)
	t.report()
	if err := inParallel(missing, func(h string) error {
		if err := r.stopped(); err != nil {
			return err
		}
		if err := copyObject(c, d, h); err != nil {
			return err
		}
		t.fileDone()
		return nil
	}); err != nil {
		return err
	}
	// 3. The versions, their branches' names and colours, the milestones.
	r.report(StageFinishing, 0, 2)
	need, err := d.MissingSnapshots(pid, ids)
	if err != nil {
		return err
	}
	if err := inParallel(need, func(id string) error {
		data, err := os.ReadFile(r.snapshotPath(id))
		if err != nil {
			return err
		}
		return d.PutSnapshot(pid, id, data)
	}); err != nil {
		return err
	}
	if from, ok := remote.BranchRecordsOf(c); ok {
		if to, ok := remote.BranchRecordsOf(d); ok {
			recs, err := from.BranchRecords(pid)
			if err != nil {
				return err
			}
			for key, rec := range recs {
				if err := to.PutBranchRecord(pid, key, rec); err != nil {
					return err
				}
			}
		}
	}
	if to, ok := remote.MilestonesOf(d); ok {
		if from, ok := remote.MilestonesOf(c); ok {
			all, err := from.Milestones(pid)
			if err != nil {
				return err
			}
			for id, m := range all {
				if err := to.PutMilestone(pid, id, m); err != nil {
					return err
				}
			}
		}
	}
	r.report(StageFinishing, 1, 2)
	// 4. The branches (one gone meanwhile is left: the team moving it said so).
	for key, h := range heads {
		err := d.UpdateBranch(pid, key, before[key], h)
		var conflict *remote.ErrConflict
		if errors.As(err, &conflict) && conflict.Current == h {
			err = nil // there already (a move done again)
		}
		if err != nil {
			return fmt.Errorf("branch %s: %w", key, err)
		}
	}
	r.report(StageFinishing, 2, 2)
	return nil
}

// copyObject copies file content h from c to d as it is stored; a big
// file's pieces first (those d lacks), then its list.
func copyObject(c, d remote.Backend, h string) error {
	rc, err := c.GetObject(h)
	if err != nil {
		return fmt.Errorf("%s: %w", short(h), err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return err
	}
	bs, ok := d.(remote.BodyStore)
	if !ok {
		return errors.New("the other team's storage can't take files as they are stored")
	}
	if blob.IsChunkList(data) {
		lr, _, err := blob.Open(bytes.NewReader(data))
		if err != nil {
			return err
		}
		text, err := io.ReadAll(lr)
		lr.Close()
		if err != nil {
			return err
		}
		l, err := chunk.Parse(text)
		if err != nil {
			return err
		}
		pieces, err := d.MissingObjects(dedupe(l.Hashes()))
		if err != nil {
			return err
		}
		var mu sync.Mutex
		var first error
		inParallelN(pieceTransfers, pieces, func(p string) error {
			if err := copyObject(c, d, p); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
				return err
			}
			return nil
		})
		if first != nil {
			return first
		}
		if err := bs.MarkChunked(h); err != nil {
			return err
		}
	}
	sum := sha256.Sum256(data)
	return bs.PutObjectBody(h, bytes.NewReader(data), int64(len(data)), hex.EncodeToString(sum[:]))
}
