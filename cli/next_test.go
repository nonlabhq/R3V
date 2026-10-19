package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// asPerson, asAgent and asUnknown set who runs R3V for a test: the person
// (R3V_ACTOR=none), Claude Code, or a tool R3V can't tell (the agents of
// the field tests: no R3V_ACTOR, no marker of their own).
func asPerson(t *testing.T) { t.Setenv(actor.EnvTool, actor.None); t.Setenv("CLAUDECODE", "") }
func asAgent(t *testing.T)  { t.Setenv(actor.EnvTool, ""); t.Setenv("CLAUDECODE", "1") }
func asUnknown(t *testing.T) {
	t.Setenv(actor.EnvTool, "")
	t.Setenv("CLAUDECODE", "")
}

// noTeams gives the test a team store of its own, empty; withTeam adds a
// team to it (no storage behind: nothing here reaches it).
func noTeams(t *testing.T) { t.Setenv("R3V_CONFIG_DIR", t.TempDir()) }

func withTeam(t *testing.T, name string) {
	t.Helper()
	if _, err := teams.Update(func(s *teams.Store) error {
		s.Upsert(remote.Config{URL: "s3+http://127.0.0.1:1/" + strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "/r3v",
			AccessKey: "key", SecretKey: "secret"}, name)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commands(steps []nextJSON) string {
	var out []string
	for _, s := range steps {
		out = append(out, s.Command)
	}
	return strings.Join(out, " | ")
}

// After init, what to do next: connect to a team this computer has (by
// name where the build can), else sign in or a connection code; an agent
// R3V can't tell is sent to the guide, Claude Code to agent setup.
func TestInitNext(t *testing.T) {
	initIn := func(t *testing.T) initJSON {
		t.Helper()
		dir := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "a.txt": "a"})
		exit, rep := run(t, dir, "init")
		if exit != 0 {
			t.Fatalf("init: %+v", rep.Error)
		}
		var out initJSON
		json.Unmarshal(rep.Result, &out)
		if out.Folder == "" || out.Team != "" || len(out.Next) == 0 {
			t.Fatalf("init: %s", rep.Result)
		}
		if last := out.Next[len(out.Next)-1].Command; !strings.HasPrefix(last, "r3v commit") {
			t.Errorf("the first commit isn't last: %s", commands(out.Next))
		}
		return out
	}

	t.Run("no teams", func(t *testing.T) {
		noTeams(t)
		asPerson(t)
		got := commands(initIn(t).Next)
		if !strings.HasPrefix(got, "r3v login | r3v remote <connection-code> | r3v commit") {
			t.Errorf("next: %s", got)
		}
		if strings.Contains(got, "help agents") || strings.Contains(got, "agent setup") {
			t.Errorf("a person told of agents: %s", got)
		}
	})
	t.Run("teams, by name", func(t *testing.T) {
		noTeams(t)
		withTeam(t, "Test Band")
		withTeam(t, "Other")
		byName(t, true)
		asPerson(t)
		got := commands(initIn(t).Next)
		if !strings.HasPrefix(got, `r3v remote --team "Test Band" | r3v remote --team Other | r3v commit`) {
			t.Errorf("next: %s", got)
		}
	})
	t.Run("teams, no names", func(t *testing.T) {
		noTeams(t)
		withTeam(t, "Test Band")
		byName(t, false)
		asPerson(t)
		next := initIn(t).Next
		if commands(next) != `r3v remote <connection-code> | r3v commit -m "first version"` ||
			!strings.Contains(next[0].Why, "Test Band") || !strings.Contains(next[0].Why, "R3V app") {
			t.Errorf("next: %+v", next)
		}
	})
	t.Run("an agent R3V can't tell", func(t *testing.T) {
		noTeams(t)
		asUnknown(t)
		if got := commands(initIn(t).Next); !strings.Contains(got, "r3v help agents") {
			t.Errorf("next: %s", got)
		}
	})
	t.Run("Claude Code", func(t *testing.T) {
		noTeams(t)
		asAgent(t)
		if _, ok := extraCommands["agent"]; !ok {
			if got := commands(initIn(t).Next); strings.Contains(got, "agent setup") {
				t.Errorf("agent setup in a build without it: %s", got)
			}
		}
		withAgentLocks(t)
		got := commands(initIn(t).Next)
		if !strings.Contains(got, "r3v agent setup | r3v commit") || strings.Contains(got, "help agents") {
			t.Errorf("next: %s", got)
		}
	})
	t.Run("init --team", func(t *testing.T) {
		noTeams(t)
		byName(t, true)
		asPerson(t)
		teamProject(t) // (the person joined the team)
		store, _ := teams.Load()
		name := store.Teams[0].Name
		dir := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "a.txt": "a"})
		exit, rep := run(t, dir, "init", "--team", name)
		var out initJSON
		json.Unmarshal(rep.Result, &out)
		if exit != 0 || out.Team != name || commands(out.Next) != `r3v commit -m "first version"` {
			t.Fatalf("init --team: %d %s %+v", exit, rep.Result, rep.Error)
		}
	})
	t.Run("text", func(t *testing.T) {
		noTeams(t)
		withTeam(t, "Test Band")
		byName(t, true)
		asUnknown(t)
		dir := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "a.txt": "a"})
		code, text := runText(t, dir, "init")
		for _, want := range []string{"not connected to a team yet", "next:", `r3v remote --team "Test Band"`, "r3v help agents", `r3v commit -m "first version"`} {
			if !strings.Contains(text, want) {
				t.Errorf("no %q:\n%s", want, text)
			}
		}
		if code != 0 {
			t.Fatalf("init: %d", code)
		}
	})
}

// A commit in no team says the version stays on this computer, and how to
// connect; status says so before.
func TestNotConnected(t *testing.T) {
	noTeams(t)
	withTeam(t, "Test Band")
	byName(t, true)
	asPerson(t)
	dir := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "a.txt": "a"})
	if exit, rep := run(t, dir, "init"); exit != 0 {
		t.Fatalf("init: %+v", rep.Error)
	}
	hasWarning := func(ws []warningJSON) bool {
		for _, w := range ws {
			if w.Code == "not_connected" && w.Reason != "" {
				return true
			}
		}
		return false
	}

	exit, rep := run(t, dir, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || st.Connected || st.Team != nil || !hasWarning(st.Warnings) ||
		commands(st.Next) != `r3v remote --team "Test Band"` {
		t.Fatalf("status: %s", rep.Result)
	}
	var raw map[string]any
	json.Unmarshal(rep.Result, &raw)
	if c, ok := raw["connected"]; !ok || c != false {
		t.Errorf("status: connected %v", c)
	}

	exit, rep = run(t, dir, "commit", "-m", "first")
	var cm commitJSON
	json.Unmarshal(rep.Result, &cm)
	if exit != 0 || cm.Action != "local" || cm.Shared || !hasWarning(cm.Warnings) ||
		!strings.HasPrefix(commands(cm.Next), `r3v remote --team "Test Band" | r3v commit`) {
		t.Fatalf("commit: %s", rep.Result)
	}
	json.Unmarshal(rep.Result, &raw)
	if s, ok := raw["shared"]; !ok || s != false {
		t.Errorf("commit: shared %v", s)
	}

	exit, rep = run(t, dir, "update")
	if exit != 1 || rep.Error.Code != "not_connected" || commands(rep.Error.Next) != `r3v remote --team "Test Band"` {
		t.Fatalf("update: %d %+v", exit, rep.Error)
	}
	if exit, rep = run(t, dir, "merge", "other"); exit != 1 || rep.Error.Code != "not_connected" || len(rep.Error.Next) == 0 {
		t.Fatalf("merge: %d %+v", exit, rep.Error)
	}

	writeFile(t, dir, "a.txt", "b")
	code, text := runText(t, dir, "commit", "-m", "second")
	if code != 0 || !strings.Contains(text, "on this computer only: it is not shared") ||
		!strings.Contains(text, `r3v remote --team "Test Band"`) || strings.Contains(text, "looks like a build") {
		t.Errorf("commit text: %d\n%s", code, text)
	}
	code, text = runText(t, dir, "status")
	if code != 0 || !strings.Contains(text, "not connected to a team") || !strings.Contains(text, `r3v remote --team "Test Band"`) {
		t.Errorf("status text: %d\n%s", code, text)
	}
}

// In a team, nothing of the above.
func TestConnectedNoHint(t *testing.T) {
	asPerson(t)
	a := teamProject(t)
	exit, rep := run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || !st.Connected || len(st.Next) != 0 {
		t.Fatalf("status: %s", rep.Result)
	}
	for _, w := range st.Warnings {
		if w.Code == "not_connected" {
			t.Error("status: not_connected in a team")
		}
	}
	writeFile(t, a, "src/main.ts", "export const x = 1;\n")
	exit, rep = run(t, a, "commit", "-m", "second")
	var cm commitJSON
	json.Unmarshal(rep.Result, &cm)
	if exit != 0 || cm.Action != "published" || !cm.Shared || len(cm.Next) != 0 || len(cm.Warnings) != 0 {
		t.Fatalf("commit: %s", rep.Result)
	}
}
