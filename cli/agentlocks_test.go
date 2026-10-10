package cli

import (
	"bytes"
	"encoding/json"
	"github.com/nonlabhq/r3v/internal/actor"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/docs"
	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/project"
)

// withAgentLocks turns agent locks and their commands on for a test (the
// Nightly channel has them).
func withAgentLocks(t *testing.T) {
	t.Helper()
	was := project.AgentLocks
	project.AgentLocks = true
	saved := map[string]func([]string) error{}
	for _, c := range []string{"lock", "unlock", "locks", "agent"} {
		saved[c] = extraCommands[c]
	}
	agentCommands()
	t.Cleanup(func() {
		project.AgentLocks = was
		for c, f := range saved {
			if f == nil {
				delete(extraCommands, c)
				delete(jsonCommands, c)
			} else {
				extraCommands[c] = f
			}
		}
	})
}

func agentFolder(t *testing.T) (string, *project.Repo) {
	t.Helper()
	dir := newFolder(t, map[string]string{"tex/rock.png": "rock", "tex/moss.png": "moss", "notes.txt": "n"})
	r, err := project.Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Snapshot("first"); err != nil {
		t.Fatal(err)
	}
	return dir, r
}

var otherWS = agentlock.Holder{Workspace: "0123456789abcdef", Branch: "textures", Actor: "claude-code"}

func hook(t *testing.T, in string) (hookDecision, string) {
	t.Helper()
	var out bytes.Buffer
	hookPreTool(strings.NewReader(in), &out)
	if out.Len() == 0 {
		return hookDecision{}, ""
	}
	var o hookOutput
	if err := json.Unmarshal(out.Bytes(), &o); err != nil || o.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hook output: %q %v", out.String(), err)
	}
	return o.HookSpecificOutput, out.String()
}

func hookCall(tool, path, cwd string) string {
	b, _ := json.Marshal(map[string]any{"session_id": "sess-1", "cwd": cwd, "hook_event_name": "PreToolUse",
		"tool_name": tool, "tool_input": map[string]any{"file_path": path, "old_string": "a", "new_string": "b"}})
	return string(b)
}

func TestHookPreTool(t *testing.T) {
	withAgentLocks(t)
	dir, r := agentFolder(t)
	t.Setenv(actor.EnvTool, "claude-code")
	if _, err := r.AgentLockList().Take(otherWS, []string{"tex/rock.png"}, agentlock.KindLock, nil); err != nil {
		t.Fatal(err)
	}

	// Another workspace holds it: denied, saying who.
	d, raw := hook(t, hookCall("Edit", filepath.Join(dir, "tex", "rock.png"), dir))
	if d.PermissionDecision != "deny" || !strings.Contains(d.PermissionDecisionReason, "tex/rock.png") ||
		!strings.Contains(d.PermissionDecisionReason, "textures") {
		t.Fatalf("deny: %s", raw)
	}
	// First touch of a free file: taken, nothing said (Claude Code goes on).
	if d, raw := hook(t, hookCall("Write", `tex\moss.png`, dir)); d.PermissionDecision != "" || raw != "" {
		t.Fatalf("first touch: %s", raw)
	}
	locks, _ := r.AgentLockList().All()
	if len(locks) != 2 || locks[0].Path != "tex/moss.png" || locks[0].Session != "sess-1" || locks[0].Workspace != wsID(t, dir) {
		t.Fatalf("taken: %+v", locks)
	}
	// A file that doesn't lock, under another's claim: allowed, told once.
	r.AgentLockList().Take(otherWS, []string{"notes.txt"}, agentlock.KindClaim, nil)
	d, raw = hook(t, hookCall("Edit", filepath.Join(dir, "notes.txt"), dir))
	if d.PermissionDecision != "" || !strings.Contains(d.AdditionalContext, "notes.txt") {
		t.Fatalf("claim: %s", raw)
	}
	if _, raw := hook(t, hookCall("Edit", filepath.Join(dir, "notes.txt"), dir)); raw != "" {
		t.Fatalf("told once: %s", raw)
	}
	// MultiEdit and NotebookEdit name their files too.
	nb, _ := json.Marshal(map[string]any{"tool_name": "NotebookEdit", "cwd": dir,
		"tool_input": map[string]any{"notebook_path": filepath.Join(dir, "tex", "rock.png")}})
	if d, _ := hook(t, string(nb)); d.PermissionDecision != "deny" {
		t.Fatal("NotebookEdit")
	}
	me, _ := json.Marshal(map[string]any{"tool_name": "MultiEdit", "cwd": dir,
		"tool_input": map[string]any{"file_path": filepath.Join(dir, "tex", "rock.png"), "edits": []any{map[string]any{"old_string": "a"}}}})
	if d, _ := hook(t, string(me)); d.PermissionDecision != "deny" {
		t.Fatal("MultiEdit")
	}

	// Anything unexpected: nothing said.
	outside := t.TempDir()
	for _, in := range []string{"not json", `{"tool_name": "Bash", "tool_input": {"command": "rm -rf /"}}`,
		hookCall("Edit", filepath.Join(outside, "a.png"), outside), hookCall("Edit", filepath.Join(dir, ".r3v", "config.json"), dir),
		hookCall("Edit", "relative.png", ""), `{"tool_name": "Edit", "tool_input": {"file_path": 5}}`} {
		if _, raw := hook(t, in); raw != "" {
			t.Fatalf("%s: %s", in, raw)
		}
	}

	// Stable: the hook allows everything.
	project.AgentLocks = false
	if _, raw := hook(t, hookCall("Edit", filepath.Join(dir, "tex", "rock.png"), dir)); raw != "" {
		t.Fatalf("Stable: %s", raw)
	}
}

func TestLockCommands(t *testing.T) {
	withAgentLocks(t)
	dir, r := agentFolder(t)
	t.Setenv(actor.EnvTool, "claude-code")
	r.AgentLockList().Take(otherWS, []string{"tex/rock.png"}, agentlock.KindLock, nil)

	// All or none.
	exit, rep := run(t, dir, "lock", "tex/moss.png", "tex/rock.png")
	if exit != 1 || rep.Error.Code != "files_locked_by_agent" || len(rep.Error.Locks) != 1 ||
		rep.Error.Locks[0].Path != "tex/rock.png" || rep.Error.Locks[0].Holder != "agent" || rep.Error.Locks[0].Branch != "textures" {
		t.Fatalf("refused: %+v", rep.Error)
	}
	exit, rep = run(t, dir, "lock", "tex/moss.png", "notes.txt")
	var res lockResultJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || len(res.Taken) != 2 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "notes.txt") {
		t.Fatalf("lock: %s", rep.Result)
	}
	if exit, _ := run(t, dir, "lock", "--claim", "tex"); exit != 0 {
		t.Fatal("claim")
	}
	exit, rep = run(t, dir, "locks")
	var ls struct {
		Workspace string
		Locks     []struct {
			Path, Kind string
			Mine       bool
			Stale      bool
		}
	}
	json.Unmarshal(rep.Result, &ls)
	if exit != 0 || len(ls.Locks) != 4 || ls.Workspace != wsID(t, dir) {
		t.Fatalf("locks: %s", rep.Result)
	}
	mine := 0
	for _, l := range ls.Locks {
		if l.Mine {
			mine++
		}
	}
	if mine != 3 {
		t.Fatalf("mine: %s", rep.Result)
	}
	if exit, rep := run(t, dir, "unlock", "tex/rock.png"); exit != 0 || !strings.Contains(string(rep.Result), `"unlocked": []`) {
		t.Fatalf("unlock is the holder's: %s", rep.Result)
	}
	if exit, rep := run(t, dir, "lock", "--break", "tex/rock.png"); exit != 0 || !strings.Contains(string(rep.Result), "textures") {
		t.Fatalf("break: %s", rep.Result)
	}
	if exit, _ := run(t, dir, "unlock", "tex/moss.png", "notes.txt", "tex"); exit != 0 {
		t.Fatal("unlock")
	}
	if all, _ := r.AgentLockList().All(); len(all) != 0 {
		t.Fatalf("left: %+v", all)
	}
	if exit, rep := run(t, dir, "lock", "../elsewhere.png"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("outside: %+v", rep.Error)
	}

	// A commit by an agent of a file another holds.
	r.AgentLockList().Take(otherWS, []string{"tex/"}, agentlock.KindLock, nil)
	os.WriteFile(filepath.Join(dir, "tex", "rock.png"), []byte("rock 2"), 0o644)
	exit, rep = run(t, dir, "commit", "--local", "-m", "agent")
	if exit != 1 || rep.Error.Code != "files_locked_by_agent" || rep.Error.Locks[0].Path != "tex/" {
		t.Fatalf("commit: %+v", rep.Error)
	}
	t.Setenv(actor.EnvTool, actor.None)
	if exit, rep := run(t, dir, "commit", "--local", "-m", "person"); exit != 0 {
		t.Fatalf("a person's commit: %+v", rep.Error)
	}
}

func TestAgentSetup(t *testing.T) {
	withAgentLocks(t)
	dir, _ := agentFolder(t)
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o755)
	existing := `{
  "permissions": {"allow": ["Bash(npm test)"]},
  "env": {"FOO": "bar"},
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo hi"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]
  }
}`
	os.WriteFile(filepath.Join(dir, ".claude", "settings.local.json"), []byte(existing), 0o644)
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# My project\nBe nice."), 0o644)

	exit, rep := run(t, dir, "agent", "setup")
	if exit != 0 || !strings.Contains(string(rep.Result), "settings.local.json") || !strings.Contains(string(rep.Result), "CLAUDE.md") {
		t.Fatalf("setup: %s %+v", rep.Result, rep.Error)
	}
	read := func() map[string]any {
		data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
		var s map[string]any
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := read()
	env := s["env"].(map[string]any)
	if env["FOO"] != "bar" || env["R3V_ACTOR"] != "claude-code" || s["permissions"] == nil {
		t.Fatalf("env and the rest kept: %v", s)
	}
	hooks := s["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if hooks["Stop"] == nil || len(pre) != 2 {
		t.Fatalf("hooks: %v", hooks)
	}
	ours := pre[1].(map[string]any)
	cmd := ours["hooks"].([]any)[0].(map[string]any)["command"].(string)
	if ours["matcher"] != "Edit|Write|MultiEdit|NotebookEdit" || !strings.HasSuffix(cmd, "hook pre-tool") ||
		!strings.Contains(cmd, filepath.ToSlash(os.Args[0][:2])) {
		t.Fatalf("our hook (this executable, by its path): %v", ours)
	}
	// The guide in AGENTS.md (every tool's), imported by CLAUDE.md.
	guide, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(guide) != "# My project\nBe nice.\n\n@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md: %q", guide)
	}
	if agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); string(agents) != docs.AgentsSnippet {
		t.Fatalf("AGENTS.md: %q", agents)
	}

	// Again: nothing changes.
	exit, rep = run(t, dir, "agent", "setup", "--tool", "claude-code")
	if exit != 0 || !strings.Contains(string(rep.Result), `"changed": []`) {
		t.Fatalf("again: %s", rep.Result)
	}
	if again := read(); len(again["hooks"].(map[string]any)["PreToolUse"].([]any)) != 2 {
		t.Fatal("hook added twice")
	}
	if g2, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); string(g2) != string(guide) {
		t.Fatal("guide added twice")
	}

	// Settings R3V can't read are left alone.
	os.WriteFile(filepath.Join(dir, ".claude", "settings.local.json"), []byte("{broken"), 0o644)
	if exit, _ := run(t, dir, "agent", "setup"); exit == 0 {
		t.Fatal("broken settings overwritten")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json")); string(data) != "{broken" {
		t.Fatal("changed")
	}
	if exit, rep := run(t, dir, "agent", "setup", "--tool", "codex"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("unknown tool: %+v", rep.Error)
	}
}

// wsID is the folder's workspace id (made by the first command that
// needed it).
func wsID(t *testing.T, dir string) string {
	t.Helper()
	r, err := project.Open(dir)
	if err != nil || r.Config.WorkspaceID == "" {
		t.Fatalf("no workspace id: %v", err)
	}
	return r.Config.WorkspaceID
}
