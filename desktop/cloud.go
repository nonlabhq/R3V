package desktop

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/teams"
	"github.com/nonlabhq/r3v/internal/version"
)

// R3V-Cloud, the hosted service, from the app (Nightly while it's built):
// signing in, creating and joining teams, and managing a hosted team's
// people, invitations and project access.

// CloudStatus is where the app stands with the service.
type CloudStatus struct {
	// Available: this build offers it (the Nightly channel).
	Available bool   `json:"available"`
	Service   string `json:"service"`
	SignedIn  bool   `json:"signedIn"`
	Email     string `json:"email"`
}

// CloudStatus says whether this build offers R3V-Cloud and who is signed
// in (asking the service only when signed in).
func (a *App) CloudStatus() CloudStatus {
	svc := cloud.Service()
	s := CloudStatus{Available: version.Channel == "nightly", Service: svc, SignedIn: cloud.SignedIn(svc)}
	if s.Available && s.SignedIn {
		if me, err := cloud.GetMe(svc); err == nil {
			s.Email = me.User.Email
		}
	}
	return s
}

var signIn struct {
	sync.Mutex
	cancel context.CancelFunc
}

// CloudSignIn signs in through the browser and adds the account's teams to
// the team list. It waits until the person is done in the browser (or
// CloudCancelSignIn).
func (a *App) CloudSignIn() (CloudStatus, error) {
	if version.Channel != "nightly" {
		return CloudStatus{}, cloud.ErrNotInBuild
	}
	ctx, cancel := context.WithCancel(context.Background())
	signIn.Lock()
	if signIn.cancel != nil {
		signIn.cancel()
	}
	signIn.cancel = cancel
	signIn.Unlock()
	defer cancel()
	svc := cloud.Service()
	_, err := cloud.SignIn(ctx, svc, func(addr string) error {
		if a.openURL == nil {
			return errors.New("can't open the browser")
		}
		return a.openURL(addr)
	})
	if err != nil {
		return CloudStatus{}, err
	}
	if err := a.syncTeams(svc); err != nil {
		return CloudStatus{}, err
	}
	a.selectFirstHosted(svc)
	return a.CloudStatus(), nil
}

// CloudCancelSignIn stops waiting for the browser.
func (a *App) CloudCancelSignIn() {
	signIn.Lock()
	defer signIn.Unlock()
	if signIn.cancel != nil {
		signIn.cancel()
		signIn.cancel = nil
	}
}

// syncHosted brings the hosted teams up to date at start (teams joined or
// left meanwhile) and tells the frontend.
func (a *App) syncHosted() {
	svc := cloud.Service()
	if version.Channel != "nightly" || !cloud.SignedIn(svc) {
		return
	}
	a.syncTeams(svc)
}

// syncTeams brings the account's teams up to date and tells the frontend.
// A team the account is no longer in stays, with its projects (nothing is
// changed in them): their watches stop, and the person is told, once.
func (a *App) syncTeams(svc string) error {
	me, err := cloud.SyncTeams(svc)
	if err != nil {
		return err
	}
	if len(me.Lost) > 0 {
		store, _ := teams.Load()
		var names []string
		for _, id := range me.Lost {
			for key, root := range store.Projects {
				if strings.HasPrefix(key, id+"/") {
					a.stopWatch(root)
				}
			}
			if t := store.Find(id); t != nil {
				names = append(names, t.Name)
			}
		}
		if a.notify != nil {
			a.notify("No access to a team", "The account signed in to R3V-Cloud isn't in "+strings.Join(names, ", ")+
				" (any more). Its projects stay on this computer as they are; remove the team to keep them as local projects.")
		}
	}
	if a.emit != nil {
		a.emit("teams", svc)
	}
	return nil
}

// CloudSignOut signs this computer out; hosted teams stay listed, signed out.
func (a *App) CloudSignOut() error { return cloud.SignOut(cloud.Service()) }

// selectFirstHosted makes a hosted team current when the current one isn't
// a team (a first sign-in).
func (a *App) selectFirstHosted(svc string) {
	teams.Update(func(s *teams.Store) error {
		if s.Find(s.Current) != nil {
			return nil
		}
		for _, t := range s.Teams {
			if h, ok := cloud.Hosted(t); ok && h == svc {
				s.Current = t.ID
				return nil
			}
		}
		return nil
	})
}

// CloudCreateTeam makes a hosted team (you its owner) and selects it.
func (a *App) CloudCreateTeam(name string) (TeamSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return TeamSummary{}, errors.New("a team name needs 1 to 100 characters")
	}
	addr, err := cloud.CreateTeam(cloud.Service(), name)
	if err != nil {
		return TeamSummary{}, err
	}
	return a.selectByAddress(addr)
}

// CloudJoin joins the team an invitation link is for and selects it.
func (a *App) CloudJoin(link string) (TeamSummary, error) {
	token, err := inviteToken(link)
	if err != nil {
		return TeamSummary{}, err
	}
	addr, err := cloud.Accept(cloud.Service(), token)
	if err != nil {
		return TeamSummary{}, err
	}
	return a.selectByAddress(addr)
}

func inviteToken(link string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	token := ""
	if err == nil {
		_, token, _ = strings.Cut(u.Path, "/invite/")
	}
	if token == "" || strings.Contains(token, "/") {
		return "", errors.New("that isn't an invitation link (it looks like …/invite/…)")
	}
	return token, nil
}

// CloudInvitation is what an invitation link is for, before accepting it:
// the team, and the people of a team that moved in to be claimed.
type CloudInvitation struct {
	Team   string            `json:"team"`
	People []cloud.Claimable `json:"people"`
}

// CloudInvitationInfo reads an invitation link (nothing accepted).
func (a *App) CloudInvitationInfo(link string) (*CloudInvitation, error) {
	token, err := inviteToken(link)
	if err != nil {
		return nil, err
	}
	team, ms, err := cloud.InvitationClaimables(cloud.Service(), token)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		ms = []cloud.Claimable{}
	}
	return &CloudInvitation{Team: team, People: ms}, nil
}

// CloudJoinAs joins the team an invitation link is for as who one was in
// it before it moved to R3V Cloud (claim: their member id; "" for someone
// new), and selects it.
func (a *App) CloudJoinAs(link, claim string) (TeamSummary, error) {
	token, err := inviteToken(link)
	if err != nil {
		return TeamSummary{}, err
	}
	addr, err := cloud.AcceptAs(cloud.Service(), token, claim)
	if err != nil {
		return TeamSummary{}, err
	}
	return a.selectByAddress(addr)
}

func (a *App) selectByAddress(addr string) (TeamSummary, error) {
	store, err := teams.Load()
	if err != nil {
		return TeamSummary{}, err
	}
	t := store.FindByURL(addr)
	if t == nil {
		return TeamSummary{}, errors.New("the team isn't in the list")
	}
	if err := a.SelectTeam(t.ID); err != nil {
		return TeamSummary{}, err
	}
	return teamSummary(*t), nil
}

// hosted finds a hosted team by its id here: the service and the team's id
// there.
func hosted(teamID string) (service, team string, err error) {
	store, err := teams.Load()
	if err != nil {
		return "", "", err
	}
	t := store.Find(teamID)
	if t == nil {
		return "", "", errors.New("unknown team")
	}
	svc, ok := cloud.Hosted(*t)
	if !ok {
		return "", "", errors.New("not an R3V-Cloud team")
	}
	return svc, cloud.TeamID(t.Remote.URL), nil
}

// CloudPeople is what a hosted team's settings show about its people.
type CloudPeople struct {
	MyRole      string             `json:"myRole"`
	MyUserID    string             `json:"myUserId"`
	Members     []cloud.Member     `json:"members"`
	Invitations []cloud.Invitation `json:"invitations"` // owners and admins only
	Projects    []cloud.Project    `json:"projects"`
}

// CloudPeople reads a hosted team's people, pending invitations and
// projects.
func (a *App) CloudPeople(teamID string) (CloudPeople, error) {
	svc, team, err := hosted(teamID)
	if err != nil {
		return CloudPeople{}, err
	}
	me, err := cloud.GetMe(svc)
	if err != nil {
		return CloudPeople{}, err
	}
	p := CloudPeople{MyUserID: me.User.ID, Invitations: []cloud.Invitation{}}
	for _, t := range me.Teams {
		if t.ID == team {
			p.MyRole = t.Role
		}
	}
	if p.Members, err = cloud.Members(svc, team); err != nil {
		return CloudPeople{}, err
	}
	if p.Projects, err = cloud.Projects(svc, team); err != nil {
		return CloudPeople{}, err
	}
	if p.MyRole == "owner" || p.MyRole == "admin" {
		if p.Invitations, err = cloud.Invitations(svc, team); err != nil {
			return CloudPeople{}, err
		}
	}
	return p, nil
}

// CloudInvite emails someone a link to join (collaborators: with the
// projects they reach, id -> "read" or "write").
func (a *App) CloudInvite(teamID, email, role string, projects map[string]string) error {
	svc, team, err := hosted(teamID)
	if err != nil {
		return err
	}
	return cloud.Invite(svc, team, strings.TrimSpace(email), role, projects)
}

func (a *App) CloudWithdraw(teamID, invitationID string) error {
	svc, team, err := hosted(teamID)
	if err != nil {
		return err
	}
	return cloud.Withdraw(svc, team, invitationID)
}

func (a *App) CloudSetRole(teamID, userID, role string) error {
	svc, team, err := hosted(teamID)
	if err != nil {
		return err
	}
	return cloud.SetRole(svc, team, userID, role)
}

// CloudRemoveMember takes someone out of the team.
func (a *App) CloudRemoveMember(teamID, userID string) error {
	svc, team, err := hosted(teamID)
	if err != nil {
		return err
	}
	return cloud.RemoveMember(svc, team, userID)
}

// CloudSetAccess gives a collaborator a project ("read", "write") or takes
// it away ("").
func (a *App) CloudSetAccess(teamID, projectID, userID, access string) error {
	svc, team, err := hosted(teamID)
	if err != nil {
		return err
	}
	return cloud.SetAccess(svc, team, projectID, userID, access)
}

// onRecord follows a record a hosted team's service wrote (live notices):
// the team's name or projects, the team list again; a member's look, the
// team's looks asked again; a lock, a branch's record or a milestone, the
// project's page reads again.
func (a *App) onRecord(service, team string, r cloud.Record) {
	switch r.Kind {
	case "team", "project":
		a.syncTeams(service)
		return
	case "member", "lock", "branch", "milestone":
	default:
		return // a kind this build doesn't know
	}
	store, err := teams.Load()
	if err != nil {
		return
	}
	t := store.FindByURL(cloud.TeamAddress(service, team))
	if t == nil {
		return
	}
	if r.Kind == "member" {
		looksCache.forget(t.Remote.URL)
	}
	if a.emit == nil {
		return
	}
	for key, root := range store.Projects {
		pid, ok := strings.CutPrefix(key, t.ID+"/")
		if ok && (r.Kind == "member" || pid == r.Project) {
			a.emit("team-watch", WatchEvent{Root: root, Kind: "record", Labels: []string{}, Versions: []Version{}})
		}
	}
}
