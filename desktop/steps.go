package desktop

import (
	"errors"

	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/transcript"
)

// Steps (docs/design/steps.md): the project after each of an agent's
// moves, on this computer; shown above Your changes as a timeline.

// StepsInfo is a copy's steps for the app.
type StepsInfo struct {
	// Available: this build makes steps (Nightly); On: and this copy does.
	Available bool                 `json:"available"`
	On        bool                 `json:"on"`
	Settings  project.StepSettings `json:"settings"`
	// Hooked: Claude Code in this copy makes them (its hooks are set up).
	Hooked bool `json:"hooked"`
	// Tool is the agent tool the copy is set up for ("": none): another
	// than Claude Code makes its steps itself (r3v step start and end),
	// with no hooks to set up.
	Tool string `json:"tool"`
	// Session is the one asked for, or the latest (nil: none yet).
	Session  *project.Session `json:"session"`
	Sessions []SessionSummary `json:"sessions"`
}

// SessionSummary is a session in the app's list of them.
type SessionSummary struct {
	ID     string `json:"id"`
	Last   string `json:"last"`
	Steps  int    `json:"steps"`
	Prompt string `json:"prompt"` // its first prompt (what it was about)
}

// Steps lists the steps of the copy in root: a session's ("" the latest)
// and the others.
func (a *App) Steps(root, session string) (*StepsInfo, error) {
	info := &StepsInfo{Available: project.Steps, Sessions: []SessionSummary{}}
	if !project.Steps {
		return info, nil
	}
	// Read only (lines appended whole): no lock, so the agent's hooks never
	// wait for the app's timeline.
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	info.On, info.Settings = r.StepsOn(), r.StepSettingsNow()
	info.Hooked = agentsetup.StepHooks(r.Root)
	info.Tool = agentsetup.CopyTool(r.Root)
	all, err := r.Sessions()
	if err != nil {
		return nil, err
	}
	for i, s := range all {
		sum := SessionSummary{ID: s.ID, Last: s.Last, Steps: s.Steps}
		for _, t := range s.Turns {
			if t.Prompt != "" || t.Note != "" {
				sum.Prompt = t.Prompt + t.Note
				break
			}
		}
		info.Sessions = append(info.Sessions, sum)
		if s.ID == session || (session == "" && i == 0) {
			info.Session = &all[i]
		}
	}
	return info, nil
}

// StepBack puts the files as at step n of a session; the files as they are
// become a step first. Refused while Live has a set of the project open
// (unless force).
func (a *App) StepBack(root, session string, n int, force bool) (*Result, error) {
	defer a.tidyLater(root)
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if set := liveGuard(r, force); set != "" {
		return blocked(set), nil
	}
	_, notes, err := r.StepBack(session, n)
	if err != nil {
		return nil, err
	}
	return &Result{Action: "moved", Log: []string{}, Relinked: nonNil(notes), Conflicts: []Conflict{}}, nil
}

// StepDiff is what changed from step from to step to of a session (to 0:
// the files now).
func (a *App) StepDiff(root, session string, from, to int) ([]project.StepChange, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return r.StepDiff(session, from, to)
}

// SetStepSettings changes this copy's choices about steps (this computer
// only).
func (a *App) SetStepSettings(root string, s project.StepSettings) error {
	if !project.Steps {
		return errors.New("steps aren't in this build")
	}
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	return r.SetStepSettings(s)
}

// SetUpSteps sets Claude Code up in the copy in root to make steps (its
// hooks, in .claude/settings.local.json, which R3V never tracks): a copy no
// workspace was made for, the main one say, where an agent works too.
func (a *App) SetUpSteps(root string) error {
	if !project.Steps {
		return errors.New("steps aren't in this build")
	}
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	_, err = agentsetup.ClaudeCode(r.Root, agentsetup.HookCommand(cliPath()), false, r.StepsOn())
	return err
}

// VersionSteps are the steps a version was made of (any copy of the
// project here), to look at and compare: going back is the work's own.
func (a *App) VersionSteps(root, id string) ([]project.Session, error) {
	if !project.Steps {
		return []project.Session{}, nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	return r.VersionSteps(id)
}

// AllSteps are every session's steps of the copy in root, the latest first
// (a workspace's, looked at from the Workspaces tab).
func (a *App) AllSteps(root string) ([]project.Session, error) {
	if !project.Steps {
		return []project.Session{}, nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	return r.Sessions()
}

// StepsTalk is the conversation a session's steps were made in (its
// prompts, the agent's replies, its tool calls), read from where the agent
// tool keeps it; nothing when it's gone or not one R3V reads.
func (a *App) StepsTalk(root, session string) ([]transcript.Entry, error) {
	if !project.Steps {
		return []transcript.Entry{}, nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	s, err := r.SessionSteps(session)
	if err != nil {
		return nil, err
	}
	for _, t := range s.Turns {
		if t.Transcript != "" {
			if got, err := transcript.Read(t.Transcript); err == nil {
				return got, nil
			}
		}
	}
	return []transcript.Entry{}, nil
}
