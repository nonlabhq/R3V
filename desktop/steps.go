package desktop

import (
	"errors"

	"github.com/nonlabhq/r3v/internal/project"
)

// Steps (docs/design/steps.md): the project after each of an agent's
// moves, on this computer; shown above Your changes as a timeline.

// StepsInfo is a copy's steps for the app.
type StepsInfo struct {
	// Available: this build makes steps (Nightly); On: and this copy does.
	Available bool                 `json:"available"`
	On        bool                 `json:"on"`
	Settings  project.StepSettings `json:"settings"`
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
