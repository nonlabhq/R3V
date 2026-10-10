//go:build nightly

package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// agentProject is a project kept on this computer with files that lock
// between workspaces (a texture, a model), one version committed.
func agentProject(t *testing.T) (string, *project.Repo) {
	t.Helper()
	t.Setenv(actor.EnvTool, actor.None) // (the app is the person, whatever runs the tests)
	root := newSong(t)
	writeFile(t, root, "tex/rock.png", "rock")
	writeFile(t, root, "models/ship.glb", "ship")
	writeFile(t, root, "src/main.ts", "let a = 1\n")
	r, err := project.Init(root, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Snapshot("first"); err != nil {
		t.Fatal(err)
	}
	teams.Update(func(s *teams.Store) error { s.AddLocal(root); return nil })
	return root, r
}

var renderer = agentlock.Holder{Workspace: "0123456789abcdef", Branch: "renderer", Actor: "claude-code"}

// The Files tab shows agent locks with the team's: an agent's workspace
// (its tool and branch) and, while agents are at work, your changes not
// committed; breaking one takes it back.
func TestAppAgentLocks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	if !project.AgentLocks {
		t.Fatal("agent locks are off in Nightly")
	}
	a := NewApp()
	var events []string
	a.emit = func(name string, _ any) { events = append(events, name) }
	root, r := agentProject(t)
	writeFile(t, root, "tex/rock.png", "rock 2") // (yours, not committed)

	// No agent at work: nothing shows (not even your changes).
	if v, err := a.ProjectLocks(root); err != nil || v.On || len(v.Items) != 0 {
		t.Fatalf("no agents: %+v %v", v, err)
	}

	if _, err := r.AgentLockList().Take(renderer, []string{"models/ship.glb"}, agentlock.KindLock, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AgentLockList().Take(renderer, []string{"src/"}, agentlock.KindClaim, nil); err != nil {
		t.Fatal(err)
	}
	v, err := a.ProjectLocks(root)
	if err != nil || !v.On || v.Team || !v.Agents {
		t.Fatalf("agent locks: %+v %v", v, err)
	}
	if alone := a.AgentLocks(root); len(alone) != len(v.Items) {
		t.Errorf("agent locks alone: %+v", alone)
	}
	got := map[string]LockItem{}
	for _, it := range v.Items {
		got[it.Path] = it
	}
	if len(got) != 2 {
		t.Fatalf("items (claims hold nothing): %+v", v.Items)
	}
	if s := got["models/ship.glb"]; !s.Agent || s.Mine || s.Person || s.Tool != "claude-code" || s.Branch != "renderer" || s.Since == "" {
		t.Errorf("the agent's lock: %+v", s)
	}
	if p := got["tex/rock.png"]; !p.Agent || !p.Mine || !p.Person {
		t.Errorf("your changes: %+v", p)
	}

	// Broken: gone, and the page told.
	if err := a.BreakAgentLock(root, "models/ship.glb"); err != nil {
		t.Fatal(err)
	}
	if all, _ := r.AgentLockList().All(); len(all) != 1 || all[0].Kind != agentlock.KindClaim {
		t.Fatalf("after breaking: %+v", all)
	}
	if len(events) == 0 || events[len(events)-1] != "locks" {
		t.Errorf("events: %v", events)
	}
}

// An agent's commit of a file another workspace holds is refused with who
// holds it; nothing is committed.
func TestAppCommitRefusedByAgentLock(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	root, r := agentProject(t)
	if _, err := r.AgentLockList().Take(renderer, []string{"tex/"}, agentlock.KindLock, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "tex/rock.png", "rock 2")
	head := r.Head()
	t.Setenv(actor.EnvTool, "codex") // (an app started by an agent)
	res, err := a.Save(root, "rock", false, nil, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "agent-locked" || len(res.Locks) != 1 {
		t.Fatalf("result: %+v", res)
	}
	if l := res.Locks[0]; l.Path != "tex/" || l.Tool != "claude-code" || l.Branch != "renderer" || l.Person {
		t.Errorf("held: %+v", l)
	}
	r2, _ := project.Open(root)
	if r2.Head() != head {
		t.Error("committed")
	}
	if _, err := os.Stat(filepath.Join(root, "tex", "rock.png")); err != nil {
		t.Fatal(err)
	}
}
