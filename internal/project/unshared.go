package project

import (
	"errors"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Versions not shared: committed on this computer only (commit --local, a
// share that didn't get through), they are in the store every copy of the
// project here shares, but on no team branch, so another copy merging or
// updating can't see them. Saying "up to date" then would be wrong: the
// copies that have them are named instead.

// UnsharedCopy is a copy of the project on this computer whose branch has
// versions the team's branch doesn't.
type UnsharedCopy struct {
	Folder string
	Main   bool   // the project's main copy (else a workspace)
	Branch string // its branch's key
	Count  int    // its versions the team's branch lacks
}

// unsharedOn counts the versions in head's history that teamHead's lacks
// (teamHead "": the branch isn't on the team, so all of them). Its history
// must be here (fetchSnapshots); -1 when it can't tell.
func (r *Repo) unsharedOn(head, teamHead string) int {
	if head == "" {
		return 0
	}
	mine, err := r.ancestors(head)
	if err != nil {
		return -1
	}
	if teamHead != "" {
		theirs, err := r.ancestors(teamHead)
		if err != nil {
			return -1
		}
		for id := range theirs {
			delete(mine, id)
		}
	}
	return len(mine)
}

// teamHeads are the team's branches, their history fetched as far as it
// isn't here (nil, nil: the project isn't on the team yet).
func (r *Repo) teamHeads() (remote.Backend, map[string]string, error) {
	c, err := r.Client()
	if err != nil {
		return nil, nil, err
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if errors.Is(err, remote.ErrNotFound) {
		return c, nil, nil
	}
	return c, heads, err
}

// Unshared counts this copy's versions its team branch doesn't have, asking
// the team (no team: 0).
func (r *Repo) Unshared() (int, error) {
	if r.Config.Remote == nil {
		return 0, nil
	}
	c, heads, err := r.teamHeads()
	if err != nil {
		return 0, err
	}
	team := heads[r.BranchName()]
	if team != "" {
		if err := r.fetchSnapshots(c, team); err != nil {
			return 0, err
		}
	}
	n := r.unsharedOn(r.Latest(), team)
	if n < 0 {
		return 0, errors.New("the history here is incomplete: can't tell what is shared")
	}
	return n, nil
}

// UnsharedElsewhere lists the project's other copies on this computer on
// branch (a key; "": any) with versions the team's branch doesn't have:
// what a merge or an update here can't see. Asks the team; a copy it can't
// tell about is left out.
func (r *Repo) UnsharedElsewhere(branch string) ([]UnsharedCopy, error) {
	others := r.otherCopies()
	if len(others) == 0 || r.Config.Remote == nil {
		return nil, nil
	}
	c, heads, err := r.teamHeads()
	if err != nil {
		return nil, err
	}
	var out []UnsharedCopy
	for _, o := range others {
		b := o.BranchName()
		if branch != "" && b != branch {
			continue
		}
		team := heads[b]
		if team != "" && r.fetchSnapshots(c, team) != nil {
			continue
		}
		if n := r.unsharedOn(o.Latest(), team); n > 0 {
			out = append(out, UnsharedCopy{Folder: o.Root, Main: !o.IsWorkspace(), Branch: b, Count: n})
		}
	}
	return out, nil
}
