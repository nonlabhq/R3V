package desktop

import (
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

// At start a signed-in app, Stable as well as Nightly, brings the account's
// teams up to date: a team joined meanwhile (on the web, say) appears, and
// one it lost access to is marked so, said once.
func TestStartupSyncsHostedTeams(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t) // signed in as the team's owner
	t.Setenv("R3V_CLOUD_URL", f.Service)
	gone := cloud.TeamAddress(f.Service, strings.Repeat("1", 32))
	var goneID string
	teams.Update(func(s *teams.Store) error {
		tm := s.Upsert(remote.Config{URL: gone}, "Old band")
		tm.MemberID, goneID = strings.Repeat("a", 32), tm.ID
		return nil
	})
	a := NewApp()
	t.Cleanup(a.live.Close)
	var told []string
	a.notify = func(title, _ string) { told = append(told, title) }
	var emitted []string
	a.emit = func(name string, _ any) { emitted = append(emitted, name) }
	if s := a.CloudStatus(); !s.Available || !s.SignedIn {
		t.Fatalf("cloud status: %+v", s)
	}

	a.syncHosted()
	store, _ := teams.Load()
	var joined *teams.Team
	for i := range store.Teams {
		if teams.NormalizeURL(store.Teams[i].Remote.URL) == teams.NormalizeURL(f.Address) {
			joined = &store.Teams[i]
		}
	}
	if joined == nil || joined.MemberID != cloudtest.Owner || joined.NoAccess {
		t.Fatalf("the team joined meanwhile: %+v (teams %+v)", joined, store.Teams)
	}
	if lost := store.Find(goneID); lost == nil || !lost.NoAccess {
		t.Errorf("the team without access: %+v", lost)
	}
	if len(told) != 1 || told[0] != "No access to a team" {
		t.Errorf("told %v", told)
	}
	if len(emitted) == 0 || emitted[len(emitted)-1] != "teams" {
		t.Errorf("emitted %v", emitted)
	}

	a.syncHosted() // said once
	if len(told) != 1 {
		t.Errorf("told again: %v", told)
	}
}

// Signed out, the start asks the service nothing.
func TestStartupSignedOutSyncsNothing(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	t.Setenv("R3V_CLOUD_URL", f.Service)
	t.Setenv("R3V_CLOUD_TOKEN", "")
	a := NewApp()
	t.Cleanup(a.live.Close)
	a.syncHosted()
	if store, _ := teams.Load(); len(store.Teams) != 0 {
		t.Errorf("teams %+v", store.Teams)
	}
}
