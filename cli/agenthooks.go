package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Claude Code's worktrees as R3V workspaces (docs/design/agent-workspaces.md,
// "Claude Code's worktrees"): its WorktreeCreate and WorktreeRemove hooks
// make and remove workspaces (`claude --worktree`, subagents' isolation),
// the pre-tool hook turns `git worktree` away, and a git worktree of an R3V
// project is told so. Its SessionStart and PostModelSwitch hooks record the
// session's model (no variable says it) for the versions it commits.

// worktreeHookIn is what Claude Code tells a WorktreeCreate or
// WorktreeRemove hook (only what's used).
type worktreeHookIn struct {
	SessionID    string `json:"session_id"`
	Cwd          string `json:"cwd"`
	Name         string `json:"name"`
	WorktreePath string `json:"worktree_path"`
}

func readWorktreeHook(in io.Reader) (worktreeHookIn, error) {
	var h worktreeHookIn
	if err := json.NewDecoder(io.LimitReader(in, 1<<20)).Decode(&h); err != nil {
		return h, fmt.Errorf("R3V: the hook's input isn't JSON R3V can read: %w", err)
	}
	if h.Cwd == "" {
		h.Cwd = "."
	}
	return h, nil
}

// hookWorktreeCreate makes the workspace Claude Code asked for (named
// after the worktree's name, its branch an agent's, set up for Claude
// Code) and writes its folder, absolute, alone on out: Claude Code reads
// the last line. Everything else goes to stderr. The same name again is
// that workspace. An error fails the worktree's creation.
func hookWorktreeCreate(in io.Reader, out io.Writer) error {
	stdout := os.Stdout
	os.Stdout = os.Stderr // (only the folder on stdout)
	defer func() { os.Stdout = stdout }()
	h, err := readWorktreeHook(in)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(h.Name)
	if name == "" {
		return errors.New("R3V: Claude Code gave no name for the worktree")
	}
	r, err := openRepoAt(h.Cwd)
	if err != nil {
		return fmt.Errorf("R3V can't make a workspace for this worktree: %w", err)
	}
	folder := ""
	list, err := r.LocalWorkspaces()
	if err != nil {
		return err
	}
	for _, e := range list {
		if remote.SameBranchName(r.BranchLabel(e.Branch), name) && isDir(filepath.Join(e.Folder, ".r3v")) {
			folder = e.Folder
			fmt.Fprintf(os.Stderr, "R3V: the workspace for %s is there already: %s\n", name, folder)
			break
		}
	}
	if folder == "" {
		e, err := r.AddWorkspace(r.DefaultWorkspaceFolder(name), project.WorkspaceOptions{Branch: name, Agent: true})
		if err != nil {
			return fmt.Errorf("R3V can't make a workspace for this worktree: %w", err)
		}
		folder = e.Folder
		fmt.Fprintf(os.Stderr, "R3V: workspace %s on branch %s (from %s)\n", folder, r.BranchLabel(e.Branch), r.BranchLabel(e.Base))
	}
	ws := filepath.Join(folder, ".r3v")
	if _, _, err := agentsetup.Setup(folder, ws, agentsetup.Options{Tool: agentsetup.ClaudeCodeTool,
		Hook: hookCommand(), Steps: project.Steps}); err != nil {
		return fmt.Errorf("R3V made the workspace %s, but setting Claude Code up there failed: %w", folder, err)
	}
	// A subagent runs in its parent's session: its model, as recorded here.
	if m := actor.SessionModel(r.Dir, agentsetup.ClaudeCodeTool, h.SessionID); m != "" {
		actor.RecordModel(ws, agentsetup.ClaudeCodeTool, h.SessionID, m)
	}
	_, err = fmt.Fprintln(out, folder)
	return err
}

// hookWorktreeRemove removes the workspace Claude Code is done with, as
// `workspace remove` does without --force: refused (non-zero, the folder
// kept) while it has work not merged into its base branch, changes not
// committed or parked. Never deletes unmerged work.
func hookWorktreeRemove(in io.Reader) error {
	h, err := readWorktreeHook(in)
	if err != nil {
		return err
	}
	if h.WorktreePath == "" {
		return errors.New("R3V: Claude Code gave no worktree to remove")
	}
	folder, err := filepath.Abs(h.WorktreePath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(folder); errors.Is(err, os.ErrNotExist) {
		return nil // gone already
	}
	if !isDir(filepath.Join(folder, ".r3v")) {
		return fmt.Errorf("R3V: %s is not one of R3V's workspaces, so R3V leaves it as it is (ask the user before deleting it)", folder)
	}
	ws, err := project.Open(folder)
	if err != nil || !ws.IsWorkspace() {
		return fmt.Errorf("R3V: %s is not a workspace R3V can remove here: run `r3v workspace remove %s` in the project's main copy, or ask the user", folder, folder)
	}
	main := ws.MainRoot()
	os.Chdir(main) // (nothing of this process holds the folder open)
	r, err := openRepoAt(main)
	if err != nil {
		return err
	}
	err = r.RemoveWorkspace(folder, false)
	var nd *project.WorkspaceNotDoneError
	if errors.As(err, &nd) {
		branch := ws.BranchLabel(ws.BranchName())
		return &cliError{Code: "workspace_not_merged", Exit: exitError, Err: fmt.Errorf(
			"R3V kept the workspace, nothing was deleted: %v. Merge its branch first (`r3v merge %s` in the project's main copy, %s) and commit or discard its changes, or ask the user what to do with it",
			nd, branch, main)}
	}
	if err != nil {
		return fmt.Errorf("R3V didn't remove the workspace %s: %w", folder, err)
	}
	fmt.Fprintf(os.Stderr, "R3V: removed the workspace %s (its branch, merged, is archived)\n", folder)
	return nil
}

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

// --- git worktree ---------------------------------------------------------

// gitWorktreeCmd finds `git worktree add` or `remove` in a shell command
// (with git's own options before it: -C dir, -c key=value, --no-pager).
var gitWorktreeCmd = regexp.MustCompile(`(?i)(?:^|[\s;&|(` + "`" + `{])git(?:\.exe)?(?:\s+(?:-C|-c)\s+(?:"[^"]*"|'[^']*'|\S+)|\s+--?[\w-]+(?:=\S+)?)*\s+worktree\s+(add|remove)\b`)

// gitDirOption is git's -C, the folder it works in.
var gitDirOption = regexp.MustCompile(`\bgit(?:\.exe)?\s+(?:.*?\s)?-C\s+("[^"]*"|'[^']*'|\S+)`)

// gitWorktreeDenial is why a shell command making or removing a git
// worktree in an R3V project is refused ("" when it isn't one).
func gitWorktreeDenial(command, cwd string) string {
	m := gitWorktreeCmd.FindStringSubmatch(command)
	if m == nil {
		return ""
	}
	dirs := []string{cwd}
	if d := gitDirOption.FindStringSubmatch(command); d != nil {
		p := strings.Trim(d[1], `"'`)
		if !filepath.IsAbs(p) && cwd != "" {
			p = filepath.Join(cwd, p)
		}
		dirs = append(dirs, p)
	}
	inProject := false
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := project.Open(d); err == nil {
			inProject = true
		} else if _, main := gitWorktreeOf(d); main != "" {
			inProject = true
		}
	}
	if !inProject {
		return ""
	}
	if strings.EqualFold(m[1], "remove") {
		return "R3V: this project uses R3V workspaces, not git worktrees: remove a workspace with `r3v workspace remove <folder>` " +
			"in the project's main copy (it refuses while its work isn't merged). A git worktree made before: ask the user."
	}
	return "R3V: this project uses R3V workspaces: run `r3v workspace add --branch <name> --agent claude-code` instead " +
		"(the folder, its branch and the agent setup in one go). R3V doesn't see a git worktree: its work wouldn't be committed, shared or merged."
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
			Why: "run in the project's main copy (" + main + "): makes a workspace (folder, branch, agent setup); move the work there, then ask the user before removing this git worktree"}}}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
