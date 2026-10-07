//go:build nightly

package cloud

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team the account isn't in (taken out, the team deleted, or another
// account signed in) stays, with its projects untouched, marked NoAccess:
// said once, and cleared once the account is in it again. Nothing in the
// projects changes: the person decides (removing the team).
func TestSyncTeamsMarksATeamWithoutAccess(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	tid := strings.Repeat("1", 32)
	var in atomic.Bool // is the account in the team?
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		teams := `[]`
		if in.Load() {
			teams = `[{"id":"` + tid + `","name":"Band","memberId":"m1"}]`
		}
		w.Write([]byte(`{"user":{"id":"u1","email":"yi@example.test"},"teams":` + teams + `}`))
	}))
	defer srv.Close()
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	addr := TeamAddress(srv.URL, tid)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Song.als"), []byte("set"), 0o644)
	r, err := project.Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	r.Config.Remote = &project.RemoteConfig{URL: addr}
	if err := r.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	var teamID string
	if _, err := teams.Update(func(s *teams.Store) error {
		tm := s.Upsert(remote.Config{URL: addr}, "Band")
		teamID = tm.ID
		s.SetProjectRoot(tm.ID, r.Config.ProjectID, r.Root)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check := func(what string, noAccess bool) {
		t.Helper()
		store, _ := teams.Load()
		tm := store.Find(teamID)
		if tm == nil || tm.NoAccess != noAccess {
			t.Fatalf("%s: team %+v", what, tm)
		}
		if len(store.Projects) != 1 || len(store.Local) != 0 {
			t.Errorf("%s: projects %v, local %v", what, store.Projects, store.Local)
		}
		if again, _ := project.Open(r.Root); again.Config.Remote == nil || again.Config.Remote.URL != addr {
			t.Errorf("%s: the project's team changed", what)
		}
	}

	me, err := SyncTeams(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(me.Lost, []string{teamID}) {
		t.Errorf("lost %v", me.Lost)
	}
	check("not in it", true)
	if me, _ := SyncTeams(srv.URL); len(me.Lost) != 0 {
		t.Errorf("said again: %v", me.Lost)
	}
	in.Store(true)
	if _, err := SyncTeams(srv.URL); err != nil {
		t.Fatal(err)
	}
	check("in it again", false)
}

// An invitation's token is in its path: an error doesn't show it.
func TestErrorsKeepInvitationsOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "oops", http.StatusInternalServerError) // not the service's JSON
	}))
	defer srv.Close()
	err := call(srv.URL, "tok", "POST", "/v1/invitations/s3cr3t-token/accept", nil, nil)
	if err == nil || strings.Contains(err.Error(), "s3cr3t-token") {
		t.Errorf("error %v", err)
	}
	srv.Close() // and when the service can't be reached
	err = call(srv.URL, "tok", "POST", "/v1/invitations/s3cr3t-token/accept", nil, nil)
	if err == nil || strings.Contains(err.Error(), "s3cr3t-token") {
		t.Errorf("error %v", err)
	}
}
