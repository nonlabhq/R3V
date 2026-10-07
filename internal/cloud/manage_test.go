package cloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nonlabhq/r3v/internal/teams"
)

// A fake service that records what it is asked and answers like the real
// one's shapes.
type fakeManage struct {
	*httptest.Server
	mu    sync.Mutex
	calls []string
	teams []MyTeam
}

func newFakeManage(t *testing.T) *fakeManage {
	f := &fakeManage{}
	team := strings.Repeat("1", 32)
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path+" "+strings.TrimSpace(string(body)))
		f.mu.Unlock()
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/teams":
			f.mu.Lock()
			f.teams = append(f.teams, MyTeam{ID: team, Name: "New band", Role: "owner", MemberID: strings.Repeat("a", 32)})
			f.mu.Unlock()
			w.Write([]byte(`{"id":"` + team + `","name":"New band"}`))
		case r.URL.Path == "/v1/me":
			f.mu.Lock()
			defer f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"id": "u1", "email": "yi@example.test"}, "teams": f.teams})
		case strings.HasSuffix(r.URL.Path, "/members") && r.Method == "GET":
			w.Write([]byte(`{"items":[{"userId":"u1","memberId":"` + strings.Repeat("a", 32) + `","name":"Yi","email":"yi@example.test","role":"owner"}],"more":false}`))
		case strings.HasSuffix(r.URL.Path, "/invitations") && r.Method == "GET":
			w.Write([]byte(`{"items":[],"more":false}`))
		case strings.HasSuffix(r.URL.Path, "/projects") && r.Method == "GET":
			w.Write([]byte(`{"items":[{"id":"p1","name":"Song","access":"write"}],"more":false}`))
		case strings.Contains(r.URL.Path, "/accept"):
			f.mu.Lock()
			f.teams = append(f.teams, MyTeam{ID: strings.Repeat("2", 32), Name: "Label", Role: "member", MemberID: strings.Repeat("b", 32)})
			f.mu.Unlock()
			w.Write([]byte(`{"teamId":"` + strings.Repeat("2", 32) + `","role":"member"}`))
		case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/members/"):
			w.WriteHeader(403)
			w.Write([]byte(`{"error":{"code":"forbidden","message":"Only owners change owners."}}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(f.Close)
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	return f
}

func TestManagingATeam(t *testing.T) {
	f := newFakeManage(t)
	addr, err := CreateTeam(f.URL, "New band")
	if err != nil {
		t.Fatal(err)
	}
	team := TeamID(addr)
	if store, _ := teams.Load(); store.FindByURL(addr) == nil {
		t.Fatal("the new team isn't in the team list")
	}
	ms, err := Members(f.URL, team)
	if err != nil || len(ms) != 1 || ms[0].Role != "owner" {
		t.Fatalf("members %+v %v", ms, err)
	}
	if inv, err := Invitations(f.URL, team); err != nil || inv == nil || len(inv) != 0 {
		t.Fatalf("invitations %+v %v", inv, err)
	}
	if ps, err := Projects(f.URL, team); err != nil || len(ps) != 1 || ps[0].Name != "Song" {
		t.Fatalf("projects %+v %v", ps, err)
	}
	if err := Invite(f.URL, team, "alex@example.test", "collaborator", nil); err == nil {
		t.Error("a collaborator was invited to no project")
	}
	if err := Invite(f.URL, team, "alex@example.test", "collaborator", map[string]string{"p1": "write"}); err != nil {
		t.Fatal(err)
	}
	if err := Invite(f.URL, team, "sam@example.test", "member", map[string]string{"p1": "write"}); err != nil {
		t.Fatal(err)
	}
	if err := SetAccess(f.URL, team, "p1", "u2", "read"); err != nil {
		t.Fatal(err)
	}
	if err := SetAccess(f.URL, team, "p1", "u2", ""); err != nil {
		t.Fatal(err)
	}
	if err := Withdraw(f.URL, team, "i1"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveMember(f.URL, team, "u2"); err != nil {
		t.Fatal(err)
	}
	// The service's own words come through.
	if err := SetRole(f.URL, team, "u1", "member"); err == nil || !strings.Contains(err.Error(), "Only owners change owners.") {
		t.Errorf("SetRole: %v", err)
	}
	joined, err := Accept(f.URL, "the-token")
	if err != nil || TeamID(joined) != strings.Repeat("2", 32) {
		t.Fatalf("accept: %q %v", joined, err)
	}
	if store, _ := teams.Load(); store.FindByURL(joined) == nil {
		t.Error("the joined team isn't in the team list")
	}

	want := []string{
		"POST /v1/teams/" + team + "/invitations " + `{"email":"alex@example.test","projects":{"p1":"write"},"role":"collaborator"}`,
		"POST /v1/teams/" + team + "/invitations " + `{"email":"sam@example.test","projects":null,"role":"member"}`,
		"PUT /v1/teams/" + team + "/projects/p1/collaborators/u2 " + `{"access":"read"}`,
		"DELETE /v1/teams/" + team + "/projects/p1/collaborators/u2 ",
		"DELETE /v1/teams/" + team + "/invitations/i1 ",
		"DELETE /v1/teams/" + team + "/members/u2 ",
		"POST /v1/invitations/the-token/accept ",
	}
	got := strings.Join(f.calls, "\n")
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("no call %q in\n%s", w, got)
		}
	}
}
