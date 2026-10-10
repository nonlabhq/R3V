package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

func withWorkspaces(t *testing.T) {
	t.Helper()
	was := AgentWorkspaces
	AgentWorkspaces = true
	t.Cleanup(func() { AgentWorkspaces = was })
}

// codeProject: a code project in a team, one version shared: sources, a
// texture, and what its rules leave out (node_modules/, target/).
func codeProject(t *testing.T, code string) *Repo {
	t.Helper()
	root := filepath.Join(t.TempDir(), "game")
	write(t, root, ".r3v.yaml", "presets:\n  ./: none\nrules:\n  - ignore: \"node_modules/\"\n  - ignore: \"target/\"\n")
	write(t, root, "src/main.ts", "export const a = 1\n")
	write(t, root, "src/render.ts", "render()\n")
	write(t, root, "assets/wall.png", string(randomBytes(21, 300<<10)))
	write(t, root, "node_modules/x/index.js", "module.exports = 1\n")
	write(t, root, "target/debug/app", "built")
	r, err := Init(root, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Save("first", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	return r
}

// tracked is files without what the code project's rules leave out.
func tracked(m map[string]string) map[string]string {
	out := map[string]string{}
	for p, c := range m {
		if !strings.HasPrefix(p, "node_modules/") && !strings.HasPrefix(p, "target/") {
			out[p] = c
		}
	}
	return out
}

// objectReads counts the file contents a step downloads.
func objectReads(fake *s3test.Server, step func()) int {
	var n atomic.Int32
	fake.OnRead = func(path string) {
		if strings.Contains(path, "/objects/") {
			n.Add(1)
		}
	}
	defer func() { fake.OnRead = nil }()
	step()
	return int(n.Load())
}

// A workspace: made from the files here (nothing downloaded, though the
// store gave the team's files up), without what the rules leave out; work
// committed there is on its own branch, listed as ahead; it can't be
// removed until merged and committed, then goes whole.
func TestWorkspaceAddListRemove(t *testing.T) {
	withWorkspaces(t)
	fake, code := cutTeam(t)
	r := codeProject(t, code)
	if _, err := r.PruneObjects(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "renderer")
	var e *WorkspaceEntry
	if n := objectReads(fake, func() {
		var err error
		if e, err = r.AddWorkspace(dir, WorkspaceOptions{Branch: "renderer"}); err != nil {
			t.Fatal(err)
		}
	}); n != 0 {
		t.Errorf("downloaded %d files", n)
	}
	if e.Branch != "renderer" || e.Base != "main" || !samePath(e.Folder, dir) {
		t.Fatalf("entry: %+v", e)
	}
	if got, want := files(t, dir), tracked(files(t, r.Root)); !maps.Equal(got, want) {
		t.Fatalf("workspace files %v, want %v", fileNames(got), fileNames(want))
	}
	if _, err := os.Stat(filepath.Join(dir, ".r3v", "objects")); err == nil {
		t.Error("the workspace has a store of its own")
	}
	if list, _ := r.LocalWorkspaces(); len(list) != 1 {
		t.Fatalf("registry: %+v", list)
	}
	// Again: nothing more; a folder in the project, or one not empty: refused.
	if again, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "renderer"}); err != nil || again.Created != e.Created {
		t.Fatalf("adding again: %v %+v", err, again)
	}
	if _, err := r.AddWorkspace(filepath.Join(r.Root, "wt"), WorkspaceOptions{Branch: "inside"}); err == nil {
		t.Error("a workspace inside the project")
	}
	full := filepath.Join(t.TempDir(), "full")
	write(t, full, "x.txt", "x")
	if _, err := r.AddWorkspace(full, WorkspaceOptions{Branch: "full"}); err == nil {
		t.Error("a workspace in a folder with files")
	}
	if _, err := r.AddWorkspace(filepath.Join(t.TempDir(), "m"), WorkspaceOptions{Branch: "main"}); err == nil {
		t.Error("a workspace on the branch it's made from")
	}

	// Working there: a commit goes to its branch.
	ws, err := Open(filepath.Join(dir, "src"))
	if err != nil || !ws.IsWorkspace() || !samePath(ws.Root, dir) || ws.BranchName() != "renderer" ||
		ws.Config.ProjectID != r.Config.ProjectID || ws.Config.WorkspaceID == r.Config.WorkspaceID {
		t.Fatalf("open: %v %+v", err, ws)
	}
	if ch, err := ws.Status(); err != nil || len(ch) != 0 {
		t.Fatalf("status: %v %+v", err, ch)
	}
	write(t, dir, "src/render.ts", "render(fast)\n")
	if _, res, err := ws.Save("faster", Strategy("fail")); err != nil || res.Action != "published" {
		t.Fatalf("commit in the workspace: %v %+v", err, res)
	}
	if r.Head() == ws.Head() || !r.HasSnapshot(ws.Head()) {
		t.Fatal("the version isn't in the shared store, or main moved")
	}
	st, err := r.WorkspaceStatuses()
	if err != nil || len(st) != 1 || st[0].Ahead == nil || *st[0].Ahead != 1 || *st[0].Behind != 0 ||
		st[0].Changes != 0 || st[0].Head != ws.Head() || st[0].Missing {
		t.Fatalf("list: %v %+v", err, st)
	}

	// Not merged: kept.
	if err := r.RemoveWorkspace(dir, false); !errors.Is(err, ErrWorkspaceNotDone) {
		t.Fatalf("remove before merging: %v", err)
	}
	if _, err := r.MergeBranch("renderer", "", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if got := fileOf(t, filepath.Join(r.Root, "src", "render.ts")); got != "render(fast)\n" {
		t.Fatalf("merged: %q", got)
	}
	// Changes not committed: kept.
	write(t, dir, "src/main.ts", "export const a = 2\n")
	var nd *WorkspaceNotDoneError
	if err := r.RemoveWorkspace(dir, false); !errors.As(err, &nd) || nd.Changes != 1 || nd.Unmerged != 0 {
		t.Fatalf("remove with a change: %v", err)
	}
	if st, _ := r.WorkspaceStatuses(); st[0].Changes != 1 || *st[0].Ahead != 0 || *st[0].Behind != 1 {
		t.Fatalf("list: %+v", st[0])
	}
	if err := r.RemoveWorkspace(dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder is still there: %v", err)
	}
	if list, _ := r.LocalWorkspaces(); len(list) != 0 {
		t.Errorf("still listed: %+v", list)
	}
	if err := r.RemoveWorkspace(dir, false); err == nil {
		t.Error("removing it again")
	}
}

// A workspace's own work: a version only it has, changes parked by going
// to another version; cleanup in the main copy keeps them both.
func TestWorkspaceWorkAndCleanup(t *testing.T) {
	withWorkspaces(t)
	defer func(p bool) { Parking = p }(Parking)
	Parking = true
	_, code := cutTeam(t)
	r := codeProject(t, code)
	dir := filepath.Join(t.TempDir(), "art")
	if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "art"}); err != nil {
		t.Fatal(err)
	}
	ws, _ := Open(dir)

	// A version only here (not shared) and changes parked by a switch.
	local := string(randomBytes(22, 200<<10))
	write(t, dir, "assets/floor.png", local)
	kept, err := ws.Snapshot("floor, here only")
	if err != nil {
		t.Fatal(err)
	}
	parked := string(randomBytes(23, 200<<10))
	write(t, dir, "assets/ceiling.png", parked)
	if _, _, err := ws.GoTo(kept.Parents[0], false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "ceiling.png")); err == nil {
		t.Fatal("the change wasn't parked")
	}
	if ps := ws.ParkedSets(); len(ps) != 1 {
		t.Fatalf("parked: %+v", ps)
	}

	// The main copy cleans up: what the workspace needs stays.
	if _, err := r.PruneObjects(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GC(); err != nil {
		t.Fatal(err)
	}
	if !r.HasSnapshot(kept.ID) {
		t.Fatal("the workspace's version went")
	}
	for _, f := range kept.Files {
		if f.Path == "assets/floor.png" && !r.Store.Has(f.Hash) {
			t.Fatal("its file went")
		}
	}
	// A workspace at work: no cleanup meanwhile.
	release, err := ws.Lock(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.GC(); !errors.Is(err, ErrBusy) {
		t.Errorf("GC while a workspace works: %v", err)
	}
	release()

	// Back on its latest version, the parked changes come back.
	if _, _, err := ws.GoTo("latest", false); err != nil {
		t.Fatal(err)
	}
	if got := fileOf(t, filepath.Join(dir, "assets", "ceiling.png")); got != parked {
		t.Fatal("the parked change didn't come back whole")
	}
	if got := fileOf(t, filepath.Join(dir, "assets", "floor.png")); got != local {
		t.Fatal("the version only here lost its file")
	}
	// Shared, then to another branch and back.
	if _, res, err := ws.Save("ceiling", Strategy("fail")); err != nil || res.Action != "published" {
		t.Fatalf("commit: %v %+v", err, res)
	}
	if bs, err := ws.Branches(); err != nil || len(bs) != 2 {
		t.Fatalf("branches: %v %+v", err, bs)
	}
	if _, err := ws.SwitchBranch("main", false); err != nil || ws.Head() != r.Head() {
		t.Fatalf("switch: %v", err)
	}
	if _, err := ws.SwitchBranch("art", false); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if got := fileOf(t, filepath.Join(dir, "assets", "ceiling.png")); got != parked {
		t.Fatal("not back on its branch")
	}
	// A workspace's team is the main copy's.
	if err := ws.SetRemote(code); !errors.Is(err, ErrInWorkspace) {
		t.Errorf("joining a team in a workspace: %v", err)
	}
	// Removed (forced: work not merged), what only it kept goes at the next
	// cleanup.
	if err := r.RemoveWorkspace(dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GC(); err != nil {
		t.Fatal(err)
	}
	for _, f := range ws.ParkedSets() {
		t.Errorf("still parked: %+v", f)
	}
}

// R3V killed while it adds a workspace (placing its files, making its
// branch, after the branch): the main copy is as it was, the workspace
// isn't listed, and adding again finishes it (or removing clears it).
func TestWorkspaceAddKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills adds")
	}
	withWorkspaces(t)
	setup := func() (*s3test.Server, *Repo, string) {
		fake, code := cutTeam(t)
		r := codeProject(t, code)
		return fake, r, filepath.Join(t.TempDir(), "agent")
	}
	fake, r, dir := setup()
	steps := requests(fake, "write", func() {
		if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
			t.Fatal(err)
		}
	})
	want := files(t, dir)
	type kill struct {
		env []string
		n   int
	}
	kills := []kill{{env: []string{"R3V_KILL_WRITES=1"}}, {env: []string{"R3V_KILL_WRITES=3"}},
		{env: []string{"R3V_KILL_STEP=files"}}, {env: []string{"R3V_KILL_STEP=branch"}}}
	for _, n := range killPoints(steps) {
		kills = append(kills, kill{n: n})
	}
	for _, k := range kills {
		for _, finish := range []string{"add", "remove"} {
			what := fmt.Sprintf("killed %v at write %d of %d, then %s", k.env, k.n, steps, finish)
			fake, r, dir := setup()
			before, head := files(t, r.Root), r.Head()
			env := append([]string{"R3V_KILL_DIR=" + r.Root, "R3V_KILL_WS=" + dir}, k.env...)
			if !runKilled(t, fake, "workspace-add", "write", k.n, env...) {
				t.Fatalf("%s: it finished", what)
			}
			r, _ = Open(r.Root)
			if !maps.Equal(files(t, r.Root), before) || r.Head() != head {
				t.Fatalf("%s: the main copy changed", what)
			}
			if list, _ := r.LocalWorkspaces(); len(list) != 0 {
				t.Fatalf("%s: listed: %+v", what, list)
			}
			if finish == "add" {
				if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
					t.Fatalf("%s: adding again: %v", what, err)
				}
				if got := files(t, dir); !maps.Equal(got, want) || len(leftovers(dir)) != 0 {
					t.Fatalf("%s: files %v", what, fileNames(got))
				}
				ws, err := Open(dir)
				if err != nil || ws.BranchName() != "agent" || ws.UnfinishedSwitch() != "" {
					t.Fatalf("%s: %v %+v", what, err, ws)
				}
				if list, _ := r.LocalWorkspaces(); len(list) != 1 {
					t.Fatalf("%s: listed: %+v", what, list)
				}
				continue
			}
			if err := r.RemoveWorkspace(dir, false); err != nil {
				t.Fatalf("%s: removing: %v", what, err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s: the folder is still there", what)
			}
		}
	}
}

// R3V killed while it removes a workspace (after unregistering it, after
// deleting some of its files): it is not listed, the main copy is whole,
// and removing again clears the folder.
func TestWorkspaceRemoveKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills removes")
	}
	withWorkspaces(t)
	for _, step := range []string{"unregistered", "removed assets", "removed src"} {
		_, code := cutTeam(t)
		r := codeProject(t, code)
		dir := filepath.Join(t.TempDir(), "agent")
		if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
			t.Fatal(err)
		}
		before := files(t, r.Root)
		if !runKilled(t, nil, "workspace-remove", "", 0, "R3V_KILL_DIR="+r.Root, "R3V_KILL_WS="+dir,
			"R3V_KILL_STEP="+step) {
			t.Fatalf("killed at %s: it finished", step)
		}
		if list, _ := r.LocalWorkspaces(); len(list) != 0 {
			t.Fatalf("killed at %s: still listed", step)
		}
		if !maps.Equal(files(t, r.Root), before) {
			t.Fatalf("killed at %s: the main copy changed", step)
		}
		if _, err := r.GC(); err != nil {
			t.Fatalf("killed at %s: cleanup: %v", step, err)
		}
		if step == "removed assets" { // (or made again: whole)
			want := tracked(before)
			if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
				t.Fatalf("killed at %s: adding again: %v", step, err)
			}
			if got := files(t, dir); !maps.Equal(got, want) {
				t.Fatalf("killed at %s: added again: %v", step, fileNames(got))
			}
		}
		if err := r.RemoveWorkspace(dir, false); err != nil {
			t.Fatalf("killed at %s: removing again: %v", step, err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("killed at %s: the folder is still there", step)
		}
	}
}
