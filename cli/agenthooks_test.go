package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
)

// runHook runs `r3v hook <kind>` in dir as Claude Code does: input on
// stdin, no --json; its exit status, stdout and stderr.
func runHook(t *testing.T, dir, kind string, input any) (int, string, string) {
	t.Helper()
	t.Chdir(dir)
	data, _ := json.Marshal(input)
	tmp := t.TempDir()
	in := filepath.Join(tmp, "in")
	os.WriteFile(in, data, 0o644)
	stdin, _ := os.Open(in)
	defer stdin.Close()
	outF, _ := os.Create(filepath.Join(tmp, "out"))
	errF, _ := os.Create(filepath.Join(tmp, "err"))
	savedIn, savedOut, savedErr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = stdin, outF, errF
	code := Run([]string{"hook", kind})
	os.Stdin, os.Stdout, os.Stderr = savedIn, savedOut, savedErr
	outF.Close()
	errF.Close()
	out, _ := os.ReadFile(filepath.Join(tmp, "out"))
	errOut, _ := os.ReadFile(filepath.Join(tmp, "err"))
	return code, string(out), string(errOut)
}

// workspace list says which workspaces can go: merged into their base,
// nothing changed (agent tools leave them behind).
func TestWorkspaceListMerged(t *testing.T) {
	a := teamProject(t)
	ws := filepath.Join(t.TempDir(), "fix-menu")
	if exit, rep := run(t, a, "workspace", "add", ws, "--branch", "fix-menu", "--agent", "claude-code"); exit != 0 {
		t.Fatalf("add: %+v", rep.Error)
	}
	listed := func() *bool {
		exit, rep := run(t, a, "workspace", "list")
		var ls struct{ Workspaces []workspaceJSON }
		json.Unmarshal(rep.Result, &ls)
		if exit != 0 || len(ls.Workspaces) != 1 {
			t.Fatalf("list: %s %+v", rep.Result, rep.Error)
		}
		return ls.Workspaces[0].Merged
	}
	if m := listed(); m == nil || !*m {
		t.Fatalf("a new workspace: %v", m)
	}
	t.Run("in the workspace", func(t *testing.T) {
		os.WriteFile(filepath.Join(ws, "src", "menu.ts"), []byte("export const menu = 1;\n"), 0o644)
		if exit, rep := run(t, ws, "commit", "-m", "Menu"); exit != 0 {
			t.Fatalf("commit: %+v", rep.Error)
		}
	})
	if m := listed(); m == nil || *m {
		t.Fatalf("merged before merging: %v", m)
	}
	if exit, rep := run(t, a, "merge", "fix-menu"); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}
	if m := listed(); m == nil || !*m {
		t.Fatalf("not merged after merging: %v", m)
	}
}

// fakeGitWorktree makes folder look like a git worktree of the repository
// whose main folder is repo (what `git worktree add` writes for it).
func fakeGitWorktree(t *testing.T, repo, folder string) {
	t.Helper()
	gitdir := filepath.Join(repo, ".git", "worktrees", filepath.Base(folder))
	os.MkdirAll(gitdir, 0o755)
	os.MkdirAll(folder, 0o755)
	os.WriteFile(filepath.Join(folder, ".git"), []byte("gitdir: "+filepath.ToSlash(gitdir)+"\n"), 0o644)
}

// R3V run in a git worktree of an R3V project says so (git_worktree) and
// what to run instead, beside the project or inside it (Claude Code's
// .claude/worktrees: not taken for the main copy); a git worktree of a
// repository that is no R3V project is none of R3V's business.
func TestGitWorktreeStatus(t *testing.T) {
	dir, _ := agentFolder(t)
	for _, wt := range []string{filepath.Join(t.TempDir(), "wt"), filepath.Join(dir, ".claude", "worktrees", "inside")} {
		fakeGitWorktree(t, dir, wt)
		os.MkdirAll(filepath.Join(wt, "tex"), 0o755)
		exit, rep := run(t, filepath.Join(wt, "tex"), "status")
		if exit != 1 || rep.Error == nil || rep.Error.Code != "git_worktree" || len(rep.Error.Next) != 1 ||
			!strings.HasPrefix(rep.Error.Next[0].Command, "r3v workspace add --branch") ||
			!strings.Contains(rep.Error.Next[0].Why, dir) {
			t.Fatalf("%s: %d %+v", wt, exit, rep.Error)
		}
		if exit, rep := run(t, wt, "init"); exit != 1 || rep.Error.Code != "git_worktree" {
			t.Fatalf("init in %s: %d %+v", wt, exit, rep.Error)
		}
	}
	if exit, rep := run(t, dir, "status"); exit != 0 {
		t.Fatalf("the main copy: %+v", rep.Error)
	}
	plain := t.TempDir()
	os.MkdirAll(filepath.Join(plain, ".git"), 0o755)
	wt := filepath.Join(t.TempDir(), "plainwt")
	fakeGitWorktree(t, plain, wt)
	if exit, rep := run(t, wt, "status"); exit == 0 || rep.Error.Code != "not_a_project" {
		t.Fatalf("not an R3V project's: %d %+v", exit, rep.Error)
	}
}

// SessionStart and PostModelSwitch record the session's model in the
// copy; the versions Claude Code commits in that session say it.
func TestHookModel(t *testing.T) {
	dir, _ := agentFolder(t)
	t.Setenv(actor.EnvTool, "claude-code")
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-7")
	t.Setenv(actor.EnvModel, "")
	if code, out, _ := runHook(t, dir, "session-start", map[string]any{"session_id": "sess-7", "cwd": dir,
		"hook_event_name": "SessionStart", "source": "startup", "model": "claude-opus-5-5"}); code != 0 || out != "" {
		t.Fatalf("session-start: %d %q", code, out)
	}
	model := func() string {
		exit, rep := run(t, dir, "log")
		var lg struct{ Versions []versionJSON }
		json.Unmarshal(rep.Result, &lg)
		if exit != 0 || len(lg.Versions) == 0 || lg.Versions[0].Actor == nil {
			t.Fatalf("log: %s", rep.Result)
		}
		return lg.Versions[0].Actor.Model
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("two"), 0o644)
	if exit, rep := run(t, dir, "commit", "--local", "-m", "two"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	if m := model(); m != "claude-opus-5-5" {
		t.Fatalf("model %q", m)
	}
	runHook(t, dir, "model-switch", map[string]any{"session_id": "sess-7", "cwd": dir,
		"hook_event_name": "PostModelSwitch", "from_model": "claude-opus-5-5", "to_model": "claude-sonnet-5"})
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("three"), 0o644)
	run(t, dir, "commit", "--local", "-m", "three")
	if m := model(); m != "claude-sonnet-5" {
		t.Fatalf("after the switch: %q", m)
	}
}
