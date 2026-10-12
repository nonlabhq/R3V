package project

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// A workspace gets copies of the build folders .r3v.yaml names (any of a
// name with **/), not other left-out folders, nor links; the main copy's
// stay as they are.
func TestWorkspaceCopiesBuildFolders(t *testing.T) {
	withWorkspaces(t)
	_, code := cutTeam(t)
	r := codeProject(t, code)
	write(t, r.Root, ".r3v.yaml", "presets:\n  ./: none\nrules:\n  - ignore: \"node_modules/\"\n"+
		"  - ignore: \"target/\"\n  - ignore: \"**/gen/\"\n  - ignore: \"cache/\"\n"+
		"workspaces:\n  copy: [\"target/\", \"**/gen/\"]\n")
	write(t, r.Root, "apps/tool/gen/schema.json", "{}")
	write(t, r.Root, "cache/big.bin", "not copied")
	r.forgetProfile() // (as a new process reads the rules again)
	if _, _, err := r.Save("copy the build folders", Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	os.Symlink(filepath.Join(r.Root, "src"), filepath.Join(r.Root, "target", "link")) // (where the system lets it)

	dir := filepath.Join(t.TempDir(), "renderer")
	if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "renderer"}); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]bool{"target/debug/app": true, "apps/tool/gen/schema.json": true,
		"cache/big.bin": false, "node_modules/x/index.js": false, "target/link": false} {
		if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel))); (err == nil) != want {
			t.Errorf("%s in the workspace: %v, want %v", rel, err == nil, want)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "target", "debug", "app")); string(data) != "built" {
		t.Errorf("target/debug/app: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".r3v", copyingDir)); err == nil {
		t.Error("the copying folder left behind")
	}
	ws, _ := Open(dir)
	if changes, _ := ws.Status(); len(changes) != 0 {
		t.Errorf("the copies show as changes: %+v", changes)
	}
}

// A rule that isn't a folder of the project is refused.
func TestWorkspaceCopyRules(t *testing.T) {
	r, _ := Init(newProject(t), "yi")
	for _, bad := range []string{"*.bin", "../outside/", "/abs/", "C:/x/"} {
		write(t, r.Root, ".r3v.yaml", "workspaces:\n  copy: [\""+bad+"\"]\n")
		r.forgetProfile()
		if _, err := r.Profile(); err == nil {
			t.Errorf("%q taken", bad)
		}
	}
}

// R3V stopped while a workspace's build folders are copied: the main copy
// as it was, the workspace not listed; added again, the folders are whole
// and nothing of the copy is left.
func TestWorkspaceCopyKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills workspace adds")
	}
	withWorkspaces(t)
	setup := func() (*Repo, string) {
		_, code := cutTeam(t)
		r := codeProject(t, code)
		write(t, r.Root, ".r3v.yaml", "presets:\n  ./: none\nrules:\n  - ignore: \"node_modules/\"\n  - ignore: \"target/\"\n"+
			"workspaces:\n  copy: [\"target/\"]\n")
		for i := 0; i < 4; i++ {
			write(t, r.Root, fmt.Sprintf("target/debug/part%d.o", i), string(randomBytes(int64(40+i), 100<<10)))
		}
		r.forgetProfile()
		if _, _, err := r.Save("copy target", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
		return r, filepath.Join(t.TempDir(), "agent")
	}
	r, dir := setup()
	all := writes(func() {
		if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
			t.Fatal(err)
		}
	})
	want := files(t, dir)
	for _, env := range [][]string{{"R3V_KILL_STEP=files"}, {fmt.Sprintf("R3V_KILL_WRITES=%d", all-2)}, {fmt.Sprintf("R3V_KILL_WRITES=%d", all)}} {
		what := fmt.Sprintf("killed %v (%d writes in all)", env, all)
		r, dir := setup()
		if !runKilled(t, nil, "workspace-add", "", 0, append([]string{"R3V_KILL_DIR=" + r.Root, "R3V_KILL_WS=" + dir}, env...)...) {
			t.Fatalf("%s: it finished", what)
		}
		r, _ = Open(r.Root)
		if list, _ := r.LocalWorkspaces(); len(list) != 0 {
			t.Fatalf("%s: listed: %+v", what, list)
		}
		if _, err := r.AddWorkspace(dir, WorkspaceOptions{Branch: "agent"}); err != nil {
			t.Fatalf("%s: adding again: %v", what, err)
		}
		if got := files(t, dir); !maps.Equal(got, want) || len(leftovers(dir)) != 0 {
			t.Fatalf("%s: files %v", what, fileNames(got))
		}
		if _, err := os.Stat(filepath.Join(dir, ".r3v", copyingDir)); err == nil {
			t.Fatalf("%s: the copying folder left behind", what)
		}
	}
}
