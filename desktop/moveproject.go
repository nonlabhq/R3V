package desktop

import (
	"errors"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// MoveProject moves project root to team teamID (copy: copies it, the
// project staying in its team): its whole history goes straight from one
// team's storage to the other's (see project.MoveToTeam), with the
// project's progress; CancelSave stops it (what was copied stays unseen
// on the other team, and moving again goes on from there). The files here
// are untouched.
func (a *App) MoveProject(root, teamID string, copy bool) error {
	a.stopPreupload(root)
	defer a.tidyLater(root)
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	defer a.cancellable(root, r)()
	store, err := teams.Load()
	if err != nil {
		return err
	}
	t := store.Find(teamID)
	if t == nil {
		return errors.New("unknown team")
	}
	if t.NoAccess {
		return errors.New("the account signed in isn't in that team")
	}
	if err := r.MoveToTeam(t, project.MoveOptions{Copy: copy}); err != nil {
		return err
	}
	if !copy {
		// The watch follows the folder to its new team.
		a.stopWatch(root)
		a.startWatch(root)
		a.storeTeam(root, nil, errors.New("asked again"))
		if view, err := r.FetchTeam(); err == nil {
			a.storeTeam(root, view, nil)
		}
	}
	return nil
}
