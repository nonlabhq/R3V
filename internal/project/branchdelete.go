package project

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Deleting a branch and getting it back. Deleting takes only the branch's
// key away (a conditional delete: only if it hasn't moved since read); its
// versions stay in the team's storage for good (storage cleanup never
// deletes a version), its record keeps its name and colour, and the branch
// log keeps where it was. Getting it back puts the key there again (only if
// no branch has it meanwhile).

// ErrMainBranch: the main branch can't be deleted.
var ErrMainBranch = errors.New("the main branch can't be deleted")

// ErrOnBranch: the branch you are on can't be deleted.
var ErrOnBranch = errors.New("you're on this branch: switch to another one first")

// OnlyOnBranch counts the versions of branch key no other branch has (what
// a delete would leave on no branch; they stay recoverable).
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

// DeleteBranch deletes branch key from the team (not main, not the one you
// are on).
func (r *Repo) DeleteBranch(key string) error {
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
		return errors.New("someone shared on this branch just now: look at it again before deleting it")
	}
	return err
}

// DeletedBranch is a branch deleted from the team, as it was then.
type DeletedBranch struct {
	Key  string
	Head string // its latest version when deleted
	By   string // member id ("" unknown)
	Time time.Time
}

// DeletedBranches lists the branches deleted from the team and not there
// again, the last deleted first.
func (r *Repo) DeletedBranches() ([]DeletedBranch, error) {
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
	found := map[string]DeletedBranch{}
	for key, m := range last {
		if m.To == "" && m.From != "" {
			found[key] = DeletedBranch{Key: key, Head: m.From, By: m.By, Time: m.Time}
		}
	}
	// A deletion the branch log missed (stopped before it was written).
	if store, ok := remote.BranchRecordsOf(c); ok {
		recs, err := store.BranchRecords(r.Config.ProjectID)
		if err != nil {
			return nil, err
		}
		for key, rec := range recs {
			if d := rec.Deleted; d != nil && d.Head != "" && !found[key].Time.After(d.Time) {
				found[key] = DeletedBranch{Key: key, Head: d.Head, By: d.By, Time: d.Time}
			}
		}
	}
	var out []DeletedBranch
	for key, d := range found {
		if _, there := heads[key]; !there && key != "main" {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// RestoreBranch puts deleted branch key back where it was when deleted.
func (r *Repo) RestoreBranch(key string) error {
	deleted, err := r.DeletedBranches()
	if err != nil {
		return err
	}
	for _, d := range deleted {
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
	return fmt.Errorf("no deleted branch %q", key)
}
