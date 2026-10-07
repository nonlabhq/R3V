package desktop

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team moving from its own storage to R3V Cloud (docs/design/moving.md,
// Nightly). This computer plans and finishes; R3V Cloud's service copies
// the contents in the background:
//
//  1. StartTeamMove: the service gets a read-only key, the old members
//     come along to be claimed, each project's plan is sent, the copy
//     starts. The team keeps working meanwhile (phase "copying").
//  2. FinishTeamMove, once the bulk is copied: the old team is frozen, the
//     plans are sent again (what was shared since), and once that is
//     copied too, each project's records and branches are written on the
//     hosted team, the old team says where it went, this computer's
//     projects follow, and the service forgets the key (phase "done").
//
// The move is kept in the team store, so a restart picks it up; each step
// is safe to do again.

// mover is R3V Cloud's part (package cloud; tests put a fake).
type mover interface {
	StartMove(service, team string, src cloud.MoveSource) (string, error)
	AddPlan(service, team, move, project string, items []cloud.PlanItem) (int, int, error)
	StartCopy(service, team, move string) error
	Move(service, team, move string) (*cloud.MoveStatus, error)
	ForgetMove(service, team, move string) error
	ImportMembers(service, team string, ms []cloud.ImportedMember) error
	Claim(service, team, id string) error
}

type cloudMover struct{}

func (cloudMover) StartMove(s, t string, src cloud.MoveSource) (string, error) {
	return cloud.StartMove(s, t, src)
}
func (cloudMover) AddPlan(s, t, m, p string, items []cloud.PlanItem) (int, int, error) {
	return cloud.AddPlan(s, t, m, p, items)
}
func (cloudMover) StartCopy(s, t, m string) error                 { return cloud.StartCopy(s, t, m) }
func (cloudMover) Move(s, t, m string) (*cloud.MoveStatus, error) { return cloud.Move(s, t, m) }
func (cloudMover) ForgetMove(s, t, m string) error                { return cloud.ForgetMove(s, t, m) }
func (cloudMover) ImportMembers(s, t string, ms []cloud.ImportedMember) error {
	return cloud.ImportMembers(s, t, ms)
}
func (cloudMover) Claim(s, t, id string) error { return cloud.Claim(s, t, id) }

var moves mover = cloudMover{}

// moveEvery is how often finishing asks how the copy goes.
var moveEvery = 2 * time.Second

// TeamMoveEstimate is what moving a team means, before anything is done.
type TeamMoveEstimate struct {
	Projects []TeamMoveProject `json:"projects"`
	Hosted   int64             `json:"hosted"`  // bytes R3V Cloud will keep
	Storage  int64             `json:"storage"` // bytes the team's storage holds now
	People   int               `json:"people"`
	// Where the team's storage is (for making the read-only key): its
	// address, bucket, folder; "r2", "aws" or "" for another.
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Folder   string `json:"folder"`
	Provider string `json:"provider"`
}

// TeamMoveProject is one project's part.
type TeamMoveProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Versions int    `json:"versions"`
	Bytes    int64  `json:"bytes"`
}

// TeamMoveState is how a team's move goes.
type TeamMoveState struct {
	Phase     string `json:"phase"` // "" (none), "planning", "copying", "finishing", "done"
	To        string `json:"to"`    // the hosted team's id
	Items     int    `json:"items"`
	ItemsDone int    `json:"itemsDone"`
	Bytes     int64  `json:"bytes"`
	BytesDone int64  `json:"bytesDone"`
	Copied    bool   `json:"copied"` // the service copied all it was given
	Failed    int    `json:"failed"`
	Error     string `json:"error"`
}

func storageTeam(teamID string) (*teams.Team, error) {
	s, err := teams.Load()
	if err != nil {
		return nil, err
	}
	t := s.Find(teamID)
	if t == nil {
		return nil, errors.New("unknown team")
	}
	if _, hosted := cloud.Hosted(*t); hosted {
		return nil, errors.New("this team is on R3V Cloud already")
	}
	return t, nil
}

func movingTo(teamID string) (*teams.Team, string, string, error) {
	s, err := teams.Load()
	if err != nil {
		return nil, "", "", err
	}
	t := s.Find(teamID)
	if t == nil {
		return nil, "", "", errors.New("unknown team")
	}
	svc, hosted := cloud.Hosted(*t)
	if !hosted {
		return nil, "", "", errors.New("pick a team on R3V Cloud")
	}
	return t, svc, t.Remote.URL[strings.LastIndex(t.Remote.URL, "/")+1:], nil
}

// EstimateTeamMove plans every project of team teamID (reading its
// versions: a while for a long history) and says what moving means.
func (a *App) EstimateTeamMove(teamID string) (*TeamMoveEstimate, error) {
	if !remote.HostedTeams {
		return nil, cloud.ErrNotInBuild
	}
	t, err := storageTeam(teamID)
	if err != nil {
		return nil, err
	}
	est, err := project.EstimateMove(t)
	if err != nil {
		return nil, err
	}
	out := &TeamMoveEstimate{Projects: []TeamMoveProject{}, Hosted: est.Hosted, Storage: est.Storage, People: est.People}
	for _, p := range est.Plans {
		out.Projects = append(out.Projects, TeamMoveProject{p.Project.ID, p.Project.Name, p.Versions, p.Bytes})
	}
	if st, ok := remote.StorageOf(t.Remote); ok {
		out.Endpoint, out.Bucket, out.Folder = st.Endpoint, st.Bucket, st.Folder
		switch {
		case strings.Contains(st.Endpoint, "r2.cloudflarestorage.com"):
			out.Provider = "r2"
		case strings.Contains(st.Endpoint, "amazonaws.com"):
			out.Provider = "aws"
		}
	}
	return out, nil
}

var moving sync.Map // old team id → its step under way (one at a time)

// StartTeamMove starts moving team teamID's projects (ids) to hosted team
// toID: the service gets the read-only key (accessKey, secretKey; region
// "" for R2's "auto"), then plans and the copy go on in the background
// (TeamMoveState says how; "team-move" events tell of changes).
func (a *App) StartTeamMove(teamID, toID string, projects []string, accessKey, secretKey, region string) error {
	if !remote.HostedTeams {
		return cloud.ErrNotInBuild
	}
	from, err := storageTeam(teamID)
	if err != nil {
		return err
	}
	_, svc, team, err := movingTo(toID)
	if err != nil {
		return err
	}
	st, ok := remote.StorageOf(from.Remote)
	if !ok {
		return errors.New("this team's storage can't be read by R3V Cloud: move its projects one by one instead")
	}
	if region == "" {
		region = st.Region
	}
	if region == "" {
		region = "auto"
	}
	move, err := moves.StartMove(svc, team, cloud.MoveSource{Endpoint: st.Endpoint, Region: region, Bucket: st.Bucket,
		Prefix: strings.Trim(st.Folder, "/"), AccessKey: strings.TrimSpace(accessKey), SecretKey: strings.TrimSpace(secretKey)})
	if err != nil {
		return err
	}
	if _, err := teams.Update(func(s *teams.Store) error {
		if s.Moves == nil {
			s.Moves = map[string]*teams.Move{}
		}
		s.Moves[teamID] = &teams.Move{Move: move, To: toID, Projects: projects, Phase: "copying"}
		return nil
	}); err != nil {
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
	if _, err := teams.Update(func(s *teams.Store) error {
		m := s.Moves[teamID]
		if m == nil {
			return errors.New("this team isn't moving")
		}
		m.Phase, m.Error = "finishing", ""
		return nil
	}); err != nil {
		return err
	}
	go a.runMove(teamID, true)
	return nil
}

// CancelTeamMove gives a move up: the old team is as it was (unfrozen),
// the service forgets the key; what was copied stays unseen on the hosted
// team. Not once it's done.
func (a *App) CancelTeamMove(teamID string) error {
	s, err := teams.Load()
	if err != nil {
		return err
	}
	m := s.Moves[teamID]
	if m == nil {
		return nil
	}
	if m.Phase == "done" {
		return errors.New("the team has moved already")
	}
	if from := s.Find(teamID); from != nil {
		if b, err := from.Open(); err == nil {
			if info, err := b.Info(); err == nil && info.Moving != nil {
				info.Moving = nil
				if err := b.SetInfo(info); err != nil {
					return err
				}
			}
		}
	}
	if _, svc, team, err := movingTo(m.To); err == nil {
		moves.ForgetMove(svc, team, m.Move)
	}
	_, err = teams.Update(func(s *teams.Store) error {
		delete(s.Moves, teamID)
		return nil
	})
	a.emitMove(teamID)
	return err
}

// TeamMoveState says how team teamID's move goes (asking the service).
func (a *App) TeamMoveState(teamID string) (*TeamMoveState, error) {
	s, err := teams.Load()
	if err != nil {
		return nil, err
	}
	m := s.Moves[teamID]
	if m == nil {
		return &TeamMoveState{}, nil
	}
	out := &TeamMoveState{Phase: m.Phase, To: m.To, Error: m.Error}
	if _, busy := moving.Load(teamID); busy && m.Phase == "copying" {
		out.Phase = "planning"
	}
	if m.Phase == "done" {
		return out, nil
	}
	if _, svc, team, err := movingTo(m.To); err == nil {
		if st, err := moves.Move(svc, team, m.Move); err == nil {
			out.Items, out.ItemsDone, out.Bytes, out.BytesDone = st.Items, st.ItemsDone, st.Bytes, st.BytesDone
			out.Copied = st.State == "done"
			out.Failed = len(st.Failed)
			if st.State == "cancelled" && out.Error == "" {
				out.Error = "R3V Cloud stopped the move (left alone too long): start it again"
			}
		}
	}
	return out, nil
}

func (a *App) emitMove(teamID string) {
	if a.emit != nil {
		a.emit("team-move", teamID)
	}
}

// ResumeTeamMoves picks up moves this computer was making (at start).
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

// runMove does a move's step: planning and copying, or (finish) the rest.
func (a *App) runMove(teamID string, finish bool) {
	if _, busy := moving.LoadOrStore(teamID, true); busy {
		return
	}
	defer moving.Delete(teamID)
	defer a.emitMove(teamID)
	a.emitMove(teamID)
	err := a.moveStep(teamID, finish)
	if err != nil {
		log.Printf("team move %s: %v", teamID, err)
	}
	teams.Update(func(s *teams.Store) error {
		if m := s.Moves[teamID]; m != nil {
			m.Error = ""
			if err != nil {
				m.Error = err.Error()
			}
		}
		return nil
	})
}

func (a *App) moveStep(teamID string, finish bool) error {
	s, err := teams.Load()
	if err != nil {
		return err
	}
	m := s.Moves[teamID]
	from := s.Find(teamID)
	if m == nil || from == nil {
		return errors.New("this team isn't moving")
	}
	to, svc, team, err := movingTo(m.To)
	if err != nil {
		return err
	}
	fb, err := from.Open()
	if err != nil {
		return err
	}
	if finish {
		// Frozen: nobody's R3V shares to the old team from now on.
		info, err := fb.Info()
		if err != nil {
			return err
		}
		if info.Moving == nil && info.MovedTo == "" {
			info.Moving = &remote.TeamMove{To: to.Remote.URL, By: from.MemberID, Time: time.Now().UTC()}
			if err := fb.SetInfo(info); err != nil {
				return err
			}
		}
	}
	// The old members, to be claimed (again: names brought up to date).
	if ms, err := fb.Members(); err == nil {
		var in []cloud.ImportedMember
		for _, mb := range ms {
			in = append(in, cloud.ImportedMember{ID: mb.ID, Name: mb.Name, Color: mb.Color})
		}
		if len(in) > 0 {
			if err := moves.ImportMembers(svc, team, in); err != nil {
				return fmt.Errorf("members: %w", err)
			}
		}
	}
	// The plans (again when finishing: what was shared since; what is
	// there already is skipped).
	projects, err := fb.Projects()
	if err != nil {
		return err
	}
	sizes, err := project.SizesOf(from)
	if err != nil {
		return err
	}
	st, _ := remote.StorageOf(from.Remote)
	prefix := strings.Trim(st.Folder, "/")
	var chosen []remote.Project
	for _, p := range projects {
		if slices.Contains(m.Projects, p.ID) {
			chosen = append(chosen, p)
		}
	}
	for _, p := range chosen {
		plan, err := project.PlanMove(from, p, sizes)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		items := make([]cloud.PlanItem, len(plan.Items))
		for i, it := range plan.Items {
			src := it.Src
			if prefix != "" {
				src = prefix + "/" + src
			}
			items[i] = cloud.PlanItem{Src: src, Dst: it.Dst, Size: it.Size}
		}
		if _, _, err := moves.AddPlan(svc, team, m.Move, p.ID, items); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	if err := moves.StartCopy(svc, team, m.Move); err != nil {
		return err
	}
	if !finish {
		return nil
	}
	// Waiting for the copy (the app may close: the move picks up again).
	for {
		status, err := moves.Move(svc, team, m.Move)
		if err != nil {
			return err
		}
		a.emitMove(teamID)
		if status.State == "done" {
			break
		}
		switch status.State {
		case "failed":
			return fmt.Errorf("%d files couldn't be copied (Finish again to retry)", len(status.Failed))
		case "paused":
			return errors.New("R3V Cloud paused the copy: the team's plan has no room for it")
		case "cancelled":
			return errors.New("R3V Cloud stopped the move (left alone too long): start it again")
		}
		time.Sleep(moveEvery)
	}
	// Records and branches, project by project: then listed there.
	for _, p := range chosen {
		if err := project.FinishMove(from, to, p); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	// The old team says where it went; whoever moved it is their old self.
	info, err := fb.Info()
	if err != nil {
		return err
	}
	info.Moving, info.MovedTo = nil, to.Remote.URL
	if err := fb.SetInfo(info); err != nil {
		return err
	}
	if from.MemberID != "" {
		moves.Claim(svc, team, from.MemberID) // (one already claimed: it stays)
	}
	if err := a.followMovedTeam(teamID, m.To, m.Projects); err != nil {
		return err
	}
	moves.ForgetMove(svc, team, m.Move)
	_, err = teams.Update(func(s *teams.Store) error {
		if mv := s.Moves[teamID]; mv != nil {
			mv.Phase = "done"
		}
		return nil
	})
	return err
}

// followMovedTeam has this computer's projects of team fromID (those in
// ids; all when nil) belong to team toID from now on: the same folders,
// versions and files, nothing downloaded.
func (a *App) followMovedTeam(fromID, toID string, ids []string) error {
	s, err := teams.Load()
	if err != nil {
		return err
	}
	to := s.Find(toID)
	if to == nil {
		return errors.New("unknown team")
	}
	for key, root := range s.Projects {
		pid, ok := strings.CutPrefix(key, fromID+"/")
		if !ok || (ids != nil && !slices.Contains(ids, pid)) {
			continue
		}
		r, err := project.Open(root)
		if err != nil {
			continue // the folder is gone: nothing to follow
		}
		if err := r.JoinTeam(to); err != nil {
			return err
		}
		teams.Update(func(s *teams.Store) error {
			s.ForgetProject(fromID, pid)
			return nil
		})
		a.stopWatch(root)
		a.startWatch(root)
	}
	return nil
}

// FollowMovedTeam is for a teammate: their team moved to R3V Cloud and
// they joined it there (toID); their projects of the old team follow.
func (a *App) FollowMovedTeam(fromID, toID string) error {
	if !remote.HostedTeams {
		return cloud.ErrNotInBuild
	}
	return a.followMovedTeam(fromID, toID, nil)
}
