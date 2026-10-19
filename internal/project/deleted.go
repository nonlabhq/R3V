package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A project deleted from its team while a copy still points at it: moved
// to another team, or deleted for everyone, from another copy or another
// computer. It was done on purpose, so a share from such a copy (a stale
// one, or an agent committing) fails with what happened (DeletedError)
// rather than bringing it back; the versions stay here. Bringing it back
// is asked for (RestoreDeleted): R3V Cloud keeps a deleted project 30
// days, out of reach, and lets its owners and admins restore it whole; a
// team's own storage deletes it for good, and it is shared anew. Moved,
// the copy can follow it instead (FollowTo). docs/design/moving.md.

// ErrProjectDeleted: the project was deleted from its team; a share
// doesn't bring it back unless asked. Its versions stay here.
var ErrProjectDeleted = errors.New("this project was deleted from the team")

// DeletedError says what happened to a project deleted from its team, as
// far as the team's log tells (it is ErrProjectDeleted).
type DeletedError struct {
	Project string    // its name
	Team    string    // the team it was deleted from (this copy's)
	Time    time.Time // when; zero if the log doesn't say
	By      string    // who (a member's name), if the log says
	// MovedTo: it moved to this team (its name then); MovedToTeam: that
	// team's id on this computer, when it has joined it.
	MovedTo     string
	MovedToTeam string
	// NotAllowed: bringing it back was asked for and refused: on R3V
	// Cloud only the team's owners and admins do.
	NotAllowed bool
	// Gone: asked for, it can't be brought back (the team no longer keeps it).
	Gone bool
}

func (e *DeletedError) Is(target error) bool { return target == ErrProjectDeleted }

func (e *DeletedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s was removed from %s", e.Project, e.Team)
	if !e.Time.IsZero() {
		fmt.Fprintf(&b, " on %s", e.Time.Local().Format("2006-01-02"))
	}
	if e.By != "" {
		fmt.Fprintf(&b, " (by %s)", e.By)
	}
	if e.MovedTo != "" {
		fmt.Fprintf(&b, "; it moved to %s: this copy still points at %s", e.MovedTo, e.Team)
	}
	b.WriteString("; the versions stay here")
	switch {
	case e.NotAllowed:
		b.WriteString("; only the team's owners and admins can bring it back: ask one")
	case e.Gone:
		b.WriteString("; the team no longer keeps it, so it can't be brought back")
	}
	return b.String()
}

// removedLogDays: how far back the team's log is read for a project's
// removal (R3V Cloud keeps a deleted project 30 days; a team's own storage
// for good, its log too).
const removedLogDays = 366

// removal is the team's last word on project pid: its last
// project-added or project-removed record (nil when the log has none).
func removal(c remote.Backend, pid string) *remote.TeamEvent {
	tl, ok := c.(remote.TeamLogger)
	if !ok {
		return nil
	}
	evs, err := tl.TeamLog(time.Now().AddDate(0, 0, -removedLogDays))
	if err != nil {
		return nil
	}
	var last *remote.TeamEvent
	for i := range evs {
		if evs[i].Project == pid && (evs[i].Kind == remote.EventProjectAdded || evs[i].Kind == remote.EventProjectRemoved) {
			last = &evs[i]
		}
	}
	return last
}

// deletedError says what the team's log tells of the project's removal;
// nil when it tells none and known is false (a project new to the team).
func (r *Repo) deletedError(c remote.Backend, known bool) *DeletedError {
	ev := removal(c, r.Config.ProjectID)
	if (ev == nil || ev.Kind != remote.EventProjectRemoved) && !known {
		return nil
	}
	e := &DeletedError{Project: r.Config.Name}
	from, _ := r.Team()
	if from != nil {
		e.Team = from.Name
	}
	defer func() {
		if e.MovedToTeam == "" && from != nil {
			if t := movedTo(from.ID, r.Config.ProjectID); t != nil {
				e.MovedTo, e.MovedToTeam = t.Name, t.ID
			}
		}
	}()
	if ev == nil || ev.Kind != remote.EventProjectRemoved {
		return e
	}
	e.Time, e.MovedTo = ev.Time, ev.MovedTo
	if ev.By != "" {
		if ms, err := c.Members(); err == nil {
			for _, m := range ms {
				if m.ID == ev.By {
					e.By = m.Name
				}
			}
		}
	}
	if ev.MovedToRef != "" {
		if store, err := teams.Load(); err == nil {
			if t := store.FindByRef(ev.MovedToRef); t != nil {
				e.MovedToTeam = t.ID
			}
		}
	}
	return e
}

// addToTeam writes the record of a project not on the team (listed: the
// team's list was read): a first share. Deleted from the team, it fails
// (DeletedError) unless the person asked to bring it back.
func (r *Repo) addToTeam(c remote.Backend, listed bool) error {
	pid := r.Config.ProjectID
	var gone *DeletedError
	if listed {
		if gone = r.deletedError(c, false); gone != nil && !r.RestoreDeleted {
			return gone
		}
	}
	err := c.PutProject(remote.Project{ID: pid, Name: r.Config.Name})
	if errors.Is(err, remote.ErrNotFound) {
		// A hosted team deleted it and keeps it (with no log record of it:
		// an older R3V, or the team's log out of reach).
		if gone == nil {
			gone = r.deletedError(c, true)
		}
		if !r.RestoreDeleted {
			return gone
		}
		err = r.restoreProject(c, gone)
	}
	if err != nil {
		return err
	}
	if gone != nil {
		r.restored = true
	}
	remote.LogTeamEvent(c, remote.TeamEvent{Kind: remote.EventProjectAdded, Project: pid, Name: r.Config.Name})
	return nil
}

// restoreProject brings back a project its hosted team deleted and keeps:
// its records and versions as they were; the share goes on from them.
func (r *Repo) restoreProject(c remote.Backend, gone *DeletedError) error {
	pid := r.Config.ProjectID
	err := remote.RestoreProject(c, pid)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, remote.ErrForbidden):
		gone.NotAllowed = true
		return gone
	case errors.Is(err, remote.ErrNotFound):
		// Brought back meanwhile, or not kept any more: as new.
		if err := c.PutProject(remote.Project{ID: pid, Name: r.Config.Name}); !errors.Is(err, remote.ErrNotFound) {
			return err
		}
	case !errors.Is(err, errors.ErrUnsupported):
		return err
	}
	gone.Gone = true
	return gone
}

// Deleted says whether the project was deleted from its team (what the
// team's log tells of it), nil when it is on the team or new to it.
func (r *Repo) Deleted() (*DeletedError, error) {
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	ps, err := c.Projects()
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(ps, func(p remote.Project) bool { return p.ID == r.Config.ProjectID }) {
		return nil, nil
	}
	return r.deletedError(c, false), nil
}

// FollowTo points this copy at team t, which has its project: the team it
// moved to (DeletedError.MovedToTeam), as moving it from here would have.
// Refused unless the copy's own team no longer has the project. Its
// versions not shared yet are shared there next, as any commit's are (the
// other team's versions merged first, when it moved on).
//
// Stopped half-way: registered for t with the folder still the first
// team's (FollowTo again does it), or the folder t's and still registered
// for the first team too (nothing to do: the folder says t).
func (r *Repo) FollowTo(t *teams.Team) error {
	if r.IsWorkspace() {
		return ErrInWorkspace
	}
	from, err := r.Team()
	if err != nil {
		return err
	}
	if from.ID == t.ID {
		return nil
	}
	gone, err := r.Deleted()
	if err != nil {
		return err
	}
	if gone == nil {
		return fmt.Errorf("%s is still on %s: move it from there (Project settings), not by pointing this copy elsewhere", r.Config.Name, from.Name)
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	ps, err := b.Projects()
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(ps, func(p remote.Project) bool { return p.ID == r.Config.ProjectID }) {
		return fmt.Errorf("%s isn't on %s", r.Config.Name, t.Name)
	}
	if err := r.JoinTeam(t); err != nil {
		return err
	}
	_, err = teams.Update(func(s *teams.Store) error {
		if s.ProjectRoot(from.ID, r.Config.ProjectID) == r.Root {
			s.ForgetProject(from.ID, r.Config.ProjectID)
		}
		return nil
	})
	return err
}

// movedTo finds, among the other teams this computer has joined, one that
// has project pid: where it went, when the first team's log doesn't say
// (moved by an older R3V). Asked only once the project is known gone, of
// all of them at once, for MovedToWait at most (a team out of reach is
// left out).
func movedTo(fromID, pid string) *teams.Team {
	store, err := teams.Load()
	if err != nil {
		return nil
	}
	found := make(chan *teams.Team, len(store.Teams))
	n := 0
	for i, t := range store.Teams {
		if t.ID == fromID || t.NoAccess {
			continue
		}
		n++
		go func(t *teams.Team) {
			if b, err := t.Open(); err == nil {
				if ps, err := b.Projects(); err == nil && slices.ContainsFunc(ps, func(p remote.Project) bool { return p.ID == pid }) {
					found <- t
					return
				}
			}
			found <- nil
		}(&store.Teams[i])
	}
	timeout := time.After(MovedToWait)
	for ; n > 0; n-- {
		select {
		case t := <-found:
			if t != nil {
				return t
			}
		case <-timeout:
			return nil
		}
	}
	return nil
}

// MovedToWait: how long movedTo waits for the teams' answers (tests
// shorten it).
var MovedToWait = 10 * time.Second
