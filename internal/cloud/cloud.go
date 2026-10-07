// Package cloud is R3V's side of the hosted service, R3V-Cloud: signing in
// (the session kept in the system's credential store), the account's
// teams, and keeping them in the teams store as hosted teams, whose storage
// is reached through the broker (internal/remote).
package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/keyring"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
	"github.com/nonlabhq/r3v/internal/version"
)

// Default is the service R3V signs in to unless told otherwise; Nightly
// builds use the test service while R3V-Cloud is built (default_nightly.go).
var Default = "https://api.r3v.so"

// Service is the service to use: R3V_CLOUD_URL, or Default.
func Service() string {
	if s := os.Getenv("R3V_CLOUD_URL"); s != "" {
		return strings.TrimRight(s, "/")
	}
	return Default
}

func init() {
	keyring.Dir = teams.Dir()
	remote.SessionToken = func(service string) string {
		t, _ := Token(service)
		return t
	}
}

// target names a service's session in the credential store; a build with
// extensions keeps its own (version.Name).
func target(service string) string {
	u, err := url.Parse(service)
	host := service
	if err == nil && u.Host != "" {
		host = u.Host
	}
	return version.Name() + "/R3V-Cloud/" + strings.ToLower(host)
}

// Token is this computer's session with service; remote.ErrSignedOut when
// there is none. R3V_CLOUD_TOKEN stands in for it (tests).
func Token(service string) (string, error) {
	if t := os.Getenv("R3V_CLOUD_TOKEN"); t != "" {
		return t, nil
	}
	t, err := keyring.Get(target(service))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", remote.ErrSignedOut
	}
	return t, err
}

// SignedIn says whether this computer has a session with service.
func SignedIn(service string) bool {
	_, err := Token(service)
	return err == nil
}

var client = &http.Client{Timeout: 30 * time.Second}

// ErrNotInBuild: this build has no hosted teams (Stable; see
// remote.HostedTeams). Every way to the service stops here.
var ErrNotInBuild = errors.New("R3V-Cloud is in the Nightly build for now")

// invitation: an invitation's token in a path (kept out of errors).
var invitation = regexp.MustCompile(`/invitations/[^/?]+`)

// call sends a request to the service with the session; out may be nil.
func call(service, token, method, path string, in, out any) error {
	if !remote.HostedTeams {
		return ErrNotInBuild
	}
	var body io.Reader
	if in != nil {
		data, _ := json.Marshal(in)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, service+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("content-type", "application/json")
	}
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	// (an invitation's token is in its path: errors say the path without it)
	shown := invitation.ReplaceAllString(path, "/invitations/…")
	resp, err := client.Do(req)
	if err != nil {
		if ue, ok := err.(*url.Error); ok {
			c := *ue
			c.URL = service + shown
			return &c
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return remote.ErrSignedOut
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Error struct{ Code, Message string }
		}
		json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&e)
		if e.Error.Message != "" {
			return fmt.Errorf("R3V-Cloud: %s", e.Error.Message)
		}
		return fmt.Errorf("R3V-Cloud: %s %s: %d", method, shown, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Me is the signed-in person and their teams.
type Me struct {
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Teams []MyTeam `json:"teams"`
	// Lost: the teams (ids) SyncTeams found the account no longer in, just
	// now: marked NoAccess.
	Lost []string `json:"-"`
}

// MyTeam is a team the person is in.
type MyTeam struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"` // owner, admin, member, collaborator
	MemberID string `json:"memberId"`
	// Projects: a collaborator's projects (id -> "read" or "write").
	Projects map[string]string `json:"projects,omitempty"`
}

// GetMe asks the service who this computer is signed in as.
func GetMe(service string) (*Me, error) {
	tok, err := Token(service)
	if err != nil {
		return nil, err
	}
	var me Me
	if err := call(service, tok, "GET", "/v1/me", nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// TeamAddress is a hosted team's address, as the teams store keeps it.
func TeamAddress(service, team string) string {
	return "r3v-cloud+" + strings.TrimRight(service, "/") + "/v1/teams/" + team
}

// Hosted says whether a team is kept by a service, and which.
func Hosted(t teams.Team) (service string, ok bool) { return remote.BrokerService(t.Remote.URL) }

// SyncTeams puts the account's teams in the teams store (new ones added,
// names and member ids brought up to date) and marks NoAccess the service's
// teams the account isn't in (taken out, the team deleted, or another
// account signed in): they stay listed, their projects untouched, until
// the person removes them (Me.Lost: those just marked). Teams of other
// services, and storage teams, are left alone.
func SyncTeams(service string) (*Me, error) {
	me, err := GetMe(service)
	if err != nil {
		return nil, err
	}
	name := me.User.Name
	if name == "" {
		name, _, _ = strings.Cut(me.User.Email, "@")
	}
	var lost []string
	_, err = teams.Update(func(s *teams.Store) error {
		lost = nil
		keep := map[string]bool{}
		for _, mt := range me.Teams {
			addr := teams.NormalizeURL(TeamAddress(service, mt.ID))
			keep[addr] = true
			t := s.Upsert(remote.Config{URL: addr}, mt.Name)
			t.MemberID, t.MemberName, t.NoAccess = mt.MemberID, name, false
		}
		for i := range s.Teams {
			t := &s.Teams[i]
			if svc, ok := Hosted(*t); ok && strings.EqualFold(svc, service) && !keep[teams.NormalizeURL(t.Remote.URL)] {
				if !t.NoAccess {
					lost = append(lost, t.ID)
				}
				t.NoAccess = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	me.Lost = lost
	return me, nil
}

// SignOut ends this computer's session with service (on the service too,
// when it can be reached). Its teams stay listed, signed out.
func SignOut(service string) error {
	tok, err := keyring.Get(target(service))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	call(service, tok, "POST", "/v1/signout", nil, nil) // offline: the session lapses on its own
	return keyring.Delete(target(service))
}
