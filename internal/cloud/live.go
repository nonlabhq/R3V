package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Live notices: one WebSocket per hosted team, subscribed to the projects
// open on this computer. A branch moving wakes the watch of that project at
// once instead of at its next poll; after a reconnect every project is
// looked at (notices sent meanwhile are lost). Polling stays, slower, as
// the safety net.

// Hub keeps the live connections of this process, one per hosted team.
type Hub struct {
	// OnTeamChange is called (on its own goroutine) when a team's people,
	// roles or projects change, or the person's own access: read them again.
	OnTeamChange func(service string)

	mu    sync.Mutex
	conns map[string]*liveConn // by team address
	// Waits, for tests: first wait between reconnects, longest wait, ping.
	backoff, maxBackoff, ping time.Duration
	// steady: how long a connection lasts before it counts as working (one
	// the service takes and drops at once mustn't start the waiting over:
	// that would reconnect every second, for good)
	steady time.Duration
}

func NewHub() *Hub {
	return &Hub{conns: map[string]*liveConn{}, backoff: time.Second, maxBackoff: time.Minute, ping: 30 * time.Second,
		steady: 30 * time.Second}
}

// Watch subscribes to a hosted project's changes: nudge receives (never
// blocking the sender) when one of its branches moves, or after a
// reconnect; connected says whether notices are coming right now. stop
// ends the subscription. For a team that isn't hosted, nudge never fires
// and connected is always false.
func (h *Hub) Watch(teamAddress, pid string) (nudge <-chan struct{}, connected func() bool, stop func()) {
	ch := make(chan struct{}, 1)
	service, ok := remote.BrokerService(teamAddress)
	if !ok {
		return ch, func() bool { return false }, func() {}
	}
	h.mu.Lock()
	c := h.conns[teamAddress]
	if c == nil {
		team := teamAddress[strings.LastIndex(teamAddress, "/")+1:]
		c = &liveConn{hub: h, service: service, team: team, subs: map[string][]chan struct{}{}, resub: make(chan struct{}, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		c.cancel = cancel
		h.conns[teamAddress] = c
		go c.run(ctx)
	}
	// (added while the hub is held: a stop of the connection's last watch
	// meanwhile would otherwise end it with this one on it)
	c.add(pid, ch)
	h.mu.Unlock()
	return ch, c.isConnected, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if c.remove(pid, ch) && h.conns[teamAddress] == c {
			delete(h.conns, teamAddress)
			c.cancel()
		}
	}
}

// Close ends every connection.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, c := range h.conns {
		c.cancel()
		delete(h.conns, k)
	}
}

type liveConn struct {
	hub           *Hub
	service, team string
	cancel        context.CancelFunc
	resub         chan struct{} // the projects wanted changed

	mu        sync.Mutex
	subs      map[string][]chan struct{} // project -> watches
	connected bool
}

func (c *liveConn) add(pid string, ch chan struct{}) {
	c.mu.Lock()
	c.subs[pid] = append(c.subs[pid], ch)
	c.mu.Unlock()
	poke(c.resub)
}

// remove reports whether no watch is left.
func (c *liveConn) remove(pid string, ch chan struct{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.subs[pid][:0]
	for _, x := range c.subs[pid] {
		if x != ch {
			list = append(list, x)
		}
	}
	if len(list) == 0 {
		delete(c.subs, pid)
	} else {
		c.subs[pid] = list
	}
	return len(c.subs) == 0
}

func (c *liveConn) projects() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.subs))
	for pid := range c.subs {
		out = append(out, pid)
	}
	return out
}

func (c *liveConn) isConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *liveConn) setConnected(v bool) {
	c.mu.Lock()
	c.connected = v
	c.mu.Unlock()
}

// nudge wakes the watches of pid ("" for all).
func (c *liveConn) nudge(pid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for p, list := range c.subs {
		if pid == "" || p == pid {
			for _, ch := range list {
				poke(ch)
			}
		}
	}
}

func poke(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// run connects, listens and reconnects until ctx ends.
func (c *liveConn) run(ctx context.Context) {
	wait := c.hub.backoff
	for ctx.Err() == nil {
		began := time.Now()
		up, err := c.session(ctx)
		c.setConnected(false)
		if ctx.Err() != nil {
			return
		}
		if up && time.Since(began) >= c.hub.steady {
			wait = c.hub.backoff // it worked a while: start over
		}
		if errors.Is(err, remote.ErrSignedOut) {
			wait = c.hub.maxBackoff * 5 // until the person signs in again
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, c.hub.maxBackoff)
	}
}

type liveMsg struct {
	Type     string   `json:"type"`
	Project  string   `json:"project"`
	Key      string   `json:"key"`
	Projects []string `json:"projects"`
}

// session is one connection: it says whether it got connected.
func (c *liveConn) session(ctx context.Context) (bool, error) {
	tok, err := Token(c.service)
	if err != nil {
		return false, err
	}
	var t struct{ URL string }
	if err := call(c.service, tok, "POST", "/v1/teams/"+c.team+"/live", nil, &t); err != nil {
		return false, err
	}
	ws, _, err := websocket.Dial(ctx, t.URL, nil)
	if err != nil {
		return false, err
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 20)
	if err := c.subscribe(ctx, ws); err != nil {
		return false, err
	}
	c.setConnected(true)
	c.nudge("") // catch up on what happened while away

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var heard atomic.Int64 // when the service last said anything
	heard.Store(time.Now().UnixNano())
	go func() { // keep the connection alive; resubscribe when asked
		t := time.NewTicker(c.hub.ping)
		defer t.Stop()
		for {
			select {
			case <-sctx.Done():
				return
			case <-t.C:
				// The last ping unanswered: the connection is gone without a
				// word (sleep, a network change), so connect again.
				if time.Since(time.Unix(0, heard.Load())) > 2*c.hub.ping {
					cancel()
					return
				}
				if ws.Write(sctx, websocket.MessageText, []byte("ping")) != nil {
					cancel()
					return
				}
			case <-c.resub:
				if c.subscribe(sctx, ws) != nil {
					cancel()
					return
				}
			}
		}
	}()
	for {
		_, data, err := ws.Read(sctx)
		if err != nil {
			return true, err
		}
		heard.Store(time.Now().UnixNano())
		if string(data) == "pong" {
			continue
		}
		var m liveMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "key":
			if strings.Contains(m.Key, "/branches/") {
				c.nudge(m.Project)
			}
		case "team", "access":
			if f := c.hub.OnTeamChange; f != nil {
				go f(c.service)
			}
		}
	}
}

func (c *liveConn) subscribe(ctx context.Context, ws *websocket.Conn) error {
	data, _ := json.Marshal(map[string][]string{"subscribe": c.projects()})
	return ws.Write(ctx, websocket.MessageText, data)
}
