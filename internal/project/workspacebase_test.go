package project

import (
	"errors"
	"path/filepath"
	"testing"
)

// A workspace made from main whose work lands on dev now: listed against
// dev for one call, then given dev as its base: ahead and behind, what
// status says it lacks, the locks kept for base branches and the check
// before removing it all go by dev.
func TestWorkspaceBase(t *testing.T) {
	withWorkspaces(t)
	_, code := cutTeam(t)
	r := codeProject(t, code)
	dir := filepath.Join(t.TempDir(), "renderer")
	if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "renderer"}); err != nil {
		t.Fatal(err)
	}
	ws, _ := Open(dir)
	write(t, dir, "src/render.ts", "render(fast)\n")
	if _, _, err := ws.Save("faster", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	// dev: main and a version of its own.
	if _, err := r.NewBranch("dev", "", true); err != nil {
		t.Fatal(err)
	}
	write(t, r.Root, "src/main.ts", "export const a = 2\n")
	if _, _, err := r.Save("dev's own", Strategy("fail")); err != nil {
		t.Fatal(err)
	}

	counts := func(st []WorkspaceStatus, err error) (int, int) {
		t.Helper()
		if err != nil || len(st) != 1 || st[0].Ahead == nil {
			t.Fatalf("statuses: %v %+v", err, st)
		}
		return *st[0].Ahead, *st[0].Behind
	}
	if a, b := counts(r.WorkspaceStatuses()); a != 1 || b != 0 {
		t.Fatalf("against main: %d ahead, %d behind", a, b)
	}
	if a, b := counts(r.WorkspaceStatusesAgainst("dev")); a != 1 || b != 1 {
		t.Fatalf("against dev: %d ahead, %d behind", a, b)
	}
	if _, err := r.WorkspaceStatusesAgainst("nope"); err == nil {
		t.Error("against a branch that isn't there")
	}
	if list, _ := r.LocalWorkspaces(); list[0].Base != "main" {
		t.Fatalf("--against changed the base: %+v", list[0])
	}
	if _, behind := ws.BaseNews(); behind != 0 {
		t.Fatalf("main hasn't moved, yet %d behind", behind)
	}

	// Refused: no such branch, its own branch, not a workspace.
	if _, err := r.SetWorkspaceBase(dir, "nope"); err == nil {
		t.Error("a base that isn't there")
	}
	if _, err := r.SetWorkspaceBase(dir, "renderer"); err == nil {
		t.Error("its own branch as its base")
	}
	if _, err := r.SetWorkspaceBase(filepath.Join(t.TempDir(), "x"), "dev"); err == nil {
		t.Error("a folder that isn't a workspace")
	}
	if r.isBaseBranch("dev") {
		t.Fatal("dev a base branch before any workspace merges into it")
	}

	e, err := r.SetWorkspaceBase(dir, "dev")
	if err != nil || e.Base != "dev" || e.Branch != "renderer" {
		t.Fatalf("base: %v %+v", err, e)
	}
	if a, b := counts(r.WorkspaceStatuses()); a != 1 || b != 1 {
		t.Fatalf("against its new base: %d ahead, %d behind", a, b)
	}
	if base, behind := ws.BaseNews(); base != "dev" || behind != 1 {
		t.Fatalf("base news: %q %d", base, behind)
	}
	if !r.isBaseBranch("dev") {
		t.Error("dev isn't a base branch now")
	}
	// Merged into main only, it's still not done; merged into dev, it is.
	var nd *WorkspaceNotDoneError
	if err := r.RemoveWorkspace(dir, false); !errors.As(err, &nd) || nd.Base != "dev" || nd.Unmerged != 1 {
		t.Fatalf("remove before merging into dev: %v", err)
	}
	if _, err := r.MergeBranch("renderer", "", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveWorkspace(dir, false); err != nil {
		t.Fatalf("remove once merged into dev: %v", err)
	}
}
