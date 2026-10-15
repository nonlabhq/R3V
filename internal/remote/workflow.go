package remote

import (
	"errors"
	"slices"
)

// Workflow settings (docs/design/team-settings.md): how a team works
// together. The team's are defaults, in its info (team.json); a project
// overrides one in its record (projects.json), and a setting it doesn't
// name follows the team. Both records keep what an older R3V doesn't know
// (jsonx.Extra).
//
// An older R3V reads neither: it would lock where a project turned file
// locking off, or keep making steps and prompts where the team said not to.
// So writing any of them turns the team feature "workflow" on, and R3Vs
// without it update before sharing. Every build knows the feature: a build
// without file locks or steps (Stable) has nothing to misread.

// FeatureWorkflow is the team feature turned on with workflow settings.
const FeatureWorkflow = "workflow"

func init() { RegisterFeature(FeatureWorkflow) }

// Workflow is a team's defaults or a project's overrides: nil is "not
// set" (R3V's default for a team, the team's for a project).
type Workflow struct {
	// Locks: a project's own file locking (R3V Cloud teams; the team's is
	// LockSettings). Only in a project's record.
	Locks *ProjectLocks `json:"locks,omitempty"`
	// Steps: agents' steps are recorded (docs/design/steps.md).
	Steps *bool `json:"steps,omitempty"`
	// Prompts: what was asked is kept with the steps.
	Prompts *bool `json:"prompts,omitempty"`
	// AgentLocks: the workspaces of a project take turns on files that
	// can't be merged (docs/design/agent-workspaces.md, "Locks").
	AgentLocks *bool `json:"agentLocks,omitempty"`
}

// Empty says whether w sets nothing.
func (w *Workflow) Empty() bool {
	return w == nil || (w.Locks == nil && w.Steps == nil && w.Prompts == nil && w.AgentLocks == nil)
}

// ProjectLocks is a project's file locking, over the team's.
type ProjectLocks struct {
	// On: locking on or off in this project (nil: as the team's default).
	On *bool `json:"on,omitempty"`
	// Kinds: the kinds of file that lock by themselves here (nil: the
	// team's); AutoOff: none do, manual locks only.
	Kinds   []string `json:"kinds,omitempty"`
	AutoOff bool     `json:"autoOff,omitempty"`
	// Add and Remove: paths that lock by themselves too, and those that
	// never do (the patterns .r3v.yaml took: "Content/Maps/", "*.uasset").
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// Value is a setting as it applies: the project's, else the team's, else
// def.
func Value(team, project *bool, def bool) bool {
	switch {
	case project != nil:
		return *project
	case team != nil:
		return *team
	}
	return def
}

// LocksDefault is whether a project with no locking of its own locks: the
// team's default (with locking on, on unless it said off).
func LocksDefault(s LockSettings) bool { return s.On && (s.Default == nil || *s.Default) }

// ProjectLocksOn is whether a project locks: its own say, else the team's
// default.
func ProjectLocksOn(s LockSettings, p *ProjectLocks) bool {
	if p != nil && p.On != nil {
		return *p.On
	}
	return LocksDefault(s)
}

// ErrNotAdmin: on an R3V Cloud team, workflow settings are its admins'.
var ErrNotAdmin = errors.New("only the team's admins change this")

// SetTeamWorkflow writes the team's workflow defaults (its locks stay
// LockSettings), turning the feature on.
func SetTeamWorkflow(b Backend, w Workflow) error {
	info, err := b.Info()
	if err != nil {
		return err
	}
	w.Locks = nil
	if w.Empty() {
		info.Workflow = nil
	} else {
		info.Workflow = &w
	}
	withFeature(&info, FeatureWorkflow)
	return b.SetInfo(info)
}

// SetProjectWorkflow writes a project's overrides (nil or empty: it follows
// the team in everything), turning the feature on. A project turning file
// locking on also turns it on for the team's service, which keeps who
// holds what (its default for other projects unchanged).
func SetProjectWorkflow(b Backend, pid string, w *Workflow) error {
	if w != nil && w.Locks != nil && !FileLocks {
		return ErrLocksNotInBuild
	}
	ps, err := b.Projects()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(ps, func(p Project) bool { return p.ID == pid })
	if i < 0 {
		return ErrNotFound
	}
	info, err := b.Info()
	if err != nil {
		return err
	}
	write := !withFeature(&info, FeatureWorkflow)
	if s := LocksOf(info); w != nil && w.Locks != nil && w.Locks.On != nil && *w.Locks.On && !s.On {
		// The service starts keeping locks; the other projects stay off.
		off := false
		s.On, s.Default = true, &off
		info.Locks = &s
		withFeature(&info, FeatureLocks)
		write = true
	}
	if write {
		if err := b.SetInfo(info); err != nil {
			return err
		}
	}
	p := ps[i]
	if w.Empty() {
		p.Workflow = nil
	} else {
		p.Workflow = w
	}
	return b.PutProject(p)
}

// ClearProjectWorkflow makes every project follow the team again in what
// pick names ("locks", "agents"; "" for everything), and says how many
// changed.
func ClearProjectWorkflow(b Backend, pick string) (int, error) {
	ps, err := b.Projects()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range ps {
		if p.Workflow.Empty() {
			continue
		}
		w := *p.Workflow
		switch pick {
		case "locks":
			w.Locks = nil
		case "agents":
			w.Steps, w.Prompts, w.AgentLocks = nil, nil, nil
		default:
			w = Workflow{}
		}
		if w == *p.Workflow {
			continue
		}
		if w.Empty() {
			p.Workflow = nil
		} else {
			p.Workflow = &w
		}
		if err := b.PutProject(p); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// withFeature lists feature f in info, saying whether it was there.
func withFeature(info *TeamInfo, f string) bool {
	if slices.Contains(info.Features, f) {
		return true
	}
	info.Features = append(info.Features, f)
	return false
}
