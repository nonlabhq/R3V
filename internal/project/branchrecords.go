package project

import (
	"errors"
	"fmt"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Branch names and colours (remote.BranchRecord): what people call a
// branch, apart from its key. On a team that keeps no records (Stable, a
// hosted team not yet) a branch is called by its key, as before.

// BranchRecords maps the team's branch keys to their records (none on a
// team that keeps no records).
func (r *Repo) BranchRecords() (map[string]remote.BranchRecord, error) {
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	store, ok := remote.BranchRecordsOf(c)
	if !ok {
		return map[string]remote.BranchRecord{}, nil
	}
	return store.BranchRecords(r.Config.ProjectID)
}

// BranchLabel is what branch key is called, asking the team (its key when
// it can't).
func (r *Repo) BranchLabel(key string) string {
	recs, err := r.BranchRecords()
	if err != nil {
		return key
	}
	return branchLabel(recs, key)
}

// branchLabel is what branch key is called: its record's name, else key.
func branchLabel(recs map[string]remote.BranchRecord, key string) string {
	if n := recs[key].Name; n != "" {
		return n
	}
	return key
}

// CreateBranchNamed starts a branch called name (any text) with color (a
// palette number, "" for the app's pick) at the current version, and
// switches to it. It returns the branch's key. On a team that keeps no
// records, name is the key (CreateBranch).
func (r *Repo) CreateBranchNamed(name, color string) (string, error) {
	return r.NewBranch(name, color, true)
}

// NewBranch is CreateBranchNamed, the project going on with the branch only
// when switchTo is set (else it stays where it is).
func (r *Repo) NewBranch(name, color string, switchTo bool) (string, error) {
	c, err := r.Client()
	if err != nil {
		return "", err
	}
	store, ok := remote.BranchRecordsOf(c)
	if !ok {
		return name, r.createBranch(name, switchTo)
	}
	if r.Head() == "" {
		return "", errors.New("save a first version before creating branches")
	}
	if name, err = remote.CleanBranchName(name); err != nil {
		return "", err
	}
	if !remote.ValidLookName(color) {
		return "", errors.New("unknown colour")
	}
	pid := r.Config.ProjectID
	heads, err := c.Branches(pid)
	if err != nil && !errors.Is(err, remote.ErrNotFound) {
		return "", err
	}
	recs, err := store.BranchRecords(pid)
	if err != nil {
		return "", err
	}
	if err := nameFree(heads, recs, "", name); err != nil {
		return "", err
	}
	key := remote.BranchKeyFor(name, func(k string) bool { _, ok := heads[k]; return ok || k == r.BranchName() })
	// The record first: stopped before the branch, it is never shown.
	if err := store.PutBranchRecord(pid, key, remote.BranchRecord{Name: name, Color: color, Parent: r.BranchName(), From: r.Head()}); err != nil {
		return "", err
	}
	return key, r.createBranch(key, switchTo)
}

// SetBranchRecord renames branch key and sets its colour, for the whole
// team.
func (r *Repo) SetBranchRecord(key, name, color string) error {
	c, err := r.Client()
	if err != nil {
		return err
	}
	store, ok := remote.BranchRecordsOf(c)
	if !ok {
		return remote.ErrNoBranchRecords
	}
	if name, err = remote.CleanBranchName(name); err != nil {
		return err
	}
	if !remote.ValidLookName(color) {
		return errors.New("unknown colour")
	}
	pid := r.Config.ProjectID
	heads, err := c.Branches(pid)
	if err != nil {
		return err
	}
	if _, ok := heads[key]; !ok {
		return fmt.Errorf("no branch %q on the team", key)
	}
	recs, err := store.BranchRecords(pid)
	if err != nil {
		return err // never write the record back without what it holds
	}
	if err := nameFree(heads, recs, key, name); err != nil {
		return err
	}
	rec := recs[key]
	rec.Name, rec.Color = name, color
	return store.PutBranchRecord(pid, key, rec)
}

// nameFree fails when a branch other than self is already called name.
func nameFree(heads map[string]string, recs map[string]remote.BranchRecord, self, name string) error {
	for k := range heads {
		if k != self && remote.SameBranchName(branchLabel(recs, k), name) {
			return fmt.Errorf("there is already a branch called %q", branchLabel(recs, k))
		}
	}
	return nil
}

// ResolveBranch finds the key of the branch called name (its key, or the
// name people gave it).
func (r *Repo) ResolveBranch(name string) (string, error) {
	c, err := r.Client()
	if err != nil {
		return "", err
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil && !errors.Is(err, remote.ErrNotFound) {
		return "", err
	}
	if _, ok := heads[name]; ok {
		return name, nil
	}
	if store, ok := remote.BranchRecordsOf(c); ok {
		recs, err := store.BranchRecords(r.Config.ProjectID)
		if err != nil {
			return "", err
		}
		for k := range heads {
			if remote.SameBranchName(branchLabel(recs, k), name) {
				return k, nil
			}
		}
	}
	return name, nil // as given: the caller says it isn't there
}
