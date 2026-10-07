//go:build nightly

package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/keyring"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// fakeService answers the calls R3V makes; its teams can change between
// calls.
type fakeService struct {
	*httptest.Server
	mu        sync.Mutex
	challenge string
	code      string
	teams     []MyTeam
	signedOut []string
}

const fakeToken = "session-token-1"

func newFakeService(t *testing.T) *fakeService {
	f := &fakeService{code: "one-time-code"}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/desktop/token", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Code, Verifier string }
		json.NewDecoder(r.Body).Decode(&b)
		sum := sha256.Sum256([]byte(b.Verifier))
		f.mu.Lock()
		ok := b.Code == f.code && base64.RawURLEncoding.EncodeToString(sum[:]) == f.challenge
		f.code = "" // good once
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"code":"bad_request","message":"This sign-in code doesn't work: sign in again."}}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"token": fakeToken, "expiresAt": time.Now().Add(90 * 24 * time.Hour)})
	})
	mux.HandleFunc("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer "+fakeToken {
			w.WriteHeader(401)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"user":  map[string]string{"id": "u1", "email": "yi@example.test", "name": ""},
			"teams": f.teams,
		})
	})
	mux.HandleFunc("POST /v1/signout", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.signedOut = append(f.signedOut, r.Header.Get("authorization"))
		f.mu.Unlock()
		w.Write([]byte(`{"ok":true}`))
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	t.Cleanup(func() { keyring.Delete(target(f.URL)) })
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Setenv("R3V_CLOUD_TOKEN", "")
	return f
}

// browser plays the person: it checks the address R3V opens, keeps the
// challenge (the service would), then goes to the app's loopback address.
func (f *fakeService) browser(t *testing.T, state string) func(string) error {
	return func(addr string) error {
		u, err := url.Parse(addr)
		if err != nil || u.Path != "/desktop/start" || !strings.HasPrefix(addr, f.URL) {
			t.Errorf("opened %q", addr)
		}
		q := u.Query()
		f.mu.Lock()
		f.challenge = q.Get("challenge")
		f.mu.Unlock()
		if state == "" {
			state = q.Get("state")
		}
		go func() {
			cb := "http://127.0.0.1:" + q.Get("port") + "/callback?code=" + f.code + "&state=" + url.QueryEscape(state)
			if resp, err := http.Get(cb); err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
}

func TestSignInKeepsTheSessionAndSyncsTeams(t *testing.T) {
	f := newFakeService(t)
	f.teams = []MyTeam{
		{ID: strings.Repeat("1", 32), Name: "Band", Role: "owner", MemberID: strings.Repeat("a", 32)},
		{ID: strings.Repeat("2", 32), Name: "Label", Role: "collaborator", MemberID: strings.Repeat("b", 32),
			Projects: map[string]string{strings.Repeat("c", 32): "write"}},
	}
	if SignedIn(f.URL) {
		t.Fatal("signed in before signing in")
	}
	me, err := SignIn(context.Background(), f.URL, f.browser(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if me.User.Email != "yi@example.test" || len(me.Teams) != 2 {
		t.Errorf("me = %+v", me)
	}
	if tok, err := Token(f.URL); err != nil || tok != fakeToken {
		t.Errorf("token %q, %v", tok, err)
	}
	if remote.SessionToken(f.URL) != fakeToken {
		t.Error("the broker doesn't find the session")
	}

	// A storage team stays as it is through every sync.
	if _, err := teams.Update(func(s *teams.Store) error {
		s.Upsert(remote.Config{URL: "s3+https://storage.example/bucket/r3v", AccessKey: "k", SecretKey: "s"}, "Own storage")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncTeams(f.URL); err != nil {
		t.Fatal(err)
	}
	store, _ := teams.Load()
	band := store.FindByURL(TeamAddress(f.URL, strings.Repeat("1", 32)))
	if band == nil || band.Name != "Band" || band.MemberID != strings.Repeat("a", 32) || band.MemberName != "yi" {
		t.Fatalf("Band in the store: %+v", band)
	}
	if band.Remote.AccessKey != "" || band.Remote.SecretKey != "" {
		t.Error("a hosted team keeps no keys in the store")
	}
	if svc, ok := Hosted(*band); !ok || svc != f.URL {
		t.Errorf("Hosted = %q %v", svc, ok)
	}
	if len(store.Teams) != 3 {
		t.Errorf("%d teams, want 3", len(store.Teams))
	}

	// Taken out of Label: it goes; Band and the storage team stay.
	f.mu.Lock()
	f.teams = f.teams[:1]
	f.mu.Unlock()
	if _, err := SyncTeams(f.URL); err != nil {
		t.Fatal(err)
	}
	store, _ = teams.Load()
	if len(store.Teams) != 2 || store.FindByURL(TeamAddress(f.URL, strings.Repeat("2", 32))) != nil ||
		store.FindByURL("s3+https://storage.example/bucket/r3v") == nil {
		t.Errorf("teams after leaving Label: %+v", store.Teams)
	}

	// Signing out ends the session here and there; the teams stay listed.
	if err := SignOut(f.URL); err != nil {
		t.Fatal(err)
	}
	if len(f.signedOut) != 1 || f.signedOut[0] != "Bearer "+fakeToken {
		t.Errorf("the service was told %v", f.signedOut)
	}
	if _, err := Token(f.URL); !errors.Is(err, remote.ErrSignedOut) {
		t.Errorf("token after signing out: %v", err)
	}
	if _, err := SyncTeams(f.URL); !errors.Is(err, remote.ErrSignedOut) {
		t.Errorf("sync when signed out: %v", err)
	}
	if store, _ = teams.Load(); len(store.Teams) != 2 {
		t.Errorf("teams after signing out: %d", len(store.Teams))
	}
}

func TestSignInIgnoresAForeignCallback(t *testing.T) {
	f := newFakeService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// A callback with another state (not started here) doesn't sign in.
	_, err := SignIn(ctx, f.URL, f.browser(t, "someone-elses-state"))
	if err == nil || SignedIn(f.URL) {
		t.Errorf("signed in from a callback with the wrong state: %v", err)
	}
}

func TestSignInRefusedCode(t *testing.T) {
	f := newFakeService(t)
	f.code = "" // the service won't take any code
	_, err := SignIn(context.Background(), f.URL, func(addr string) error {
		u, _ := url.Parse(addr)
		go http.Get("http://127.0.0.1:" + u.Query().Get("port") + "/callback?code=x&state=" + url.QueryEscape(u.Query().Get("state")))
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "sign in again") || SignedIn(f.URL) {
		t.Errorf("a refused code: %v", err)
	}
}

func TestTeamAddressIsTheBrokers(t *testing.T) {
	addr := TeamAddress("https://api.r3v.so/", strings.Repeat("1", 32))
	if addr != "r3v-cloud+https://api.r3v.so/v1/teams/"+strings.Repeat("1", 32) {
		t.Errorf("address %q", addr)
	}
	if svc, ok := remote.BrokerService(addr); !ok || svc != "https://api.r3v.so" {
		t.Errorf("service %q %v", svc, ok)
	}
}
