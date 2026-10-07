package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

func hostedTeam(t *testing.T, service string) teams.Team {
	t.Helper()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	var team teams.Team
	if _, err := teams.Update(func(s *teams.Store) error {
		tm := s.Upsert(remote.Config{URL: cloud.TeamAddress(service, strings.Repeat("1", 32))}, "Band")
		tm.MemberID = strings.Repeat("a", 32)
		team = *tm
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return team
}

func TestTeamSummaryMarksHostedTeams(t *testing.T) {
	team := hostedTeam(t, "https://cloud.example")
	t.Setenv("R3V_CLOUD_TOKEN", "")
	if s := teamSummary(team); !s.Hosted || !s.SignedOut || s.IsStorage {
		t.Errorf("signed out: %+v", s)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	if s := teamSummary(team); !s.Hosted || s.SignedOut {
		t.Errorf("signed in: %+v", s)
	}
	storage := teams.Team{Remote: remote.Config{URL: "s3+https://storage.example/bucket/r3v"}}
	if s := teamSummary(storage); s.Hosted || s.SignedOut {
		t.Errorf("a storage team: %+v", s)
	}
}

func TestCloudJoinNeedsAnInvitationLink(t *testing.T) {
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	for _, link := range []string{"", "hello", "https://r3v.so/teams/x", "https://r3v.so/invite/"} {
		if _, err := NewApp().CloudJoin(link); err == nil || !strings.Contains(err.Error(), "invitation link") {
			t.Errorf("%q: %v", link, err)
		}
	}
}

// Removing a hosted team leaves it on the service; otherwise the next look
// at the account's teams would bring it back.
func TestRemovingAHostedTeamLeavesIt(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/v1/me" {
			w.Write([]byte(`{"user":{"id":"u1","email":"yi@example.test"},"teams":[]}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	team := hostedTeam(t, srv.URL)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	if err := NewApp().RemoveTeam(team.ID, false, false); err != nil {
		t.Fatal(err)
	}
	want := "DELETE /v1/teams/" + strings.Repeat("1", 32) + "/members/u1"
	if !strings.Contains(strings.Join(calls, "\n"), want) {
		t.Errorf("calls %v, want %q", calls, want)
	}
	if store, _ := teams.Load(); store.Find(team.ID) != nil {
		t.Error("the team is still listed")
	}
}
