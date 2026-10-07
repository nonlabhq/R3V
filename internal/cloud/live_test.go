//go:build nightly

package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeLive hands out tickets and keeps the app's sockets; send reaches
// every socket open, drop closes them (as the edge sometimes does).
type fakeLive struct {
	*httptest.Server
	mu         sync.Mutex
	socks      []*websocket.Conn
	subscribed chan []string
	tickets    atomic.Int32
	mute       atomic.Bool // pings go unanswered (a connection gone silently)
}

func newFakeLive(t *testing.T) *fakeLive {
	f := &fakeLive{subscribed: make(chan []string, 16)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/teams/{team}/live", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		f.tickets.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"url": "ws" + strings.TrimPrefix(f.URL, "http") + "/socket"})
	})
	mux.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.socks = append(f.socks, ws)
		f.mu.Unlock()
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			if string(data) == "ping" {
				if f.mute.Load() {
					continue
				}
				ws.Write(context.Background(), websocket.MessageText, []byte("pong"))
				continue
			}
			var m struct{ Subscribe []string }
			if json.Unmarshal(data, &m) == nil && m.Subscribe != nil {
				f.subscribed <- m.Subscribe
			}
		}
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	return f
}

func (f *fakeLive) send(t *testing.T, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ws := range f.socks {
		ws.Write(context.Background(), websocket.MessageText, []byte(msg))
	}
}

func (f *fakeLive) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ws := range f.socks {
		ws.CloseNow()
	}
	f.socks = nil
}

func testHub() *Hub {
	h := NewHub()
	h.backoff, h.maxBackoff, h.ping = 10*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond
	return h
}

func waitFor(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("no nudge: %s", what)
	}
}

func quiet(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("nudged: %s", what)
	case <-time.After(150 * time.Millisecond):
	}
}

func drain(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func TestLiveNudgesTheProjectWhoseBranchMoved(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	defer h.Close()
	team := TeamAddress(f.URL, strings.Repeat("1", 32))
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 32)

	nudgeA, connected, stopA := h.Watch(team, a)
	defer stopA()
	waitFor(t, "catching up when connected", nudgeA)
	nudgeB, _, stopB := h.Watch(team, b)
	defer stopB()
	// One socket for the team, subscribed to both projects.
	deadline := time.After(3 * time.Second)
	for got := []string{}; len(got) < 2; {
		select {
		case got = <-f.subscribed:
		case <-deadline:
			t.Fatal("never subscribed to both projects")
		}
	}
	if !connected() || f.tickets.Load() != 1 {
		t.Fatalf("connected %v with %d tickets", connected(), f.tickets.Load())
	}
	drain(nudgeA)
	drain(nudgeB)

	f.send(t, `{"type":"key","project":"`+a+`","key":"projects/`+a+`/branches/main","etag":"\"e\""}`)
	waitFor(t, "a's branch moved", nudgeA)
	quiet(t, "b, for a's branch", nudgeB)
	f.send(t, `{"type":"key","project":"`+b+`","key":"projects/`+b+`/workspaces/w.json","etag":"\"e\""}`)
	quiet(t, "a workspace isn't a new version", nudgeB)
}

func TestLiveReconnectsAndCatchesUp(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	defer h.Close()
	nudge, connected, stop := h.Watch(TeamAddress(f.URL, strings.Repeat("1", 32)), strings.Repeat("a", 32))
	defer stop()
	waitFor(t, "first connection", nudge)
	<-f.subscribed

	f.drop()
	// Back on its own, subscribed again, and every project looked at.
	select {
	case <-f.subscribed:
	case <-time.After(3 * time.Second):
		t.Fatal("didn't reconnect")
	}
	waitFor(t, "catching up after reconnecting", nudge)
	if !connected() || f.tickets.Load() < 2 {
		t.Errorf("connected %v, %d tickets", connected(), f.tickets.Load())
	}
}

func TestLiveTellsAboutTeamChanges(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	changed := make(chan string, 4)
	h.OnTeamChange = func(service string) { changed <- service }
	defer h.Close()
	nudge, _, stop := h.Watch(TeamAddress(f.URL, strings.Repeat("1", 32)), strings.Repeat("a", 32))
	defer stop()
	waitFor(t, "connected", nudge)
	<-f.subscribed
	f.send(t, `{"type":"access"}`)
	select {
	case svc := <-changed:
		if svc != f.URL {
			t.Errorf("team change for %q", svc)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no team change")
	}
}

func TestLiveSignedOutAndNotHosted(t *testing.T) {
	f := newFakeLive(t)
	t.Setenv("R3V_CLOUD_TOKEN", "wrong")
	h := testHub()
	defer h.Close()
	nudge, connected, stop := h.Watch(TeamAddress(f.URL, strings.Repeat("1", 32)), strings.Repeat("a", 32))
	quiet(t, "signed out", nudge)
	if connected() {
		t.Error("connected while signed out")
	}
	stop()

	// A storage team: nothing live, nothing to stop.
	nudge, connected, stop = h.Watch("s3+https://storage.example/bucket/r3v", strings.Repeat("a", 32))
	quiet(t, "a storage team", nudge)
	if connected() {
		t.Error("a storage team is never live")
	}
	stop()
}

func TestLiveStopsWithTheLastWatch(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	team := TeamAddress(f.URL, strings.Repeat("1", 32))
	nudge, _, stop := h.Watch(team, strings.Repeat("a", 32))
	waitFor(t, "connected", nudge)
	stop()
	h.mu.Lock()
	left := len(h.conns)
	h.mu.Unlock()
	if left != 0 {
		t.Errorf("%d connections left after the last watch stopped", left)
	}
}

// A connection whose pings go unanswered is gone (sleep, a network change):
// R3V connects again rather than wait for notices that never come.
func TestLiveReconnectsWhenPingsGoUnanswered(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	defer h.Close()
	nudge, _, stop := h.Watch("r3v-cloud+"+f.URL+"/v1/teams/t1", "p1")
	defer stop()
	waitFor(t, "connected", nudge)
	f.mute.Store(true)
	deadline := time.Now().Add(3 * time.Second)
	for f.tickets.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("no new connection after pings went unanswered")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A watch added while the team's last other watch stops still gets its
// notices: the stop doesn't end the connection under it.
func TestLiveWatchWhileAnotherStops(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	defer h.Close()
	addr := "r3v-cloud+" + f.URL + "/v1/teams/t1"
	for i := 0; i < 50; i++ {
		_, _, stopA := h.Watch(addr, "pa")
		var wg sync.WaitGroup
		var stopB func()
		wg.Add(2)
		go func() { defer wg.Done(); stopA() }()
		go func() { defer wg.Done(); _, _, stopB = h.Watch(addr, "pb") }()
		wg.Wait()
		h.mu.Lock()
		c := h.conns[addr]
		h.mu.Unlock()
		if c == nil || len(c.projects()) != 1 {
			t.Fatalf("try %d: the watch left on no connection", i)
		}
		stopB()
	}
}

// A record the service wrote is passed on with its team, kinds this build
// doesn't know too (the app decides what to do with them).
func TestLivePassesRecordsOn(t *testing.T) {
	f := newFakeLive(t)
	h := testHub()
	got := make(chan Record, 4)
	team := strings.Repeat("1", 32)
	h.OnRecord = func(service, tm string, r Record) {
		if service != f.URL || tm != team {
			t.Errorf("record for %q %q", service, tm)
		}
		got <- r
	}
	defer h.Close()
	nudge, _, stop := h.Watch(TeamAddress(f.URL, team), strings.Repeat("a", 32))
	defer stop()
	waitFor(t, "connected", nudge)
	<-f.subscribed
	f.send(t, `{"type":"record","kind":"member","id":"m1","sum":"`+strings.Repeat("ab", 32)+`"}`)
	f.send(t, `{"type":"record","kind":"lock","id":"Song.als","project":"p1"}`)
	f.send(t, `{"type":"record","kind":"someday"}`)
	want := map[string]Record{
		"member":  {Kind: "member", ID: "m1", Sum: strings.Repeat("ab", 32)},
		"lock":    {Kind: "lock", ID: "Song.als", Project: "p1"},
		"someday": {Kind: "someday"},
	}
	for range want {
		select {
		case r := <-got:
			if r != want[r.Kind] {
				t.Errorf("record %+v, want %+v", r, want[r.Kind])
			}
		case <-time.After(3 * time.Second):
			t.Fatal("a record wasn't passed on")
		}
	}
}
