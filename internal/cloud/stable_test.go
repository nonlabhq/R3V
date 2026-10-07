//go:build !nightly

package cloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Stable has no hosted teams: their addresses are no team's, and nothing
// reaches the service, even with what Nightly left (both channels share the
// teams and the session).
func TestStableReachesNoService(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	if _, ok := remote.BrokerService("r3v-cloud+" + srv.URL + "/v1/teams/t1"); ok {
		t.Error("a hosted team's address is a team's in Stable")
	}
	if _, ok := Hosted(teams.Team{Remote: remote.Config{URL: "r3v-cloud+" + srv.URL + "/v1/teams/t1"}}); ok {
		t.Error("a team is hosted in Stable")
	}
	if err := call(srv.URL, "a-session", "GET", "/v1/me", nil, nil); !errors.Is(err, ErrNotInBuild) {
		t.Errorf("call: %v", err)
	}
	if _, err := SignIn(context.Background(), srv.URL, func(string) error { return nil }); !errors.Is(err, ErrNotInBuild) {
		t.Errorf("sign in: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the service was reached %d time(s)", n)
	}
}
