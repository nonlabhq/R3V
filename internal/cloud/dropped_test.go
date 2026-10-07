//go:build nightly

package cloud

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Taken out of a team (or the team deleted): its projects here stay, as
// local projects that no longer point at the team, instead of vanishing
// from the list.
func TestSyncTeamsKeepsADroppedTeamsProjects(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"user":{"id":"u1","email":"yi@example.test"},"teams":[]}`)) // in no team now
	}))
	defer srv.Close()
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	addr := TeamAddress(srv.URL, strings.Repeat("1", 32))
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

	me, err := SyncTeams(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(me.Dropped, []string{r.Root}) {
		t.Errorf("dropped %v", me.Dropped)
	}
	store, _ := teams.Load()
	if store.Find(teamID) != nil {
		t.Error("the team is still listed")
	}
	if !slices.Contains(store.Local, r.Root) {
		t.Errorf("the project isn't kept as a local one: %v", store.Local)
	}
	if again, _ := project.Open(r.Root); again.Config.Remote != nil {
		t.Errorf("the project still points at the team: %v", again.Config.Remote.URL)
	}
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
