package desktop

import (
	"errors"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A project its team deleted while this copy still points at it (moved to
// another team, or deleted for everyone, elsewhere): a share or commit
// here stops with Result action "deleted" (the versions stay here), and
// the app asks: point this copy at the team it moved to, bring it back
// (owners and admins), or nothing. Never done without asking.

// DeletedInfo is what the team's log tells of it.
type DeletedInfo struct {
	Project string `json:"project"`
	Team    string `json:"team"`
	Time    string `json:"time"` // RFC 3339; "" when unknown
	By      string `json:"by"`   // a member's name; "" when unknown
	// MovedTo: the team it moved to (""); MovedToTeam: its id here, when
	// this computer has joined it (offer to point the copy at it).
	MovedTo     string `json:"movedTo"`
	MovedToTeam string `json:"movedToTeam"`
	// CanRestore: this person may bring it back (on R3V Cloud, the team's
	// owners and admins).
	CanRestore bool `json:"canRestore"`
}

// deletedResult is a share's DeletedError as a Result.
func deletedResult(r *project.Repo, err error) (*Result, bool) {
	var gone *project.DeletedError
	if !errors.As(err, &gone) {
		return nil, false
	}
	d := &DeletedInfo{Project: gone.Project, Team: gone.Team, By: gone.By, MovedTo: gone.MovedTo,
		MovedToTeam: gone.MovedToTeam, CanRestore: !gone.NotAllowed && mayRestore(r)}
	if !gone.Time.IsZero() {
		d.Time = gone.Time.UTC().Format(time.RFC3339)
	}
	out := syncResult(nil)
	out.Action, out.Deleted = "deleted", d
	return out, true
}

// mayRestore: on R3V Cloud only a team's owners and admins bring a project
// back; on a team's own storage, anyone in it.
func mayRestore(r *project.Repo) bool {
	t, err := r.Team()
	if err != nil {
		return false
	}
	svc, ok := cloud.Hosted(*t)
	if !ok {
		return true
	}
	me, err := cloud.GetMe(svc)
	if err != nil {
		return false
	}
	for _, mt := range me.Teams {
		if mt.ID == cloud.TeamID(t.Remote.URL) {
			return mt.Role == "owner" || mt.Role == "admin"
		}
	}
	return false
}

// RestoreProject brings the project back to its team, which deleted it,
// with its history, and shares the versions here: asked for in the app
// (owners and admins).
func (a *App) RestoreProject(root string) (*Result, error) {
	a.stopPreupload(root)
	defer a.tidyLater(root)
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	defer a.cancellable(root, r)()
	r.RestoreDeleted = true
	res, err := r.Share(opts(nil))
	if errors.Is(err, project.ErrCancelled) {
		return cancelled(nil, true), nil
	}
	if err != nil {
		return conflictResult(r, err)
	}
	a.afterShare(root)
	return syncResult(res), nil
}

// FollowProject points this copy at team teamID, which its project moved
// to (DeletedInfo.MovedToTeam); its versions not shared yet are shared
// there by the next share or commit.
func (a *App) FollowProject(root, teamID string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	store, err := teams.Load()
	if err != nil {
		return err
	}
	t := store.Find(teamID)
	if t == nil {
		return errors.New("unknown team")
	}
	if err := r.FollowTo(t); err != nil {
		return err
	}
	// The watch and the team's view follow the folder to its new team.
	a.stopWatch(root)
	a.startWatch(root)
	a.storeTeam(root, nil, errors.New("asked again"))
	if view, err := r.FetchTeam(); err == nil {
		a.storeTeam(root, view, nil)
	}
	return nil
}
