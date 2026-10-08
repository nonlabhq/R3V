//go:build nightly

package desktop

import (
	"slices"
	"sync"
	"testing"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

const kaiID = "0000000000000000000000000000000b"

type heldEvents struct {
	sync.Mutex
	got []any
}

func (e *heldEvents) of(name string, data any) {
	e.Lock()
	defer e.Unlock()
	if name == "lock-held" {
		e.got = append(e.got, data)
	}
}

func (e *heldEvents) held() []HeldEvent {
	e.Lock()
	defer e.Unlock()
	var out []HeldEvent
	for _, d := range e.got {
		out = append(out, d.(HeldEvent))
	}
	return out
}

// The app with a hosted team's file locks: offered once for a project that
// benefits; nothing while off; on, a changed file of a locked kind is
// locked by itself, one someone else holds is told of at once and its
// share refused (the version kept), offline changes wait to be locked and
// are locked once back, a share frees your locks, discarding a change
// frees its lock.
func TestAppFileLocks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	f := cloudtest.NewFake(t)
	f.AddMember("kai", kaiID, false)
	a := NewApp()
	var ev heldEvents
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
	writeFile(t, root, "Maps/Harbor.umap", "0")
	writeFile(t, root, "Art/ship.blend", "0")
	tp, err := a.AddProjectToTeam(hosted.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}

	// Off: offered (a Blender file; you the owner), nothing shows.
	part, err := a.TeamState(root)
	if err != nil || !part.LocksOffer || part.Capabilities.Locks {
		t.Fatalf("team state before: offer %v locks %v (%v)", part.LocksOffer, part.Capabilities.Locks, err)
	}
	if v, _ := a.ProjectLocks(root); v.On {
		t.Error("locks shown while off")
	}
	if _, err := a.LockFiles(root, []string{"Maps/Harbor.umap"}); err == nil {
		t.Error("locked while off")
	}
	tl, _ := a.TeamLocks(hosted.ID)
	if !tl.Available || tl.On || !tl.Admin || !slices.Equal(tl.Kinds, profile.DefaultLockKinds()) {
		t.Errorf("team locks: %+v", tl)
	}

	if err := a.SetTeamLocks(hosted.ID, true, profile.DefaultLockKinds()); err != nil {
		t.Fatal(err)
	}
	if part, _ := a.TeamState(root); part.LocksOffer || !part.Capabilities.Locks {
		t.Errorf("team state after: offer %v locks %v", part.LocksOffer, part.Capabilities.Locks)
	}
	v, _ := a.ProjectLocks(root)
	if !v.On || !v.Admin || !v.AutoLock || v.Me != cloudtest.Owner {
		t.Fatalf("locks view: %+v", v)
	}

	// Kai takes the map.
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	if _, err := cloud.SetLocks(f.Address, tp.ID, "ws-k", []string{"Maps/Harbor.umap"}, nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("R3V_CLOUD_TOKEN", "token")

	// Changing it: told at once, once; the ship locked by itself.
	writeFile(t, root, "Maps/Harbor.umap", "yi")
	writeFile(t, root, "Art/ship.blend", "yi")
	a.autoLock(root, []string{"Maps/Harbor.umap", "Art/ship.blend"})
	a.autoLock(root, []string{"Maps/Harbor.umap"})
	if h := ev.held(); len(h) != 1 || h[0].File != "Harbor.umap" || h[0].Path != "Maps/Harbor.umap" {
		t.Errorf("told: %+v", h)
	}
	if got := f.Locks(tp.ID); got["Art/ship.blend"] != cloudtest.Owner || got["Maps/Harbor.umap"] != kaiID {
		t.Errorf("locks: %v", got)
	}
	v, _ = a.ProjectLocks(root)
	if len(v.Items) != 2 || !slices.ContainsFunc(v.Items, func(l LockItem) bool { return l.Mine && l.Path == "Art/ship.blend" }) {
		t.Errorf("locks view: %+v", v.Items)
	}

	// The commit's share is refused: who holds what; the version stays.
	res, err := a.Save(root, "harbor and ship", true, nil, true, nil)
	if err != nil || res.Action != "locked" || len(res.Locks) != 1 || res.Locks[0].MemberID != kaiID {
		t.Fatalf("save: %+v %v", res, err)
	}
	st, _ := a.State(root)
	if len(st.Changes) != 0 {
		t.Errorf("the changes weren't committed: %v", st.Changes)
	}

	// Kai lets go; sharing frees the ship's lock.
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	cloud.SetLocks(f.Address, tp.ID, "ws-k", nil, []string{"Maps/Harbor.umap"})
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	if res, err := a.ShareVersions(root); err != nil || res.Action != "published" {
		t.Fatalf("share: %+v %v", res, err)
	}
	if got := f.Locks(tp.ID); len(got) != 0 {
		t.Errorf("locks after the share: %v", got)
	}

	// Offline: the change waits to be locked, then is locked once back.
	f.Down.Store(true)
	writeFile(t, root, "Art/ship.blend", "offline")
	a.autoLock(root, []string{"Art/ship.blend"})
	if v, _ := a.ProjectLocks(root); !slices.Equal(v.Waiting, []string{"Art/ship.blend"}) || v.Offline == "" {
		t.Errorf("waiting: %+v", v)
	}
	f.Down.Store(false)
	a.autoLock(root, nil)
	if got := f.Locks(tp.ID); got["Art/ship.blend"] != cloudtest.Owner {
		t.Errorf("locked once back: %v", got)
	}
	if v, _ := a.ProjectLocks(root); len(v.Waiting) != 0 {
		t.Errorf("still waiting: %v", v.Waiting)
	}

	// Discarding the change frees its lock.
	if _, err := a.DiscardFile(root, "Art/ship.blend", "", true); err != nil {
		t.Fatal(err)
	}
	a.unlockDiscarded(root, []string{"Art/ship.blend"}) // (DiscardFile does it in the background)
	if got := f.Locks(tp.ID); len(got) != 0 {
		t.Errorf("locks after discarding: %v", got)
	}

	// Offline meanwhile, someone took it: told.
	f.Down.Store(true)
	writeFile(t, root, "Art/ship.blend", "offline again")
	a.autoLock(root, []string{"Art/ship.blend"})
	f.Down.Store(false)
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	cloud.SetLocks(f.Address, tp.ID, "ws-k", []string{"Art/"}, nil)
	t.Setenv("R3V_CLOUD_TOKEN", "token")
	a.autoLock(root, nil)
	if h := ev.held(); len(h) != 2 || h[1].Path != "Art/ship.blend" || !h[1].Meanwhile {
		t.Errorf("told: %+v", h)
	}

	// The project's rules turn locks off: nothing shows.
	writeFile(t, root, ".r3v.yaml", "presets:\n  ./: ableton\nfile_locks:\n  enabled: false\n")
	if v, _ := a.ProjectLocks(root); v.On {
		t.Error("locks shown with the project's off")
	}
}
