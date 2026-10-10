package project

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Archiving a branch and getting it back (docs/design/branch-tree.md).
// Archiving takes only the branch's key away (a conditional delete: only if
// it hasn't moved since read); its versions stay in the team's storage for
// good (storage cleanup never deletes a version), its record keeps its
// name and colour (and where it was: `deleted`, its first name), and the
// branch log keeps where it was. Unarchiving puts the key there again (only
// if no branch has it meanwhile). Only an archived branch can be deleted:
// its record says so, and it is listed no more.

// ErrMainBranch: the main branch can't be archived.
var ErrMainBranch = errors.New("the main branch can't be archived")

// ErrOnBranch: the branch you are on can't be archived.
var ErrOnBranch = errors.New("you're on this branch: switch to another one first")

// OnlyOnBranch counts the versions of branch key no other branch has (what
// archiving leaves on no branch; they stay recoverable).
func (r *Repo) OnlyOnBranch(key string) (int, error) {
	c, err := r.Client()
	if err != nil {
		return 0, err
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil {
		return 0, err
	}
	head, ok := heads[key]
	if !ok {
		return 0, fmt.Errorf("no branch %q on the team", key)
	}
	for _, h := range heads {
		if err := r.fetchSnapshots(c, h); err != nil {
			return 0, err
		}
	}
	mine, err := r.ancestors(head)
	if err != nil {
		return 0, err
	}
	for k, h := range heads {
		if k == key {
			continue
		}
		theirs, err := r.ancestors(h)
		if err != nil {
			return 0, err
		}
		for id := range theirs {
			delete(mine, id)
		}
	}
	return len(mine), nil
}

// ArchiveBranch archives branch key (not main, not the one you are on).
func (r *Repo) ArchiveBranch(key string) error {
	if key == "main" {
		return ErrMainBranch
	}
	if key == r.BranchName() {
		return ErrOnBranch
	}
	c, err := r.Client()
	if err != nil {
		return err
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil {
		return err
	}
	head := heads[key]
	if head == "" {
		return fmt.Errorf("no branch %q on the team", key)
	}
	// Where it is, kept first: stopped before the branch log is written, it
	// still comes back.
	store, records := remote.BranchRecordsOf(c)
	var rec remote.BranchRecord
	if records {
		recs, err := store.BranchRecords(r.Config.ProjectID)
		if err != nil {
			return err
		}
		rec = recs[key]
		if rec.Name == "" {
			rec.Name = key
		}
		rec.Deleted = &remote.BranchDeleted{Head: head, By: remote.ActorOf(c), Time: time.Now().UTC()}
		if err := store.PutBranchRecord(r.Config.ProjectID, key, rec); err != nil {
			return err
		}
	}
	err = c.UpdateBranch(r.Config.ProjectID, key, head, "")
	if err != nil && records {
		rec.Deleted = nil
		store.PutBranchRecord(r.Config.ProjectID, key, rec) // it's there: as it was
	}
	var conflict *remote.ErrConflict
	if errors.As(err, &conflict) {
		return errors.New("someone shared on this branch just now: look at it again before archiving it")
	}
	return err
}

// ArchivedBranch is a branch archived, as it was then.
type ArchivedBranch struct {
	Key  string
	Head string // its latest version when archived
	By   string // member id ("" unknown)
	Time time.Time
}

// ArchivedBranches lists the branches archived and not there again (nor
// deleted), the last archived first.
func (r *Repo) ArchivedBranches() ([]ArchivedBranch, error) {
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	logger, ok := c.(remote.BranchLogger)
	if !ok {
		return nil, nil
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil {
		return nil, err
	}
	moves, err := logger.BranchLog(r.Config.ProjectID, "")
	if err != nil {
		return nil, err
	}
	last := map[string]remote.BranchMove{} // each branch's last move
	for _, m := range moves {
		last[m.Branch] = m
	}
	found := map[string]ArchivedBranch{}
	for key, m := range last {
		if m.To == "" && m.From != "" {
			found[key] = ArchivedBranch{Key: key, Head: m.From, By: m.By, Time: m.Time}
		}
	}
	// An archiving the branch log missed (stopped before it was written).
	removed := map[string]bool{} // (deleted for good)
	if store, ok := remote.BranchRecordsOf(c); ok {
		recs, err := store.BranchRecords(r.Config.ProjectID)
		if err != nil {
			return nil, err
		}
		for key, rec := range recs {
			removed[key] = rec.Removed
			if d := rec.Deleted; d != nil && d.Head != "" && !found[key].Time.After(d.Time) {
				found[key] = ArchivedBranch{Key: key, Head: d.Head, By: d.By, Time: d.Time}
			}
		}
	}
	var out []ArchivedBranch
	for key, d := range found {
		if _, there := heads[key]; !there && key != "main" && !removed[key] {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// UnarchiveBranch puts archived branch key back where it was.
func (r *Repo) UnarchiveBranch(key string) error {
	archived, err := r.ArchivedBranches()
	if err != nil {
		return err
	}
	for _, d := range archived {
		if d.Key != key {
			continue
		}
		c, err := r.Client()
		if err != nil {
			return err
		}
		err = c.UpdateBranch(r.Config.ProjectID, key, "", d.Head)
		var conflict *remote.ErrConflict
		if errors.As(err, &conflict) {
			return errors.New("a branch with this name was made again meanwhile")
		}
		if err != nil {
			return err
		}
		if store, ok := remote.BranchRecordsOf(c); ok {
			if recs, err := store.BranchRecords(r.Config.ProjectID); err == nil && recs[key].Deleted != nil {
				rec := recs[key]
				rec.Deleted = nil
				store.PutBranchRecord(r.Config.ProjectID, key, rec) // (listed by its log otherwise: only while not there)
			}
		}
		return nil
	}
	return fmt.Errorf("no archived branch %q", key)
}

// ArchiveBranches archives branches (a branch and those made from it), the
// branches made from them first; keep: branches made from one archived
// that stay, each with the branch it now comes from (their records say so).
func (r *Repo) ArchiveBranches(keys []string, keep map[string]string) error {
	for _, key := range keys {
		if key == "main" {
			return ErrMainBranch
		}
		if key == r.BranchName() {
			return ErrOnBranch
		}
	}
	if len(keep) > 0 {
		c, err := r.Client()
		if err != nil {
			return err
		}
		if store, ok := remote.BranchRecordsOf(c); ok {
			recs, err := store.BranchRecords(r.Config.ProjectID)
			if err != nil {
				return err
			}
			for key, parent := range keep {
				rec := recs[key]
				if rec.Name == "" {
					rec.Name = key
				}
				rec.Parent = parent
				if err := store.PutBranchRecord(r.Config.ProjectID, key, rec); err != nil {
					return err
				}
			}
		}
	}
	// (given parents first: the last archived, the first listed, unarchived first)
	for i := len(keys) - 1; i >= 0; i-- {
		if err := r.ArchiveBranch(keys[i]); err != nil {
			return fmt.Errorf("%s: %w", keys[i], err)
		}
	}
	return nil
}

// DeleteBranch deletes archived branch key for good: listed no more, its
// name free again. Its versions stay (in history where other branches have
// them); the branch log keeps what it did.
func (r *Repo) DeleteBranch(key string) error {
	archived, err := r.ArchivedBranches()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(archived, func(d ArchivedBranch) bool { return d.Key == key }) {
		return fmt.Errorf("only an archived branch can be deleted (%q isn't)", key)
	}
	c, err := r.Client()
	if err != nil {
		return err
	}
	store, ok := remote.BranchRecordsOf(c)
	if !ok {
		return errors.New("this team keeps no branch records: archived branches stay listed")
	}
	recs, err := store.BranchRecords(r.Config.ProjectID)
	if err != nil {
		return err
	}
	rec := recs[key]
	if rec.Name == "" {
		rec.Name = key
	}
	rec.Removed = true
	return store.PutBranchRecord(r.Config.ProjectID, key, rec)
}
