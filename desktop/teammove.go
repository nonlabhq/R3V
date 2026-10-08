package desktop

import (
	"errors"
	"log"
	"sync"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teammove"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team moving from its own storage to R3V Cloud (package teammove,
// Nightly): the steps go on in the background; "team-move" events (the
// old team's id) say when to ask TeamMoveState again.

// moves is R3V Cloud's part (tests put a fake).
var moves teammove.Mover = teammove.Cloud

// TeamMoveEstimate and TeamMoveState are teammove's, for the app.
type (
	TeamMoveEstimate = teammove.Estimate
	TeamMoveState    = teammove.State
)

// EstimateTeamMove plans every project of team teamID (reading its
// versions: a while for a long history) and says what moving means.
func (a *App) EstimateTeamMove(teamID string) (*TeamMoveEstimate, error) {
	if !remote.HostedTeams {
		return nil, cloud.ErrNotInBuild
	}
	return teammove.Plan(teamID)
}

var moving sync.Map // old team id → its step under way (one at a time)

// StartTeamMove starts moving team teamID's projects (ids) to hosted team
// toID with a read-only key (region "" for R2's "auto"); the plans and the
// copy go on in the background.
func (a *App) StartTeamMove(teamID, toID string, projects []string, accessKey, secretKey, region string) error {
	if !remote.HostedTeams {
		return cloud.ErrNotInBuild
	}
	if err := teammove.Start(moves, teamID, toID, projects, accessKey, secretKey, region); err != nil {
		return err
	}
	go a.runMove(teamID, false)
	return nil
}

// FinishTeamMove freezes the old team, sends what was shared since, and,
// once all of it is copied, finishes (in the background).
func (a *App) FinishTeamMove(teamID string) error {
	if !remote.HostedTeams {
		return cloud.ErrNotInBuild
	}
	s, err := teams.Load()
	if err != nil {
		return err
	}
	if s.Moves[teamID] == nil {
		return errors.New("this team isn't moving")
	}
	go a.runMove(teamID, true)
	return nil
}

// CancelTeamMove gives a move up (not once it's done).
func (a *App) CancelTeamMove(teamID string) error {
	err := teammove.Cancel(moves, teamID)
	a.emitMove(teamID)
	return err
}

// TeamMoveState says how team teamID's move goes.
func (a *App) TeamMoveState(teamID string) (*TeamMoveState, error) {
	st, err := teammove.Status(moves, teamID)
	if err == nil && st.Phase == "copying" {
		if _, busy := moving.Load(teamID); busy {
			st.Phase = "planning"
		}
	}
	return st, err
}

func (a *App) emitMove(teamID string) {
	if a.emit != nil {
		a.emit("team-move", teamID)
	}
}

// resumeTeamMoves picks up moves this computer was finishing (at start).
func (a *App) resumeTeamMoves() {
	s, err := teams.Load()
	if err != nil {
		return
	}
	for id, m := range s.Moves {
		if m.Phase == "finishing" {
			go a.runMove(id, true)
		}
	}
}

// runMove does a move's step in the background: the plans and copy, or
// (finish) the rest.
func (a *App) runMove(teamID string, finish bool) {
	if _, busy := moving.LoadOrStore(teamID, true); busy {
		return
	}
	defer moving.Delete(teamID)
	defer a.emitMove(teamID)
	a.emitMove(teamID)
	var err error
	if finish {
		var roots []string
		roots, err = teammove.Finish(moves, teamID, func(*cloud.MoveStatus) { a.emitMove(teamID) })
		for _, root := range roots { // the watches follow the folders to their new team
			a.stopWatch(root)
			a.startWatch(root)
		}
	} else {
		err = teammove.Copy(moves, teamID)
	}
	if err != nil {
		log.Printf("team move %s: %v", teamID, err)
	}
	teammove.Recorded(teamID, err)
}

// FollowMovedTeam is for a teammate: their team moved to R3V Cloud and
// they joined it there (toID); their projects of the old team follow.
func (a *App) FollowMovedTeam(fromID, toID string) error {
	if !remote.HostedTeams {
		return cloud.ErrNotInBuild
	}
	roots, err := teammove.FollowProjects(fromID, toID, nil)
	for _, root := range roots {
		a.stopWatch(root)
		a.startWatch(root)
	}
	return err
}
