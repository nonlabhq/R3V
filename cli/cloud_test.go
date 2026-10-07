//go:build nightly

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/teams"
)

// `r3v teams` brings the hosted teams of the service this computer is
// signed in to up to date, and says when it's signed out.
func TestTeamsSyncsHostedTeams(t *testing.T) {
	team := strings.Repeat("1", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me" || r.Header.Get("authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"user":  map[string]string{"id": "u1", "email": "yi@example.test", "name": "Yi"},
			"teams": []map[string]string{{"id": team, "name": "Band", "role": "member", "memberId": strings.Repeat("a", 32)}},
		})
	}))
	defer srv.Close()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Setenv("R3V_CLOUD_URL", srv.URL)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")

	if code := Run([]string{"teams"}); code != 0 {
		t.Fatalf("r3v teams: exit %d", code)
	}
	store, _ := teams.Load()
	band := store.FindByURL(cloud.TeamAddress(srv.URL, team))
	if band == nil || band.Name != "Band" || band.MemberID != strings.Repeat("a", 32) || band.MemberName != "Yi" {
		t.Fatalf("Band in the store: %+v", band)
	}

	// Signed out: the list stays as it was.
	t.Setenv("R3V_CLOUD_TOKEN", "")
	if code := Run([]string{"teams"}); code != 0 {
		t.Fatalf("r3v teams signed out: exit %d", code)
	}
	if store, _ = teams.Load(); len(store.Teams) != 1 {
		t.Errorf("teams when signed out: %+v", store.Teams)
	}
}
