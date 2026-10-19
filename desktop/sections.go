package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Sections (docs/design/sections.md): named groups of a team's projects,
// in the team's order, a project in at most one. They're read with the
// team's projects (each time its list is read, as archived projects are)
// and kept as last read, so the left panel groups at once and while the
// team can't be reached. On an R3V Cloud team its admins change them; on
// a team's own storage, anyone in it (as Workflow).

// SectionView is a section as the app shows it.
type SectionView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"` // project ids (each person orders them)
}

func sectionViews(ss []remote.Section) []SectionView {
	out := make([]SectionView, 0, len(ss))
	for _, s := range ss {
		out = append(out, SectionView{ID: s.ID, Name: s.Name, Projects: append([]string{}, s.Projects...)})
	}
	return out
}

// sectionsOf puts each project's section on it.
func sectionsOf(ps []TeamProject, ss []SectionView) {
	for i := range ps {
		if ps[i].Workspace || ps[i].ID == "" {
			continue
		}
		for _, s := range ss {
			if slices.Contains(s.Projects, ps[i].ID) {
				ps[i].Section, ps[i].SectionName = s.ID, s.Name
				break
			}
		}
	}
}

// editSections opens team teamID for a change to its sections (refused for
// those who may not), makes it, and keeps the sections as now.
func editSections(teamID string, change func(b remote.Backend) error) error {
	_, t, err := findTeam(teamID)
	if err != nil {
		return err
	}
	if !remote.ProjectSections {
		return remote.ErrSectionsNotInBuild
	}
	if !canEditWorkflow(t) {
		return remote.ErrNotAdmin
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if err := change(b); err != nil {
		return err
	}
	if ss, err := remote.Sections(b); err == nil {
		rememberSections(teamID, sectionViews(ss))
	}
	return nil
}

// NewSection adds a section to the team, last.
func (a *App) NewSection(teamID, name string) (SectionView, error) {
	var made remote.Section
	err := editSections(teamID, func(b remote.Backend) (err error) {
		made, err = remote.NewSection(b, name)
		return err
	})
	return SectionView{ID: made.ID, Name: made.Name, Projects: []string{}}, err
}

// RenameSection renames one of the team's sections.
func (a *App) RenameSection(teamID, sectionID, name string) error {
	return editSections(teamID, func(b remote.Backend) error {
		_, err := remote.RenameSection(b, sectionID, name)
		return err
	})
}

// DeleteSection removes a section; its projects become ungrouped.
func (a *App) DeleteSection(teamID, sectionID string) error {
	return editSections(teamID, func(b remote.Backend) error {
		_, err := remote.DeleteSection(b, sectionID)
		return err
	})
}

// MoveSection puts a section at place to among the team's (0 first).
func (a *App) MoveSection(teamID, sectionID string, to int) error {
	return editSections(teamID, func(b remote.Backend) error {
		_, err := remote.MoveSection(b, sectionID, to)
		return err
	})
}

// SetProjectSection puts a project in a section ("": ungrouped).
func (a *App) SetProjectSection(teamID, projectID, sectionID string) error {
	return editSections(teamID, func(b remote.Backend) error {
		_, err := remote.SetProjectSection(b, projectID, sectionID)
		return err
	})
}

// team-sections.json keeps each team's sections as last read.
var sectionsMu sync.Mutex

func sectionsFile() string { return filepath.Join(teams.Dir(), "team-sections.json") }

func readSectionsFile() map[string][]SectionView {
	all := map[string][]SectionView{}
	if data, err := os.ReadFile(sectionsFile()); err == nil {
		json.Unmarshal(data, &all)
	}
	return all
}

func lastSections(teamID string) []SectionView {
	sectionsMu.Lock()
	defer sectionsMu.Unlock()
	if !remote.ProjectSections {
		return []SectionView{}
	}
	if ss := readSectionsFile()[teamID]; ss != nil {
		return ss
	}
	return []SectionView{}
}

func rememberSections(teamID string, ss []SectionView) {
	sectionsMu.Lock()
	defer sectionsMu.Unlock()
	all := readSectionsFile()
	same := slices.EqualFunc(all[teamID], ss, func(a, b SectionView) bool {
		return a.ID == b.ID && a.Name == b.Name && slices.Equal(a.Projects, b.Projects)
	})
	if same {
		return // unchanged: not written every minute
	}
	if len(ss) == 0 {
		delete(all, teamID)
	} else {
		all[teamID] = ss
	}
	data, err := json.Marshal(all)
	if err != nil {
		return
	}
	// Whole, then renamed (stopped half-way, the old one stays).
	tmp := sectionsFile() + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		if os.Rename(tmp, sectionsFile()) != nil {
			os.Remove(tmp)
		}
	}
}
