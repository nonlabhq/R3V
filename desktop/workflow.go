package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Workflow settings (docs/design/team-settings.md): the team's defaults,
// a project's own over them. On an R3V Cloud team its admins change them;
// on a team's own storage, anyone in it.

// AgentWorkflow is what a team or a project says about agents.
type AgentWorkflow struct {
	Steps      bool `json:"steps"`      // agents' steps are recorded
	Prompts    bool `json:"prompts"`    // with what was asked
	AgentLocks bool `json:"agentLocks"` // workspaces take turns on files that can't be merged
}

// WorkflowOverride is a project setting its own.
type WorkflowOverride struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Locks     bool   `json:"locks"`
	Agents    bool   `json:"agents"`
}

// TeamWorkflowView is a team's Workflow tab.
type TeamWorkflowView struct {
	CanEdit bool `json:"canEdit"`
	// Locks: file locking can be had (an R3V Cloud team, a build with it);
	// LocksOn: projects lock unless they say otherwise; Kinds: those that
	// lock by themselves.
	Locks   bool               `json:"locks"`
	LocksOn bool               `json:"locksOn"`
	Kinds   []string           `json:"kinds"`
	All     []profile.LockKind `json:"all"`
	Agents  AgentWorkflow      `json:"agents"`
	// Overrides: the projects setting some of their own.
	Overrides []WorkflowOverride `json:"overrides"`
}

// ProjectWorkflowView is a project's Workflow section: the team's, the
// project's own (nil: as the team's), and this computer's choices.
type ProjectWorkflowView struct {
	Team    bool `json:"team"` // the project is a team's (else all is this computer's)
	CanEdit bool `json:"canEdit"`
	Locks   bool `json:"locks"` // file locking can be had
	// The team's locks (on by default, kinds) and the project's own.
	TeamLocksOn bool                 `json:"teamLocksOn"`
	TeamKinds   []string             `json:"teamKinds"`
	OwnLocks    *remote.ProjectLocks `json:"ownLocks"`
	All         []profile.LockKind   `json:"all"`
	// The team's agents' settings and the project's own (nil: as the team).
	TeamAgents AgentWorkflow `json:"teamAgents"`
	Steps      *bool         `json:"steps"`
	Prompts    *bool         `json:"prompts"`
	AgentLocks *bool         `json:"agentLocks"`
	// Mine: this computer's own choices about steps (only ever stricter).
	Mine project.StepSettings `json:"mine"`
	// RulesLocks: the project's .r3v.yaml still has file_locks: (no longer
	// read: offered to move here).
	RulesLocks bool `json:"rulesLocks"`
}

func agentsOf(w remote.Workflow) AgentWorkflow {
	return AgentWorkflow{Steps: remote.Value(w.Steps, nil, true), Prompts: remote.Value(w.Prompts, nil, true),
		AgentLocks: remote.Value(w.AgentLocks, nil, true)}
}

// canEditWorkflow: an R3V Cloud team's admins, anyone on a team's own storage.
func canEditWorkflow(t *teams.Team) bool {
	if _, ok := cloud.Hosted(*t); ok {
		return isAdmin(myRole(t))
	}
	return true
}

func findTeam(teamID string) (*teams.Store, *teams.Team, error) {
	store, err := teams.Load()
	if err != nil {
		return nil, nil, err
	}
	t := store.Find(teamID)
	if t == nil {
		return nil, nil, errors.New("unknown team")
	}
	return store, t, nil
}

// TeamWorkflow reads a team's workflow settings.
func (a *App) TeamWorkflow(teamID string) (*TeamWorkflowView, error) {
	_, t, err := findTeam(teamID)
	if err != nil {
		return nil, err
	}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	info, err := b.Info()
	if err != nil {
		return nil, err
	}
	ps, err := b.Projects()
	if err != nil {
		return nil, err
	}
	out := &TeamWorkflowView{CanEdit: canEditWorkflow(t), Kinds: []string{}, All: profile.LockKinds, Overrides: []WorkflowOverride{}}
	if info.Workflow != nil {
		out.Agents = agentsOf(*info.Workflow)
	} else {
		out.Agents = agentsOf(remote.Workflow{})
	}
	if _, ok := cloud.Hosted(*t); ok && remote.FileLocks {
		s := remote.LocksOf(info)
		out.Locks, out.LocksOn, out.Kinds = true, remote.LocksDefault(s), s.Kinds
		if !s.On && len(s.Kinds) == 0 {
			out.Kinds = profile.DefaultLockKinds() // (what turning it on starts with)
		}
	}
	for _, p := range ps {
		if p.Workflow.Empty() {
			continue
		}
		w := p.Workflow
		o := WorkflowOverride{ProjectID: p.ID, Name: p.Name, Locks: w.Locks != nil && out.Locks,
			Agents: w.Steps != nil || w.Prompts != nil || w.AgentLocks != nil}
		if o.Locks || o.Agents {
			out.Overrides = append(out.Overrides, o)
		}
	}
	return out, nil
}

// SetTeamAgents sets the team's defaults for agents.
func (a *App) SetTeamAgents(teamID string, w AgentWorkflow) error {
	_, t, err := findTeam(teamID)
	if err != nil {
		return err
	}
	if !canEditWorkflow(t) {
		return remote.ErrNotAdmin
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	// (R3V's defaults are left unset: a team that never changed them
	// needs no feature)
	off := func(on bool) *bool {
		if on {
			return nil
		}
		f := false
		return &f
	}
	if err := remote.SetTeamWorkflow(b, remote.Workflow{Steps: off(w.Steps), Prompts: off(w.Prompts), AgentLocks: off(w.AgentLocks)}); err != nil {
		return err
	}
	a.workflowChanged(t, "")
	return nil
}

// ClearProjectWorkflow makes every project follow the team again in pick
// ("locks", "agents"), and says how many did not.
func (a *App) ClearProjectWorkflow(teamID, pick string) (int, error) {
	_, t, err := findTeam(teamID)
	if err != nil {
		return 0, err
	}
	if !canEditWorkflow(t) {
		return 0, remote.ErrNotAdmin
	}
	b, err := t.Open()
	if err != nil {
		return 0, err
	}
	n, err := remote.ClearProjectWorkflow(b, pick)
	if n > 0 {
		a.workflowChanged(t, "")
	}
	return n, err
}

// projectTeam is the project in root and its team (nil for a project of no team).
func projectTeam(root string) (*project.Repo, *teams.Team, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, nil, err
	}
	if r.Config.Remote == nil {
		return r, nil, nil
	}
	t, err := r.Team()
	if err != nil {
		return nil, nil, err
	}
	return r, t, nil
}

// ProjectWorkflow reads a project's workflow settings.
func (a *App) ProjectWorkflow(root string) (*ProjectWorkflowView, error) {
	r, t, err := projectTeam(root)
	if err != nil {
		return nil, err
	}
	out := &ProjectWorkflowView{CanEdit: true, TeamKinds: []string{}, All: profile.LockKinds, Mine: r.StepSettingsNow()}
	if rules, err := r.Profile(); err == nil && rules.FileLocks.Set() {
		out.RulesLocks = true
	}
	var own remote.Workflow
	if t == nil {
		own = r.Workflow().Project
		out.TeamAgents = agentsOf(remote.Workflow{})
	} else {
		out.Team, out.CanEdit = true, canEditWorkflow(t)
		b, err := t.Open()
		if err != nil {
			return nil, err
		}
		info, err := b.Info()
		if err != nil {
			return nil, err
		}
		ps, err := b.Projects()
		if err != nil {
			return nil, err
		}
		tw := remote.Workflow{}
		if info.Workflow != nil {
			tw = *info.Workflow
		}
		out.TeamAgents = agentsOf(tw)
		for _, p := range ps {
			if p.ID == r.Config.ProjectID && p.Workflow != nil {
				own = *p.Workflow
			}
		}
		if _, ok := cloud.Hosted(*t); ok && remote.FileLocks {
			s := remote.LocksOf(info)
			out.Locks, out.TeamLocksOn, out.TeamKinds, out.OwnLocks = true, remote.LocksDefault(s), s.Kinds, own.Locks
		}
		r.KeepWorkflow(project.TeamWorkflow{Locks: remote.LocksOf(info), Team: tw, Project: own})
	}
	out.Steps, out.Prompts, out.AgentLocks = own.Steps, own.Prompts, own.AgentLocks
	return out, nil
}

// SetProjectWorkflow sets what a project sets for itself (nil fields: as
// the team's).
func (a *App) SetProjectWorkflow(root string, w remote.Workflow) error {
	r, t, err := projectTeam(root)
	if err != nil {
		return err
	}
	if t == nil {
		w.Locks = nil // (no team: no file locking, only agents')
		cur := r.Workflow()
		cur.Project = w
		r.KeepWorkflow(cur)
		a.locksChanged(root)
		return nil
	}
	if !canEditWorkflow(t) {
		return remote.ErrNotAdmin
	}
	if _, ok := cloud.Hosted(*t); !ok || !remote.FileLocks {
		w.Locks = nil
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if err := remote.SetProjectWorkflow(b, r.Config.ProjectID, &w); err != nil {
		return err
	}
	a.workflowChanged(t, r.Config.ProjectID)
	return nil
}

// MoveFileLocksToSettings moves the file locks a project's .r3v.yaml
// still names into its settings, and takes them out of the file (a change
// to commit).
func (a *App) MoveFileLocksToSettings(root string) error {
	r, t, err := projectTeam(root)
	if err != nil {
		return err
	}
	rules, err := r.Profile()
	if err != nil {
		return err
	}
	f := rules.FileLocks
	if t != nil && f.Set() {
		if _, ok := cloud.Hosted(*t); ok && remote.FileLocks {
			cur, err := a.ProjectWorkflow(root)
			if err != nil {
				return err
			}
			own := remote.Workflow{Steps: cur.Steps, Prompts: cur.Prompts, AgentLocks: cur.AgentLocks}
			own.Locks = &remote.ProjectLocks{AutoOff: f.AutoOff, Add: f.Add, Remove: f.Remove}
			if f.Disabled {
				off := false
				own.Locks.On = &off
			}
			if err := a.SetProjectWorkflow(root, own); err != nil {
				return err
			}
		}
	}
	unlock := a.lock(root)
	defer unlock()
	path := filepath.Join(root, profile.FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(profile.RemoveFileLocks(string(data))), 0o644)
}

// workflowChanged: the team's settings (or project pid's) changed: this
// computer's copies of its projects read them again, and the pages their
// locks.
func (a *App) workflowChanged(t *teams.Team, pid string) {
	forgetTeamLocks(t.Remote.URL)
	forgetProjectRecords(t.Remote.URL)
	store, err := teams.Load()
	if err != nil {
		return
	}
	for key, root := range store.Projects {
		if !strings.HasPrefix(key, t.ID+"/") || (pid != "" && key != t.ID+"/"+pid) {
			continue
		}
		go func(root string) {
			if r, err := project.Open(root); err == nil {
				r.RefreshWorkflow()
			}
			a.locksChanged(root)
		}(root)
	}
}

// projectRecords keeps each team's project records a minute (a project's
// own file locking, read for every lock).
var projectRecords = struct {
	sync.Mutex
	byURL map[string]cachedRecords
}{byURL: map[string]cachedRecords{}}

type cachedRecords struct {
	at time.Time
	ps []remote.Project
}

func forgetProjectRecords(url string) {
	projectRecords.Lock()
	delete(projectRecords.byURL, url)
	projectRecords.Unlock()
}

// projectLocksOf is a project's own file locking (nil: as the team's).
func projectLocksOf(t *teams.Team, pid string) *remote.ProjectLocks {
	projectRecords.Lock()
	c, ok := projectRecords.byURL[t.Remote.URL]
	projectRecords.Unlock()
	if !ok || time.Since(c.at) > time.Minute {
		b, err := t.Open()
		if err != nil {
			return nil
		}
		ps, err := b.Projects()
		if err != nil {
			if ok {
				ps = c.ps // (the last known)
			} else {
				return nil
			}
		} else {
			projectRecords.Lock()
			projectRecords.byURL[t.Remote.URL] = cachedRecords{time.Now(), ps}
			projectRecords.Unlock()
		}
		c.ps = ps
	}
	for _, p := range c.ps {
		if p.ID == pid && p.Workflow != nil {
			return p.Workflow.Locks
		}
	}
	return nil
}
