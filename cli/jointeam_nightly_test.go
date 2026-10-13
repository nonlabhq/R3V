//go:build nightly

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// An agent starts a project on a team the person joined, by its name: no
// connection code. init --team makes it the team's (a commit shares it),
// clone --team downloads it, remote --team connects a project made before;
// a name this computer hasn't joined leaves the folder as it was.
func TestTeamByName(t *testing.T) {
	// (init, remote and clone answer in text: run in dir, the exit status)
	plain := func(dir string, args ...string) int {
		t.Helper()
		t.Chdir(dir)
		return Run(args)
	}
	a := teamProject(t) // (the person joined the team with its code)
	ra, _ := project.Open(a)
	team, err := ra.Team()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := teams.Update(func(s *teams.Store) error { return s.Rename(team.ID, "Agent Test Band") }); err != nil {
		t.Fatal(err)
	}

	b := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.md": "# Plan\n"})
	if exit := plain(b, "init", "--team", "agent test band"); exit != 0 {
		t.Fatalf("init --team: %d", exit)
	}
	rb, err := project.Open(b)
	if err != nil {
		t.Fatal(err)
	}
	if tb, err := rb.Team(); err != nil || tb.ID != team.ID {
		t.Fatalf("its team: %+v %v", tb, err)
	}
	if exit, rep := run(t, b, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	if s, _ := teams.Load(); s.Projects[team.ID+"/"+rb.Config.ProjectID] == "" {
		t.Error("the app doesn't list it")
	}

	c := filepath.Join(t.TempDir(), "Plan copy")
	if exit := plain(t.TempDir(), "clone", "--team", team.ID, rb.Config.ProjectID, c); exit != 0 {
		t.Fatalf("clone --team: %d", exit)
	}
	if data, err := os.ReadFile(filepath.Join(c, "notes.md")); err != nil || string(data) != "# Plan\n" {
		t.Fatalf("cloned: %q %v", data, err)
	}

	d := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "a.txt": "a"})
	if exit := plain(d, "init"); exit != 0 {
		t.Fatal("init")
	}
	if exit := plain(d, "remote", "--team", "Agent Test Band"); exit != 0 {
		t.Fatalf("remote --team: %d", exit)
	}
	if rd, _ := project.Open(d); rd.Config.Remote == nil {
		t.Error("remote --team: not connected")
	}

	e := newFolder(t, map[string]string{"a.txt": "a"})
	if exit := plain(e, "init", "--team", "Nobody's Band", "--preset", "none"); exit == 0 {
		t.Fatal("a team this computer hasn't joined: taken")
	}
	if _, err := os.Stat(filepath.Join(e, ".r3v")); !os.IsNotExist(err) {
		t.Errorf("the folder was made a project anyway: %v", err)
	}
}
