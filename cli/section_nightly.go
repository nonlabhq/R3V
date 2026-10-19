//go:build nightly

package cli

import (
	"flag"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Sections grouping the team's projects (docs/design/sections.md) are on
// the Nightly channel while they're new.
func init() {
	extraCommands["section"] = cmdSection
	jsonCommands["section"] = true
	extraUsage += `
sections of the team's projects (Nightly):
  section [list]                         the team's sections in order, with their projects
  section new NAME                       add a section (last)
  section rename NAME NEW-NAME           rename a section
  section delete NAME                    delete a section: its projects become ungrouped
  section add NAME                       put this project in a section (out of any other)
  section remove                         take this project out of its section (ungrouped)
                                         on R3V Cloud teams sections change for admins only
`
}

// sectionProjectJSON is a project in a section's list.
type sectionProjectJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Here bool   `json:"here"` // the project in this folder
}

// sectionJSON is one of the team's sections.
type sectionJSON struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Projects []sectionProjectJSON `json:"projects"`
}

// sectionChangeJSON is what a change did.
type sectionChangeJSON struct {
	// Section: the one changed, as it is now (deleted: as it was; none
	// when the project was taken out of its section).
	Section *sectionJSON `json:"section,omitempty"`
	// Project and ID: the project in this folder (add, remove).
	Project string `json:"project,omitempty"`
	ID      string `json:"id,omitempty"`
	// Changed: false when it already was so.
	Changed bool `json:"changed"`
}

const sectionUsage = "usage: r3v section list | new NAME | rename NAME NEW-NAME | delete NAME | add NAME | remove"

func cmdSection(args []string) error {
	sub := "list"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("section "+sub, flag.ContinueOnError)
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	want := map[string]int{"list": 0, "new": 1, "rename": 2, "delete": 1, "add": 1, "remove": 0}
	if n, ok := want[sub]; !ok || len(pos) != n {
		return usageError(sectionUsage)
	}
	r, err := openRead()
	if err != nil {
		return err
	}
	if r.Config.Remote == nil {
		return project.ErrNoRemote
	}
	c, err := r.Client()
	if err != nil {
		return err
	}
	if sub == "list" {
		return sectionList(r, c)
	}
	t, err := r.Team()
	if err != nil {
		return err
	}
	if err := checkAdmin(t); err != nil {
		return err
	}
	// find a section by its name (or id) as the team has it now.
	find := func(key string) (remote.Section, error) {
		ss, err := remote.Sections(c)
		if err != nil {
			return remote.Section{}, err
		}
		i := remote.FindSection(ss, key)
		if i < 0 {
			return remote.Section{}, fmt.Errorf("%w: %q", remote.ErrNoSection, key)
		}
		return ss[i], nil
	}
	out := sectionChangeJSON{}
	var text string
	switch sub {
	case "new":
		s, err := remote.NewSection(c, pos[0])
		if err != nil {
			return err
		}
		out.Section, out.Changed = &sectionJSON{ID: s.ID, Name: s.Name, Projects: []sectionProjectJSON{}}, true
		text = fmt.Sprintf("added the section %q", s.Name)
	case "rename":
		s, err := find(pos[0])
		if err != nil {
			return err
		}
		if out.Changed, err = remote.RenameSection(c, s.ID, pos[1]); err != nil {
			return err
		}
		text = fmt.Sprintf("renamed the section %q", s.Name)
		if s, err = find(s.ID); err != nil {
			return err
		}
		out.Section = sectionOut(r, c, s)
	case "delete":
		s, err := find(pos[0])
		if err != nil {
			return err
		}
		out.Section = sectionOut(r, c, s)
		if out.Changed, err = remote.DeleteSection(c, s.ID); err != nil {
			return err
		}
		text = fmt.Sprintf("deleted the section %q: its projects are ungrouped", s.Name)
	case "add", "remove":
		out.Project, out.ID = r.Config.Name, r.Config.ProjectID
		id := ""
		if sub == "add" {
			s, err := find(pos[0])
			if err != nil {
				return err
			}
			id = s.ID
		}
		if out.Changed, err = remote.SetProjectSection(c, r.Config.ProjectID, id); err != nil {
			return err
		}
		if id != "" {
			s, err := find(id)
			if err != nil {
				return err
			}
			out.Section = sectionOut(r, c, s)
			text = fmt.Sprintf("%q is in the section %q", r.Config.Name, s.Name)
		} else {
			text = fmt.Sprintf("%q is in no section", r.Config.Name)
		}
	}
	result("section", out, func() {
		if !out.Changed {
			text += " (it already was)"
		}
		fmt.Println(text)
	})
	return nil
}

// sectionOut gives section s with its projects' names.
func sectionOut(r *project.Repo, c remote.Backend, s remote.Section) *sectionJSON {
	ps, _ := c.Projects()
	return sectionWith(r, ps, s)
}

func sectionWith(r *project.Repo, ps []remote.Project, s remote.Section) *sectionJSON {
	out := &sectionJSON{ID: s.ID, Name: s.Name, Projects: []sectionProjectJSON{}}
	for _, p := range ps {
		if slices.Contains(s.Projects, p.ID) {
			out.Projects = append(out.Projects, sectionProjectJSON{ID: p.ID, Name: p.Name, Here: p.ID == r.Config.ProjectID})
		}
	}
	sortSectionProjects(out.Projects)
	return out
}

func sortSectionProjects(ps []sectionProjectJSON) {
	sort.SliceStable(ps, func(i, j int) bool { return strings.ToLower(ps[i].Name) < strings.ToLower(ps[j].Name) })
}

func sectionList(r *project.Repo, c remote.Backend) error {
	ps, err := c.Projects()
	if err != nil {
		return err
	}
	ss, err := remote.Sections(c)
	if err != nil {
		return err
	}
	archived, err := remote.ArchivedProjects(c)
	if err != nil {
		return err
	}
	// (archived projects are listed apart, as the app has them)
	live := slices.DeleteFunc(slices.Clone(ps), func(p remote.Project) bool { _, a := archived[p.ID]; return a })
	sections := []*sectionJSON{}
	for _, s := range ss {
		sections = append(sections, sectionWith(r, live, s))
	}
	ungrouped := []sectionProjectJSON{}
	for _, p := range live {
		if remote.SectionOf(ss, p.ID) == "" {
			ungrouped = append(ungrouped, sectionProjectJSON{ID: p.ID, Name: p.Name, Here: p.ID == r.Config.ProjectID})
		}
	}
	sortSectionProjects(ungrouped)
	here := remote.SectionOf(ss, r.Config.ProjectID)
	result("section", map[string]any{"sections": sections, "ungrouped": ungrouped, "here": here}, func() {
		line := func(p sectionProjectJSON) {
			mark := "  "
			if p.Here {
				mark = "* "
			}
			fmt.Printf("  %s%s\n", mark, p.Name)
		}
		for _, p := range ungrouped {
			line(p)
		}
		for _, s := range sections {
			fmt.Printf("%s\n", s.Name)
			for _, p := range s.Projects {
				line(p)
			}
		}
	})
	return nil
}
