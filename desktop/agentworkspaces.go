package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/store"
	"github.com/nonlabhq/r3v/internal/teams"
)

// The Agents view (docs/design/agent-workspaces.md, "The overview";
// while project.AgentWorkspaces is): a project's workspaces on this
// computer, each an agent's folder on a branch of its own, with what each
// has to merge; made, merged and removed from the app. (Workspaces, in
// workrecord.go, is something else: the team's other copies.)

// AgentWorkspace is a workspace of a project, as the Agents view shows it.
type AgentWorkspace struct {
	Folder    string `json:"folder"`
	Branch    string `json:"branch"` // its branch's key
	Label     string `json:"label"`  // what its branch is called
	Base      string `json:"base"`   // the branch it was made from, and merges into
	BaseLabel string `json:"baseLabel"`
	Created   string `json:"created"`
	// Ahead: its versions not in the base branch yet (to merge); Behind:
	// the base branch's not in it. Nil when the team can't be reached.
	Ahead   *int `json:"ahead"`
	Behind  *int `json:"behind"`
	Changes int  `json:"changes"` // files changed, not committed
	Parked  int  `json:"parked"`  // sets of parked changes
	// LastActivity: its last commit, switch, change or word from its agent
	// (RFC 3339).
	LastActivity string `json:"lastActivity"`
	// Missing: the folder is gone (removing takes it off the list).
	Missing bool `json:"missing"`
	// Locks: the agent locks it holds (claims left out: they hold nothing).
	Locks []LockItem `json:"locks"`
	// Tool: the agent tool it is set up for, or that took its locks
	// ("claude-code"; "" not known). Owner: the member its branch works for.
	Tool  string `json:"tool"`
	Owner string `json:"owner"`
	// Stale: its agent took locks and hasn't been heard of for a while.
	Stale bool `json:"stale"`
	// Opened: shown in the app as a project (OpenWorkspace).
	Opened bool `json:"opened"`
	// Results: what its builds and tests said about the version it is on
	// (r3v result), by check.
	Results map[string]project.Result `json:"results"`
	// Steps: how many steps its sessions kept (an agent's moves).
	Steps int `json:"steps"`
}

// AgentWorkspaces lists project root's workspaces on this computer, with
// where each is against its base branch (the team is asked; out of reach,
// Ahead and Behind are nil).
func (a *App) AgentWorkspaces(root string) ([]AgentWorkspace, error) {
	sw := startWatch("AgentWorkspaces " + filepath.Base(root))
	defer sw.done()
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	list, err := r.WorkspaceStatuses()
	if err != nil {
		return nil, err
	}
	sw.lap("statuses")
	recs := branchRecords(r, false)
	if recs == nil {
		recs = branchRecords(r, true)
	}
	var names map[string]string
	var locks []agentlock.Lock
	if len(list) > 0 {
		names = a.memberNames(r)
		locks, _ = r.AgentLockList().All()
	}
	opened := map[string]bool{}
	if store, err := teams.Load(); err == nil {
		for _, f := range store.Workspaces {
			opened[strings.ToLower(filepath.Clean(f))] = true
		}
	}
	out := []AgentWorkspace{}
	for _, s := range list {
		w := AgentWorkspace{Folder: s.Folder, Branch: s.Branch, Label: s.BranchName, Base: s.Base,
			BaseLabel: r.BranchLabel(s.Base), Created: s.Created.UTC().Format(time.RFC3339),
			Ahead: s.Ahead, Behind: s.Behind, Changes: s.Changes, Parked: s.Parked, Missing: s.Missing,
			Locks: []LockItem{}, Opened: opened[strings.ToLower(filepath.Clean(s.Folder))], Results: s.Results}
		if w.Results == nil {
			w.Results = map[string]project.Result{}
		}
		if ws, err := project.Open(s.Folder); err == nil && !s.Missing {
			if all, err := ws.Sessions(); err == nil {
				for _, x := range all {
					w.Steps += x.Steps
				}
			}
		}
		if rec, ok := recs[s.Branch]; ok && rec.Owner != "" {
			w.Owner = names[rec.Owner]
			if w.Owner == "" {
				w.Owner = rec.Owner
			}
		}
		last := s.LastActivity
		id := ""
		if !s.Missing {
			if ws, err := project.Open(s.Folder); err == nil {
				id = ws.Config.WorkspaceID
			}
			w.Tool = agentsetup.CopyTool(s.Folder)
		}
		for _, l := range locks {
			// Its own: by its id; a folder that can't be read, by its branch.
			if l.Person || l.Workspace == "" || (id != "" && l.Workspace != id) || (id == "" && l.Branch != s.Branch) {
				continue
			}
			if l.Heartbeat.After(last) {
				last = l.Heartbeat
			}
			if l.Kind != agentlock.KindLock {
				continue
			}
			w.Locks = append(w.Locks, agentLockItem(r, l))
			if l.Actor != "" {
				w.Tool = l.Actor
			}
			w.Stale = w.Stale || l.Stale
		}
		w.LastActivity = last.UTC().Format(time.RFC3339)
		out = append(out, w)
	}
	return out, nil
}

// WorkspaceAdded is a workspace just made.
type WorkspaceAdded struct {
	Folder string `json:"folder"`
	Branch string `json:"branch"` // its branch's key
	Label  string `json:"label"`
	// Agent: the agent tool it was set up for ("" none). SetupError: the
	// workspace is made, but setting the tool up failed (why).
	Agent      string `json:"agent"`
	SetupError string `json:"setupError"`
}

// AddWorkspace makes folder a workspace of project root, on a new branch
// called branch, from branch or version from ("": where the project is),
// and sets it up for the agent tool agent ("" none, or "claude-code"). The
// files come from this computer, not downloaded again.
func (a *App) AddWorkspace(root, folder, branch, from, agent string) (*WorkspaceAdded, error) {
	if !project.AgentWorkspaces {
		return nil, project.ErrWorkspacesNotInBuild
	}
	if agent != "" && agent != agentsetup.ClaudeCodeTool {
		return nil, errors.New("R3V sets up Claude Code only, for now")
	}
	folder = strings.TrimSpace(folder)
	if folder == "" || !filepath.IsAbs(folder) {
		return nil, errors.New("choose a folder for the workspace")
	}
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	e, err := r.AddWorkspace(folder, project.WorkspaceOptions{Branch: branch, From: from, Agent: agent != ""})
	unlock()
	if err != nil {
		return nil, err
	}
	branchRecords(r, true) // (its branch, named)
	out := &WorkspaceAdded{Folder: e.Folder, Branch: e.Branch, Label: r.BranchLabel(e.Branch)}
	if agent != "" {
		out.Agent = agent
		if _, err := agentsetup.ClaudeCode(e.Folder, agentsetup.HookCommand(cliPath()), false, project.Steps); err != nil {
			out.SetupError = err.Error()
		}
	}
	return out, nil
}

// cliPath is the r3v program installed with the app (bin\r3v.exe beside
// it), "" when there is none (a build run from source): the hook then runs
// the r3v on PATH.
func cliPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(exe), "bin", "r3v.exe")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// WorkspaceRemoval is what removing a workspace did: removed, or refused
// for the work it has (Left; nothing was changed).
type WorkspaceRemoval struct {
	Removed bool           `json:"removed"`
	Left    *WorkspaceLeft `json:"left"`
}

// WorkspaceLeft is the work removing a workspace would lose.
type WorkspaceLeft struct {
	Unmerged int    `json:"unmerged"` // versions not merged into Base
	Changes  int    `json:"changes"`  // files changed, not committed
	Parked   int    `json:"parked"`   // sets of parked changes
	Base     string `json:"base"`     // what its base branch is called
}

// RemoveWorkspace takes workspace folder of project root away: its folder
// deleted, its agent locks freed, its branch archived (its versions stay on
// the team). With work not merged, not committed or parked it is refused
// (Left says what), unless force, which loses that work.
func (a *App) RemoveWorkspace(root, folder string, force bool) (*WorkspaceRemoval, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	err = r.RemoveWorkspace(folder, force)
	unlock()
	var nd *project.WorkspaceNotDoneError
	if errors.As(err, &nd) {
		return &WorkspaceRemoval{Left: &WorkspaceLeft{Unmerged: nd.Unmerged, Changes: nd.Changes, Parked: nd.Parked, Base: nd.Base}}, nil
	}
	if err != nil {
		return nil, err
	}
	a.forgetWorkspace(folder)
	branchRecords(r, true)
	a.refetchTeam(r)
	a.emitLocks(root)
	return &WorkspaceRemoval{Removed: true}, nil
}

// forgetWorkspace shows folder as a project no more.
func (a *App) forgetWorkspace(folder string) {
	teams.Update(func(s *teams.Store) error {
		s.RemoveWorkspace(filepath.Clean(folder))
		return nil
	})
}

// OpenWorkspace shows workspace folder of project root in the app as a
// project, beside the project (to look inside: its versions, its files,
// what its agent changed). It returns it as the sidebar lists it.
func (a *App) OpenWorkspace(root, folder string) (TeamProject, error) {
	r, err := project.Open(root)
	if err != nil {
		return TeamProject{}, err
	}
	list, err := r.LocalWorkspaces()
	if err != nil {
		return TeamProject{}, err
	}
	folder = filepath.Clean(folder)
	known := false
	for _, e := range list {
		known = known || strings.EqualFold(filepath.Clean(e.Folder), folder)
	}
	ws, err := project.Open(folder)
	if !known || err != nil || !ws.IsWorkspace() || !strings.EqualFold(filepath.Clean(ws.CommonDir()), filepath.Clean(r.CommonDir())) {
		return TeamProject{}, errors.New("that folder isn't a workspace of this project")
	}
	if _, err := teams.Update(func(s *teams.Store) error {
		s.AddWorkspace(ws.Root)
		return nil
	}); err != nil {
		return TeamProject{}, err
	}
	return workspaceProject(ws.Root), nil
}

// workspaceProject is a workspace as the sidebar lists it.
func workspaceProject(folder string) TeamProject {
	p := folderProject(folder, "downloaded")
	p.Workspace = true
	if r, err := project.Open(folder); err == nil && r.IsWorkspace() {
		p.MainRoot = r.MainRoot()
	}
	return p
}

// openedWorkspaces are the workspaces shown as projects whose main copy is
// one of mains (a team's projects here), each after its main copy's: every
// workspace of a project here (one an agent made from the command line too:
// the app then knows its folder), the sidebar folding them under it.
func openedWorkspaces(store *teams.Store, mains []TeamProject) []TeamProject {
	var found []string
	for _, m := range mains {
		if m.Status != "downloaded" || m.Root == "" || m.Workspace {
			continue
		}
		r, err := project.Open(m.Root)
		if err != nil {
			continue
		}
		list, _ := r.LocalWorkspaces()
		for _, e := range list {
			if !slices.ContainsFunc(store.Workspaces, func(f string) bool { return strings.EqualFold(filepath.Clean(f), filepath.Clean(e.Folder)) }) {
				if fi, err := os.Stat(e.Folder); err == nil && fi.IsDir() {
					found = append(found, e.Folder)
				}
			}
		}
	}
	if len(found) > 0 {
		if s, err := teams.Update(func(s *teams.Store) error {
			for _, f := range found {
				s.AddWorkspace(f)
			}
			return nil
		}); err == nil {
			store = s
		}
	}
	var out []TeamProject
	for _, folder := range store.Workspaces {
		p := workspaceProject(folder)
		for _, m := range mains {
			if p.MainRoot != "" && m.Root != "" && strings.EqualFold(filepath.Clean(m.Root), filepath.Clean(p.MainRoot)) {
				p.Icon, p.Color = m.Icon, m.Color
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// AddAgentGuide puts the agents' guide in the project (AGENTS.md, and a
// line importing it in CLAUDE.md), to be committed: every workspace then
// has it. It returns the files it wrote.
func (a *App) AddAgentGuide(root string) ([]string, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if r.IsWorkspace() {
		return nil, errors.New("the guide goes in the project's main copy (committed, every workspace has it)")
	}
	written, err := agentsetup.Guide(r.Root)
	return nonNil(written), err
}

// AgentGuide says whether the project in root has the agents' guide: "has"
// (AGENTS.md has R3V's part), "theirs" (an AGENTS.md or CLAUDE.md of the
// project's own, without it: R3V's part goes at its end), "" (none yet).
func (a *App) AgentGuide(root string) string {
	if agentsetup.HasGuide(root) {
		return "has"
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return "theirs"
		}
	}
	return ""
}

// WorkspaceRoom is the room a new workspace takes.
type WorkspaceRoom struct {
	Files  int64 `json:"files"`  // the version's files
	Copies int64 `json:"copies"` // the build folders .r3v.yaml names (workspaces: copy)
	// Clones: the folder's drive copies by block cloning (ReFS, Dev Drive):
	// no room taken until a file changes.
	Clones bool `json:"clones"`
}

// WorkspaceRoom estimates the room a new workspace of the project in root
// takes in folder (the folder needn't exist yet).
func (a *App) WorkspaceRoom(root, folder string) (*WorkspaceRoom, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	files, copies := r.WorkspaceEstimate()
	return &WorkspaceRoom{Files: files, Copies: copies, Clones: folder != "" && store.Clones(folder)}, nil
}
