package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/remote"
)

// agentProject is a project with files that lock (a texture, a model)
// and one committed version; agent locks on.
func agentProject(t *testing.T) *Repo {
	t.Helper()
	was := AgentLocks
	AgentLocks = true
	t.Cleanup(func() { AgentLocks = was })
	dir := newProject(t)
	for p, c := range map[string]string{"tex/rock.png": "rock", "models/ship.glb": "ship", "notes.txt": "notes"} {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644)
	}
	r, err := Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	mustSnapshot(t, r, "first")
	return r
}

var other = agentlock.Holder{Workspace: "0123456789abcdef", Branch: "renderer", Actor: "claude-code"}

func TestAgentLocked(t *testing.T) {
	r := agentProject(t)
	for p, want := range map[string]bool{"tex/rock.png": true, "models/ship.glb": true, "notes.txt": false,
		"Song.als": false, "tex/": false, "Backup/Song [old].als": false} {
		if got := r.AgentLocked(p); got != want {
			t.Errorf("AgentLocked(%s) = %v", p, got)
		}
	}
	// The project's own settings narrow it (its workflow, never .r3v.yaml).
	os.WriteFile(filepath.Join(r.Root, ".r3v.yaml"), []byte("file_locks:\n  enabled: false\n"), 0o644)
	r1, _ := Open(r.Root)
	if !r1.AgentLocked("models/ship.glb") {
		t.Fatal(".r3v.yaml's file_locks are no longer read")
	}
	r1.KeepWorkflow(TeamWorkflow{Project: remote.Workflow{Locks: &remote.ProjectLocks{Remove: []string{"tex/"}}}})
	r2, _ := Open(r.Root)
	if r2.AgentLocked("tex/rock.png") || !r2.AgentLocked("models/ship.glb") {
		t.Fatal("the project's paths not locking")
	}
	off := false
	r2.KeepWorkflow(TeamWorkflow{Team: remote.Workflow{AgentLocks: &off}})
	r3, _ := Open(r.Root)
	if r3.AgentLocked("models/ship.glb") {
		t.Fatal("the team turned agent locks off")
	}
}

func TestAgentCommitRefused(t *testing.T) {
	r := agentProject(t)
	if _, err := r.AgentLockList().Take(other, []string{"tex/"}, agentlock.KindLock, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(r.Root, "tex", "rock.png"), []byte("rock 2"), 0o644)
	os.WriteFile(filepath.Join(r.Root, "models", "ship.glb"), []byte("ship 2"), 0o644)

	r.Actor = claudeCode
	head := r.Head()
	_, err := r.Snapshot("an agent's")
	var locked *ErrLockedByAgent
	if !errors.As(err, &locked) || len(locked.Locks) != 1 || locked.Locks[0].Path != "tex/" || r.Head() != head {
		t.Fatalf("an agent's commit of a file another holds: %v", err)
	}
	if all, _ := r.AgentLockList().All(); len(all) != 1 {
		t.Fatalf("refused: nothing taken (ship.glb included): %+v", all)
	}

	// The person's commit is never refused.
	r.Actor = nil
	if _, err := r.Snapshot("the person's"); err != nil {
		t.Fatalf("a person's commit: %v", err)
	}

	// An agent's commit of files no one holds takes them.
	r.Actor = claudeCode
	os.WriteFile(filepath.Join(r.Root, "models", "ship.glb"), []byte("ship 3"), 0o644)
	os.WriteFile(filepath.Join(r.Root, "notes.txt"), []byte("notes 2"), 0o644)
	if _, err := r.Snapshot("an agent's"); err != nil {
		t.Fatal(err)
	}
	all, _ := r.AgentLockList().All()
	if len(all) != 2 || all[0].Path != "models/ship.glb" || all[0].Workspace != r.Config.WorkspaceID || all[0].Actor != "claude-code" {
		t.Fatalf("taken at commit (notes.txt doesn't lock): %+v", all)
	}

	// Off (Stable): nothing checked.
	AgentLocks = false
	os.WriteFile(filepath.Join(r.Root, "tex", "rock.png"), []byte("rock 3"), 0o644)
	if _, err := r.Snapshot("stable"); err != nil {
		t.Fatalf("Stable checks nothing: %v", err)
	}
}

// The person's changes not committed in their copy hold those files.
func TestPersonHolds(t *testing.T) {
	agent := agentProject(t)
	person := filepath.Join(t.TempDir(), "Person")
	copyTree(t, agent.Root, person)
	pr, err := Open(person)
	if err != nil {
		t.Fatal(err)
	}
	pr.Config.WorkspaceID = "fedcba9876543210"
	pr.SaveConfig()
	was := mainCopy
	mainCopy = func(*Repo) string { return person }
	t.Cleanup(func() { mainCopy = was })

	agent.Actor = claudeCode
	if _, err := agent.TakeAgentLocks([]string{"tex/rock.png"}, agentlock.KindLock, "s"); err != nil {
		t.Fatalf("the person changed nothing: %v", err)
	}
	agent.UnlockAgent([]string{"tex/rock.png"})

	os.WriteFile(filepath.Join(person, "tex", "rock.png"), []byte("the person's rock"), 0o644)
	os.Remove(filepath.Join(person, "models", "ship.glb"))
	os.WriteFile(filepath.Join(person, "tex", "new.png"), []byte("new"), 0o644)
	os.WriteFile(filepath.Join(person, "notes.txt"), []byte("the person's notes"), 0o644)
	for _, p := range []string{"tex/rock.png", "models/ship.glb", "tex/new.png", "tex/"} {
		_, err := agent.TakeAgentLocks([]string{p}, agentlock.KindLock, "s")
		var refused *agentlock.Refused
		if !errors.As(err, &refused) || !refused.Locks[0].Person || refused.Locks[0].Workspace != "fedcba9876543210" {
			t.Fatalf("%s: the person holds it: %v", p, err)
		}
	}
	if _, err := agent.TakeAgentLocks([]string{"notes.txt"}, agentlock.KindLock, "s"); err != nil {
		t.Fatalf("a file that doesn't lock: %v", err)
	}
	// A claim is never refused.
	if _, err := agent.TakeAgentLocks([]string{"tex/"}, agentlock.KindClaim, "s"); err != nil {
		t.Fatal(err)
	}

	// At commit too.
	os.WriteFile(filepath.Join(agent.Root, "tex", "rock.png"), []byte("the agent's rock"), 0o644)
	var locked *ErrLockedByAgent
	if _, err := agent.Snapshot("agent"); !errors.As(err, &locked) || !locked.Locks[0].Person {
		t.Fatalf("commit: %v", err)
	}

	// The list shows what the person holds.
	all, err := agent.AllAgentLocks()
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	for _, l := range all {
		if l.Person {
			held[l.Path] = true
		}
	}
	if !held["tex/rock.png"] || !held["models/ship.glb"] || !held["tex/new.png"] || held["notes.txt"] {
		t.Fatalf("person's: %+v", all)
	}

	// Their commit settles it.
	pr.Actor = nil
	mustSnapshot(t, pr, "the person's")
	agent.Actor = claudeCode
	if _, err := agent.TakeAgentLocks([]string{"tex/"}, agentlock.KindLock, "s"); err != nil {
		t.Fatalf("committed: no longer held: %v", err)
	}
}

func TestReleaseMergedLocks(t *testing.T) {
	r := agentProject(t)
	l := r.AgentLockList()
	l.Take(other, []string{"tex/rock.png"}, agentlock.KindLock, nil)
	l.Take(agentlock.Holder{Workspace: "main-copy", Branch: "main"}, []string{"models/"}, agentlock.KindLock, nil)
	r.releaseMergedLocks("main")
	r.releaseMergedLocks("renderer")
	if all, _ := l.All(); len(all) != 1 || all[0].Branch != "main" {
		t.Fatalf("left: %+v", all)
	}
}

// claudeCode is the actor an agent's commands have (the CLI sets it).
var claudeCode = &Actor{Kind: "agent", Tool: "claude-code"}
