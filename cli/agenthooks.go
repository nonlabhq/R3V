package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
)

// Passive Claude Code hooks (docs/design/agent-identity.md): SessionStart
// and PostModelSwitch only record the session's model (no variable says
// it) for the versions it commits. And R3V protecting itself: its commands
// refuse a git worktree of an R3V project (git_worktree), where they would
// take the folder for the main copy or for none.

// modelHookIn is what Claude Code tells a SessionStart (model) or
// PostModelSwitch (to_model) hook.
type modelHookIn struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	Model     json.RawMessage `json:"model"`
	ToModel   json.RawMessage `json:"to_model"`
}

// hookModel records the session's model in the copy's .r3v (actor.
// RecordModel): what its versions say. Never in the way: any error says
// nothing.
func hookModel(in io.Reader) {
	var h modelHookIn
	if json.NewDecoder(io.LimitReader(in, 1<<20)).Decode(&h) != nil || h.SessionID == "" {
		return
	}
	model := modelName(h.ToModel)
	if model == "" {
		model = modelName(h.Model)
	}
	if model == "" {
		return
	}
	dir := h.Cwd
	if dir == "" {
		dir = "."
	}
	r, err := project.Open(dir)
	if err != nil {
		return
	}
	actor.RecordModel(r.Dir, agentsetup.ClaudeCodeTool, h.SessionID, model)
}

// modelName is a model as a hook gives it: its name, or an object with
// its id (or display name).
func modelName(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var o struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	if json.Unmarshal(raw, &o) == nil {
		if o.ID != "" {
			return strings.TrimSpace(o.ID)
		}
		return strings.TrimSpace(o.DisplayName)
	}
	return ""
}

// gitWorktreeOf is the git worktree dir is in (its folder: a .git file
// pointing at <repo>/.git/worktrees/<name>) when the repository's main
// folder holds an R3V project there: the worktree and that project's
// folder. "", "" otherwise (a copy of R3V's own, with its .r3v, first).
func gitWorktreeOf(dir string) (worktree, main string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", ""
	}
	for d := abs; ; {
		if isDir(filepath.Join(d, ".r3v")) {
			return "", ""
		}
		if fi, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			if fi.IsDir() {
				return "", "" // a repository's main folder
			}
			data, err := os.ReadFile(filepath.Join(d, ".git"))
			if err != nil {
				return "", ""
			}
			line, _, _ := strings.Cut(string(data), "\n")
			gitdir, ok := strings.CutPrefix(strings.TrimSpace(line), "gitdir:")
			if !ok {
				return "", ""
			}
			gitdir = strings.TrimSpace(gitdir)
			if !filepath.IsAbs(gitdir) {
				gitdir = filepath.Join(d, gitdir)
			}
			gitdir = filepath.Clean(gitdir)
			common := filepath.Dir(filepath.Dir(gitdir))
			if !strings.EqualFold(filepath.Base(filepath.Dir(gitdir)), "worktrees") || !strings.EqualFold(filepath.Base(common), ".git") {
				return "", "" // a submodule, or a bare repository's
			}
			repo := filepath.Dir(common)
			rel, err := filepath.Rel(d, abs)
			if err != nil {
				return "", ""
			}
			for p := filepath.Join(repo, rel); ; p = filepath.Dir(p) {
				if isDir(filepath.Join(p, ".r3v")) {
					return d, p
				}
				if len(p) <= len(repo) || filepath.Dir(p) == p {
					return "", ""
				}
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", ""
		}
		d = parent
	}
}

// notGitWorktree refuses to work in a git worktree of an R3V project
// (git_worktree): R3V would take it for the project's main copy, or for
// no project, and its work for none of the project's.
func notGitWorktree(dir string) error {
	wt, main := gitWorktreeOf(dir)
	if main == "" {
		return nil
	}
	tool := "<your tool>"
	if a := actor.FromEnv(os.Getenv); a != nil {
		tool = a.Tool
	}
	return &cliError{Code: "git_worktree", Exit: exitError,
		Err:  fmt.Errorf("%s is a git worktree of the R3V project in %s: R3V doesn't see work here (it isn't committed, shared or merged)", wt, main),
		Hint: "work in an R3V workspace instead: its own folder and branch, which R3V sees",
		Next: []nextJSON{{Command: "r3v workspace add --branch NAME --agent " + tool,
			Why: "run in the project's main copy (" + main + "): makes a workspace (folder, branch, agent setup); start your tool in that folder and move the work there, then ask the user before removing this git worktree"}}}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
