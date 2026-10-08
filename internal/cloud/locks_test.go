//go:build nightly

package cloud

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

const kai = "0000000000000000000000000000000b"

func put(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// as makes the next calls go as the member whose session token is token.
func as(t *testing.T, token string) { t.Setenv("R3V_CLOUD_TOKEN", token) }

// Two members of a hosted team with file locks: locks refused when someone
// else holds them (a file, a folder over it, or anything under a folder);
// a share changing a path someone else holds refused with who holds it,
// the version kept here; the holder's own share frees their file locks,
// not their folder's; an admin breaks a lock; with the switch off, nothing
// is checked.
func TestFileLocksBetweenTwoMembers(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	f.AddMember("kai", kai, false)

	dir := t.TempDir()
	put(t, dir, ".r3v.yaml", "presets:\n  ./: none\n")
	for _, p := range []string{"Maps/Harbor.umap", "Hero.uasset", "Art/a.psd", "notes.txt"} {
		put(t, dir, p, "0")
	}
	a, err := project.Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetRemote(f.Address); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Save("first", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	pid := a.Config.ProjectID
	if f.BranchBodies["branch"] == 0 || f.BranchBodies["plain"] != 0 {
		t.Errorf("branch moves: %v (always the form with the changed paths)", f.BranchBodies)
	}
	c, _ := a.Client()
	if remote.CapabilitiesOf(c).Locks {
		t.Error("locks before the team turned them on")
	}
	// Off: nothing to lock (unlocking still works, to clear old locks).
	if _, err := SetLocks(f.Address, pid, "ws-a", []string{"notes.txt"}, nil); !errors.Is(err, remote.ErrLocksOff) {
		t.Errorf("locking with the switch off: %v", err)
	}
	if _, err := SetLocks(f.Address, pid, "ws-a", nil, []string{"notes.txt"}); err != nil {
		t.Errorf("unlocking with the switch off: %v", err)
	}

	if err := remote.SetLocks(c, remote.LockSettings{On: true, Kinds: profile.DefaultLockKinds()}); err != nil {
		t.Fatal(err)
	}
	if info, _ := c.Info(); !slices.Contains(info.Features, remote.FeatureLocks) || !remote.LocksOf(info).On {
		t.Errorf("team info: %+v", info)
	}
	if !remote.CapabilitiesOf(c).Locks {
		t.Error("no locks once on")
	}

	// Kai takes the map and the art folder.
	as(t, "kai")
	b, _, err := project.Clone(f.Address, a.Config.Name, filepath.Join(t.TempDir(), "Kai"), "kai")
	if err != nil {
		t.Fatal(err)
	}
	res, err := SetLocks(f.Address, pid, "ws-k", []string{"Maps/Harbor.umap", "Art/", "Hero.uasset"}, nil)
	if err != nil || len(res.Locked) != 3 {
		t.Fatalf("kai locking: %+v %v", res, err)
	}
	ls, err := Locks(f.Address, pid)
	if err != nil || len(ls) != 3 || !ls[0].Prefix || ls[0].Path != "Art/" || ls[0].Since.IsZero() {
		t.Fatalf("locks: %+v %v", ls, err)
	}

	// The owner can't take them, nor anything under a folder, nor a folder
	// over one.
	as(t, "token")
	res, err = SetLocks(f.Address, pid, "ws-a", []string{"Maps/Harbor.umap", "Art/a.psd", "Maps/", "notes.txt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Locked, []string{"notes.txt"}) || len(res.Refused) != 3 || res.Refused[0].MemberID != kai {
		t.Errorf("owner locking: %+v", res)
	}

	// A change to the map can't be shared: the version stays here.
	put(t, a.Root, "Maps/Harbor.umap", "yi")
	m, _, err := a.Save("my harbor", project.Strategy("fail"))
	var locked *remote.ErrLocked
	if !errors.As(err, &locked) || len(locked.Locks) != 1 || locked.Locks[0].Path != "Maps/Harbor.umap" || locked.Locks[0].MemberID != kai {
		t.Fatalf("share of a held file: %v", err)
	}
	if m == nil || a.Head() != m.ID {
		t.Error("the version isn't kept here")
	}
	heads, _ := c.Branches(pid)
	if heads["main"] == a.Head() {
		t.Error("the branch moved")
	}
	// Refused, the share stopped half-way (its versions are up, the branch
	// didn't move): sharing again still says which paths it changes.
	if _, err := a.Share(project.Strategy("fail")); !errors.As(err, &locked) {
		t.Errorf("sharing again, the versions already up: %v", err)
	}

	// Kai shares his hero: his file lock on it goes, his others stay.
	as(t, "kai")
	put(t, b.Root, "Hero.uasset", "kai")
	if _, _, err := b.Save("hero", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if got := f.Locks(pid); got["Hero.uasset"] != "" || got["Art/"] != kai || got["Maps/Harbor.umap"] != kai {
		t.Errorf("after kai's share: %v", got)
	}
	// Breaking is for admins.
	if err := BreakLock(f.Address, pid, "Maps/Harbor.umap"); err == nil {
		t.Error("kai broke a lock")
	}

	// The owner (an admin) breaks the map's lock; the share goes through.
	as(t, "token")
	if err := BreakLock(f.Address, pid, "Maps/Harbor.umap"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Share(project.Strategy("fail")); err != nil {
		t.Fatalf("share once unlocked: %v", err)
	}
	// Under kai's folder: refused.
	put(t, a.Root, "Art/a.psd", "yi")
	if _, _, err := a.Save("art", project.Strategy("fail")); !errors.As(err, &locked) || locked.Locks[0].Path != "Art/a.psd" {
		t.Errorf("share under a folder lock: %v", err)
	}

	// An older R3V (no changed paths) is refused while locks are on.
	resp := rawBranchPut(t, f, pid)
	if resp != 409 {
		t.Errorf("plain branch move with locks on: %d", resp)
	}

	// The switch off: nothing is checked, and older R3Vs share again.
	if err := remote.SetLocks(c, remote.LockSettings{On: false, Kinds: []string{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Share(project.Strategy("fail")); err != nil {
		t.Errorf("share with locks off: %v", err)
	}
	if remote.CapabilitiesOf(c).Locks {
		t.Error("locks shown once off")
	}
}

// rawBranchPut moves a branch the way an older R3V does (the head alone).
func rawBranchPut(t *testing.T, f *cloudtest.Fake, pid string) int {
	t.Helper()
	req, _ := http.NewRequest("PUT", f.Service+"/v1/teams/t/keys/"+strings.ReplaceAll("projects/"+pid+"/branches/old", "/", "%2F"),
		strings.NewReader(strings.Repeat("1", 64)+"\n"))
	req.Header.Set("authorization", "Bearer token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// Merging main into a branch brings teammates' changes, shared already:
// the share isn't refused for a file someone else holds that only main
// changed. A change of one's own to it is.
func TestMergeFromMainIsNotMyChange(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	f.AddMember("kai", kai, false)
	dir := t.TempDir()
	put(t, dir, ".r3v.yaml", "presets:\n  ./: none\n")
	put(t, dir, "Maps/Harbor.umap", "0")
	put(t, dir, "notes.txt", "0")
	a, _ := project.Init(dir, "yi")
	if err := a.SetRemote(f.Address); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Save("first", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	c, _ := a.Client()
	remote.SetLocks(c, remote.LockSettings{On: true, Kinds: profile.DefaultLockKinds()})
	if err := a.CreateBranch("idea"); err != nil {
		t.Fatal(err)
	}
	put(t, a.Root, "notes.txt", "idea")
	if _, _, err := a.Save("idea", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}

	// Kai, on main, changes the map (holding it, then keeping a lock on it
	// after his share: he took it again).
	as(t, "kai")
	b, _, err := project.Clone(f.Address, a.Config.Name, filepath.Join(t.TempDir(), "Kai"), "kai")
	if err != nil {
		t.Fatal(err)
	}
	pid := a.Config.ProjectID
	SetLocks(f.Address, pid, "ws-k", []string{"Maps/Harbor.umap"}, nil)
	put(t, b.Root, "Maps/Harbor.umap", "kai")
	if _, _, err := b.Save("harbor", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	SetLocks(f.Address, pid, "ws-k", []string{"Maps/Harbor.umap"}, nil)

	as(t, "token")
	if _, err := a.MergeBranch("main", "", project.Strategy("fail")); err != nil {
		t.Fatalf("merging main into the branch: %v", err)
	}
	put(t, a.Root, "Maps/Harbor.umap", "yi")
	var locked *remote.ErrLocked
	if _, _, err := a.Save("my harbor", project.Strategy("fail")); !errors.As(err, &locked) {
		t.Errorf("my own change to the held map: %v", err)
	}
}
