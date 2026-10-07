package desktop

import (
	"sort"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Milestones (Nightly, see remote.Milestone): asked with the team's state
// (TeamState), kept for the times the page is shown without asking.

// Milestone is a version given a name, for the page.
type Milestone struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Name    string `json:"name"`
	Note    string `json:"note"`
	By      string `json:"by"` // who named it ("" unknown)
	Time    string `json:"time"`
}

var milestoneCache = struct {
	sync.Mutex
	byRoot map[string]map[string]remote.Milestone
}{byRoot: map[string]map[string]remote.Milestone{}}

// milestonesOf returns project r's milestones, newest first: asked from
// the team when ask, else as last asked.
func (a *App) milestonesOf(r *project.Repo, ask bool, names map[string]string) []Milestone {
	milestoneCache.Lock()
	all := milestoneCache.byRoot[r.Root]
	milestoneCache.Unlock()
	if ask && r.Config.Remote != nil {
		if got, err := r.Milestones(); err == nil {
			all = got
			milestoneCache.Lock()
			milestoneCache.byRoot[r.Root] = got
			milestoneCache.Unlock()
		}
	}
	out := []Milestone{}
	for id, m := range all {
		out = append(out, Milestone{ID: id, Version: m.Version, Name: m.Name, Note: m.Note, By: names[m.By],
			Time: m.Time.Format(time.RFC3339)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out
}

func (a *App) afterMilestone(r *project.Repo) {
	a.milestonesOf(r, true, nil)
}

// AddMilestone names version for the whole team.
func (a *App) AddMilestone(root, version, name, note string) error {
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	if _, err := r.AddMilestone(version, name, note); err != nil {
		return err
	}
	a.afterMilestone(r)
	return nil
}

// EditMilestone renames milestone id and changes its note.
func (a *App) EditMilestone(root, id, name, note string) error {
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := r.EditMilestone(id, name, note); err != nil {
		return err
	}
	a.afterMilestone(r)
	return nil
}

// RemoveMilestone takes milestone id away (its version stays).
func (a *App) RemoveMilestone(root, id string) error {
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := r.RemoveMilestone(id); err != nil {
		return err
	}
	a.afterMilestone(r)
	return nil
}
