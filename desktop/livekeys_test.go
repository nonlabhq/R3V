package desktop

import (
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

type emitted struct {
	sync.Mutex
	keys    []TeamKeysEvent
	watches []WatchEvent
}

func (e *emitted) of(name string, data any) {
	e.Lock()
	defer e.Unlock()
	switch name {
	case "team-keys":
		e.keys = append(e.keys, data.(TeamKeysEvent))
	case "team-watch":
		e.watches = append(e.watches, data.(WatchEvent))
	}
}

func (e *emitted) take() ([]TeamKeysEvent, []WatchEvent) {
	time.Sleep(4 * liveKeysWait)
	e.Lock()
	defer e.Unlock()
	k, w := e.keys, e.watches
	e.keys, e.watches = nil, nil
	return k, w
}

// A hosted team's keys changing: the notices of a moment gathered into one
// event saying what to read again (its lists, its Activity), a project's
// branch or workspace reading its page again; keys nothing here shows,
// nothing. After a reconnect, everything.
func TestLiveTeamKeysInTheApp(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	was := liveKeysWait
	liveKeysWait = 20 * time.Millisecond
	t.Cleanup(func() { liveKeysWait = was })
	f := cloudtest.NewFake(t)
	a := NewApp()
	t.Cleanup(a.live.Close)
	var ev emitted
	a.emit = ev.of
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	teams.Update(func(s *teams.Store) error {
		s.Find(hosted.ID).MemberID = cloudtest.Owner
		return nil
	})
	root := newSong(t)
	tp, err := a.AddProjectToTeam(hosted.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	ev.take()
	key := func(project, k string) {
		a.onKey(f.Service, "t", cloud.KeyNotice{Project: project, Key: k, ETag: `"1"`})
	}

	key("", "archived.json")
	key("", "teamlog/20261019T101010.000000000Z-0a0b0c0d.json")
	keys, watches := ev.take()
	if len(keys) != 1 || !keys[0].Overview || !keys[0].Activity || keys[0].Team != hosted.ID || len(keys[0].Keys) != 2 || len(watches) != 0 {
		t.Fatalf("archived and the log: %+v %+v", keys, watches)
	}

	key("", "teamlog/20261019T101011.000000000Z-0a0b0c0e.json")
	if keys, _ := ev.take(); len(keys) != 1 || keys[0].Overview || !keys[0].Activity {
		t.Errorf("the log: %+v", keys)
	}
	key("", "team.json")
	key("", "pictures/team/abababab")
	if keys, _ := ev.take(); len(keys) != 1 || !keys[0].Overview || keys[0].Activity {
		t.Errorf("the team's look: %+v", keys)
	}

	key(tp.ID, "projects/"+tp.ID+"/workspaces/"+tp.ID+".json")
	keys, watches = ev.take()
	if len(keys) != 0 || len(watches) != 1 || watches[0].Root != root || watches[0].Kind != "record" {
		t.Errorf("a workspace: %+v %+v", keys, watches)
	}
	key(tp.ID, "projects/"+tp.ID+"/branches/main")
	key(tp.ID, "projects/"+tp.ID+"/branchlog/20261019T101011.000000000Z-0a0b0c0e.json")
	keys, watches = ev.take()
	if len(keys) != 1 || !keys[0].Activity || keys[0].Overview || len(watches) != 1 {
		t.Errorf("a branch moving: %+v %+v", keys, watches)
	}
	// Another team's project, nothing here; keys nothing shows.
	key("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "projects/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/workspaces/x.json")
	key("", "setups/"+cloudtest.Owner+".json")
	a.onKey(f.Service, "elsewhere", cloud.KeyNotice{Key: "archived.json"})
	if keys, watches := ev.take(); len(keys) != 0 || len(watches) != 0 {
		t.Errorf("nothing to read again: %+v %+v", keys, watches)
	}

	a.onKey(f.Service, "t", cloud.KeyNotice{})
	keys, watches = ev.take()
	if len(keys) != 1 || !keys[0].Overview || !keys[0].Activity || len(watches) != 1 || watches[0].Root != root {
		t.Errorf("after a reconnect: %+v %+v", keys, watches)
	}
}

// Every hosted team the account is in is listened to, a project of it here
// or not; one it isn't in any more, no longer.
func TestWatchHostedTeams(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	a := NewApp()
	t.Cleanup(a.live.Close)
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	listening := func() []string {
		a.mu.Lock()
		defer a.mu.Unlock()
		var out []string
		for url := range a.teamLive {
			out = append(out, url)
		}
		sort.Strings(out)
		return out
	}
	a.watchHostedTeams()
	a.watchHostedTeams()
	store, _ := teams.Load()
	if got := listening(); !reflect.DeepEqual(got, []string{store.Find(hosted.ID).Remote.URL}) {
		t.Fatalf("listening to %v", got)
	}
	teams.Update(func(s *teams.Store) error {
		s.Find(hosted.ID).NoAccess = true
		return nil
	})
	a.watchHostedTeams()
	if got := listening(); len(got) != 0 {
		t.Errorf("still listening to %v", got)
	}
}
