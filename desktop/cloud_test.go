//go:build nightly

package desktop

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
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

// Leaving a hosted team and keeping its projects: they move to Local (with
// what they need from the team's storage) before the team is left, since a
// team left no longer lets its storage be read.
func TestLeavingAHostedTeamKeepsItsProjectsFirst(t *testing.T) {
	var mu sync.Mutex
	var localWhenLeft []string
	left := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && strings.Contains(r.URL.Path, "/members/") {
			store, _ := teams.Load()
			mu.Lock()
			left, localWhenLeft = true, store.Local
			mu.Unlock()
		}
		if r.URL.Path == "/v1/me" {
			w.Write([]byte(`{"user":{"id":"u1","email":"yi@example.test"},"teams":[]}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	team := hostedTeam(t, srv.URL)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	root := newSong(t)
	r, err := project.Init(root, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := teams.Update(func(s *teams.Store) error {
		s.SetProjectRoot(team.ID, r.Config.ProjectID, r.Root)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().RemoveTeam(team.ID, true, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !left {
		t.Fatal("the team wasn't left on the service")
	}
	if !slices.Contains(localWhenLeft, r.Root) {
		t.Errorf("the team was left before its project was kept here: local %v", localWhenLeft)
	}
}

// A team the account isn't in any more: removing it doesn't ask the service
// (there's nothing to leave), and keeps its projects here as asked.
func TestRemovingATeamWithoutAccess(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	team := hostedTeam(t, srv.URL)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	root := newSong(t)
	r, err := project.Init(root, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := teams.Update(func(s *teams.Store) error {
		s.Find(team.ID).NoAccess = true
		s.SetProjectRoot(team.ID, r.Config.ProjectID, r.Root)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().RemoveTeam(team.ID, true, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("the service was asked: %v", calls)
	}
	if store, _ := teams.Load(); !slices.Contains(store.Local, r.Root) || store.Find(team.ID) != nil {
		t.Errorf("local %v, team still listed: %v", store.Local, store.Find(team.ID) != nil)
	}
}

// A hosted team is renamed on the service only, in one call: nothing else
// to write there that could fail halfway.
func TestRenamingAHostedTeam(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	team := hostedTeam(t, srv.URL)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	if err := NewApp().RenameTeamForEveryone(team.ID, "Trio"); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATCH /v1/teams/" + strings.Repeat("1", 32)}
	if !slices.Equal(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
	s, _ := teams.Load()
	if got := s.Find(team.ID); got == nil || got.Name != "Trio" {
		t.Errorf("team here: %+v", got)
	}
}

// Records a hosted team's service wrote: a member's look asks the team's
// looks again on its projects' pages; a lock reads its project's page again;
// a kind this build doesn't know does nothing.
func TestFollowingRecords(t *testing.T) {
	team := hostedTeam(t, "https://cloud.example")
	roots := map[string]string{"p1": t.TempDir(), "p2": t.TempDir()}
	if _, err := teams.Update(func(s *teams.Store) error {
		for pid, root := range roots {
			s.SetProjectRoot(team.ID, strings.Repeat(pid[1:], 32)[:32], root)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	var mu sync.Mutex
	var told []string
	a.emit = func(name string, data any) {
		if e, ok := data.(WatchEvent); ok && name == "team-watch" && e.Kind == "record" {
			mu.Lock()
			told = append(told, e.Root)
			mu.Unlock()
		}
	}
	looksCache.Lock()
	looksCache.byTeam[team.Remote.URL] = teamLooks{at: time.Now()}
	looksCache.Unlock()
	id := strings.Repeat("1", 32)

	a.onRecord("https://cloud.example", id, cloud.Record{Kind: "member", ID: "m1"})
	looksCache.Lock()
	_, cached := looksCache.byTeam[team.Remote.URL]
	looksCache.Unlock()
	if cached || len(told) != 2 {
		t.Errorf("member: looks still cached %v, pages told %v", cached, told)
	}
	told = nil
	a.onRecord("https://cloud.example", id, cloud.Record{Kind: "lock", ID: "Song.als", Project: strings.Repeat("2", 32)})
	if !slices.Equal(told, []string{roots["p2"]}) {
		t.Errorf("lock: pages told %v", told)
	}
	told = nil
	a.onRecord("https://cloud.example", id, cloud.Record{Kind: "someday"})
	if len(told) != 0 {
		t.Errorf("an unknown kind: pages told %v", told)
	}
}
