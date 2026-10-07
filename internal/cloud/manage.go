package cloud

import (
	"errors"
	"net/url"
	"strings"
)

// Managing a hosted team from the app: people and roles, invitations, who
// may reach which project (R3V-Cloud's docs/api.md). The service checks
// every call against the caller's role; these only carry them.

// TeamID is the team's id in a hosted team's address.
func TeamID(address string) string { return address[strings.LastIndex(address, "/")+1:] }

func authed(service string) (string, error) { return Token(service) }

// CreateTeam makes a team (the caller its owner) and puts it in the teams
// store; it returns the new team's address.
func CreateTeam(service, name string) (string, error) {
	tok, err := authed(service)
	if err != nil {
		return "", err
	}
	var t struct{ ID string }
	if err := call(service, tok, "POST", "/v1/teams", map[string]string{"name": name}, &t); err != nil {
		return "", err
	}
	if _, err := SyncTeams(service); err != nil {
		return "", err
	}
	return TeamAddress(service, t.ID), nil
}

// Member is a person in a hosted team.
type Member struct {
	UserID   string            `json:"userId"`
	MemberID string            `json:"memberId"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Role     string            `json:"role"`
	Projects map[string]string `json:"projects,omitempty"` // a collaborator's
}

func list[T any](service, path string) ([]T, error) {
	tok, err := authed(service)
	if err != nil {
		return nil, err
	}
	var out struct{ Items []T }
	if err := call(service, tok, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	if out.Items == nil {
		out.Items = []T{}
	}
	return out.Items, nil
}

func do(service, method, path string, in any) error {
	tok, err := authed(service)
	if err != nil {
		return err
	}
	return call(service, tok, method, path, in, nil)
}

func teamPath(team string) string { return "/v1/teams/" + url.PathEscape(team) }

// RenameTeam changes the team's name for everyone (owners and admins).
func RenameTeam(service, team, name string) error {
	return do(service, "PATCH", teamPath(team), map[string]string{"name": name})
}

// Leave takes the signed-in person out of the team.
func Leave(service, team string) error {
	me, err := GetMe(service)
	if err != nil {
		return err
	}
	return RemoveMember(service, team, me.User.ID)
}

// Members lists the team's people (a collaborator sees those of their
// projects).
func Members(service, team string) ([]Member, error) {
	return list[Member](service, teamPath(team)+"/members")
}

// SetRole makes someone owner, admin, member or collaborator.
func SetRole(service, team, user, role string) error {
	return do(service, "PATCH", teamPath(team)+"/members/"+url.PathEscape(user), map[string]string{"role": role})
}

// RemoveMember takes someone out of the team (or the caller leaves).
func RemoveMember(service, team, user string) error {
	return do(service, "DELETE", teamPath(team)+"/members/"+url.PathEscape(user), nil)
}

// Invitation is one not yet accepted.
type Invitation struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Role      string            `json:"role"`
	Projects  map[string]string `json:"projects,omitempty"`
	ExpiresAt int64             `json:"expiresAt"`
}

// Invitations lists those pending.
func Invitations(service, team string) ([]Invitation, error) {
	return list[Invitation](service, teamPath(team)+"/invitations")
}

// Invite emails someone a link to join with a role (and, for a
// collaborator, the projects they reach: id -> "read" or "write").
func Invite(service, team, email, role string, projects map[string]string) error {
	if role != "collaborator" {
		projects = nil
	} else if len(projects) == 0 {
		return errors.New("choose the projects a collaborator works on")
	}
	return do(service, "POST", teamPath(team)+"/invitations", map[string]any{"email": email, "role": role, "projects": projects})
}

// Withdraw cancels a pending invitation.
func Withdraw(service, team, id string) error {
	return do(service, "DELETE", teamPath(team)+"/invitations/"+url.PathEscape(id), nil)
}

// Project is a project of the team as the caller may reach it.
type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Access string `json:"access"` // read, write
}

// Projects lists the team's projects the caller may reach.
func Projects(service, team string) ([]Project, error) {
	return list[Project](service, teamPath(team)+"/projects")
}

// SetAccess gives a collaborator a project ("read" or "write"), or takes it
// away ("").
func SetAccess(service, team, pid, user, access string) error {
	p := teamPath(team) + "/projects/" + url.PathEscape(pid) + "/collaborators/" + url.PathEscape(user)
	if access == "" {
		return do(service, "DELETE", p, nil)
	}
	return do(service, "PUT", p, map[string]string{"access": access})
}

// Accept joins the team an invitation is for (token: the last part of its
// link) and brings the team list up to date; it returns the team's address.
func Accept(service, token string) (string, error) {
	tok, err := authed(service)
	if err != nil {
		return "", err
	}
	var out struct{ TeamID string }
	if err := call(service, tok, "POST", "/v1/invitations/"+url.PathEscape(token)+"/accept", nil, &out); err != nil {
		return "", err
	}
	if _, err := SyncTeams(service); err != nil {
		return "", err
	}
	return TeamAddress(service, out.TeamID), nil
}
