package project

import (
	"path/filepath"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/store"
)

// The team's workflow settings as they apply to this project
// (docs/design/team-settings.md): the team's defaults, the project's own
// over them. Kept on this computer beside the agent lock list, as last
// read: the hooks run on every tool call and can't ask the team.

const workflowFile = "workflow.json"

// TeamWorkflow is what the team and the project say, as last read.
type TeamWorkflow struct {
	Locks   remote.LockSettings `json:"locks"`
	Team    remote.Workflow     `json:"team"`
	Project remote.Workflow     `json:"project"`
}

// LockingFor is a project's file locking: its own over the team's.
func LockingFor(s remote.LockSettings, p *remote.ProjectLocks) profile.Locking {
	team := profile.TeamLocks{On: remote.ProjectLocksOn(s, p), Kinds: s.Kinds}
	var f profile.FileLocks
	if p != nil {
		if p.Kinds != nil {
			team.Kinds = p.Kinds
		}
		f = profile.FileLocks{AutoOff: p.AutoOff, Add: p.Add, Remove: p.Remove}
	}
	l := profile.ResolveLocking(team, f)
	if !l.On {
		own := p != nil && p.On != nil
		l.TeamOff, l.ProjectOff = !own, own
	}
	return l
}

// Workflow is the team's workflow for this project as last read (all as by
// default before it was ever read). A project of no team keeps its own
// here (Project), set on this computer.
func (r *Repo) Workflow() TeamWorkflow {
	var w TeamWorkflow
	if readJSON(filepath.Join(r.AgentLockList().Dir(), workflowFile), &w) != nil {
		// (read by an R3V before workflow settings: the team's lock settings)
		var s remote.LockSettings
		if readJSON(filepath.Join(r.AgentLockList().Dir(), teamLocksFile), &s) == nil {
			w.Locks = s
		}
	}
	return w
}

// RefreshWorkflow reads the team's workflow settings and the project's own
// into this computer's copy (best effort: out of reach, the last known
// stay).
func (r *Repo) RefreshWorkflow() {
	c, err := r.Client()
	if err != nil {
		return
	}
	info, err := c.Info()
	if err != nil {
		return
	}
	ps, err := c.Projects()
	if err != nil {
		return
	}
	w := TeamWorkflow{Locks: remote.LocksOf(info)}
	if info.Workflow != nil {
		w.Team = *info.Workflow
	}
	for _, p := range ps {
		if p.ID == r.Config.ProjectID && p.Workflow != nil {
			w.Project = *p.Workflow
		}
	}
	r.KeepWorkflow(w)
}

// KeepWorkflow keeps w as this project's workflow on this computer (the app,
// having just read or written it).
func (r *Repo) KeepWorkflow(w TeamWorkflow) {
	dir := r.AgentLockList().Dir()
	store.Remove(filepath.Join(dir, teamLocksFile))
	writeJSON(filepath.Join(dir, workflowFile), w)
	r.agentLocking = nil
}

// StepsByTeam and PromptsByTeam: what the team (or the project) says about
// agents' steps and prompts; this copy's own choices (StepSettings) can
// only keep more out.
func (w TeamWorkflow) StepsByTeam() bool { return remote.Value(w.Team.Steps, w.Project.Steps, true) }
func (w TeamWorkflow) PromptsByTeam() bool {
	return remote.Value(w.Team.Prompts, w.Project.Prompts, true)
}

// AgentLocksOn: the project's workspaces take turns on files that can't be
// merged.
func (w TeamWorkflow) AgentLocksOn() bool {
	return remote.Value(w.Team.AgentLocks, w.Project.AgentLocks, true)
}
