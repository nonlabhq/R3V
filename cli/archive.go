package cli

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Archiving a project for the whole team (docs/design/archive.md), in
// every build since 0.2.2.
func init() {
	extraCommands["project"] = cmdProject
	jsonCommands["project"] = true
	extraUsage += `
projects of the team:
  project list                           the team's projects, archived ones marked
  project archive [--force]              archive this project for the whole team: hidden for everyone,
                                         nothing deleted (its versions stay); refused with uncommitted
                                         changes or versions not shared unless --force (ask the user);
                                         on R3V Cloud teams, admins only
  project unarchive                      bring this project back for the team
`
}

// archiveJSON is a project archived or brought back.
type archiveJSON struct {
	Project  string `json:"project"`
	ID       string `json:"id"`
	Archived bool   `json:"archived"`
	// Changed: false when it already was (or wasn't).
	Changed bool `json:"changed"`
}

// projectListJSON is one of the team's projects.
type projectListJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	// ArchivedBy and ArchivedAt: who archived it (a member id) and when.
	ArchivedBy string `json:"archived_by,omitempty"`
	ArchivedAt string `json:"archived_at,omitempty"`
	Here       bool   `json:"here"` // the project in this folder
}

const projectUsage = "usage: r3v project list | archive [--force] | unarchive"

func cmdProject(args []string) error {
	if len(args) == 0 {
		return usageError(projectUsage)
	}
	fs := flag.NewFlagSet("project "+args[0], flag.ContinueOnError)
	force := fs.Bool("force", false, "archive even with uncommitted changes or versions not shared")
	pos, err := parseArgs(fs, args[1:])
	if err != nil {
		return err
	}
	if len(pos) > 0 || *force && args[0] != "archive" {
		return usageError(projectUsage)
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
	switch args[0] {
	case "list":
		return projectList(r, c)
	case "archive", "unarchive":
		on := args[0] == "archive"
		t, err := r.Team()
		if err != nil {
			return err
		}
		if err := checkAdmin(t); err != nil {
			return err
		}
		if on && !*force {
			if err := checkAllShared(r); err != nil {
				return err
			}
		}
		changed, err := remote.SetProjectArchived(c, r.Config.ProjectID, on)
		if err != nil {
			return err
		}
		out := archiveJSON{Project: r.Config.Name, ID: r.Config.ProjectID, Archived: on, Changed: changed}
		result("project", out, func() {
			switch {
			case on && changed:
				fmt.Printf("archived %q for the whole team: hidden for everyone, nothing deleted (`r3v project unarchive` brings it back)\n", r.Config.Name)
			case on:
				fmt.Printf("%q was archived already\n", r.Config.Name)
			case changed:
				fmt.Printf("%q is back for the whole team\n", r.Config.Name)
			default:
				fmt.Printf("%q isn't archived\n", r.Config.Name)
			}
		})
		return nil
	}
	return usageError(projectUsage)
}

// checkAllShared fails when the copy here has work the team hasn't got.
func checkAllShared(r *project.Repo) error {
	changes, err := r.Status()
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return fmt.Errorf("%w (%d file(s)); commit them first, or --force to archive it all the same", project.ErrDirty, len(changes))
	}
	shared, err := r.Shared()
	if err != nil {
		return err
	}
	if !shared {
		return project.ErrUnshared
	}
	return nil
}

// checkAdmin: on an R3V Cloud team only its owners and admins archive (the
// service refuses anyone else's write all the same).
func checkAdmin(t *teams.Team) error {
	svc, ok := cloud.Hosted(*t)
	if !ok {
		return nil
	}
	me, err := cloud.GetMe(svc)
	if err != nil {
		return err
	}
	team := cloud.TeamID(t.Remote.URL)
	for _, mt := range me.Teams {
		if mt.ID == team && (mt.Role == "owner" || mt.Role == "admin") {
			return nil
		}
	}
	return remote.ErrNotAdmin
}

func projectList(r *project.Repo, c remote.Backend) error {
	ps, err := c.Projects()
	if err != nil {
		return err
	}
	archived, err := remote.ArchivedProjects(c)
	if err != nil {
		return err
	}
	out := []projectListJSON{}
	for _, p := range ps {
		e := projectListJSON{ID: p.ID, Name: p.Name, Here: p.ID == r.Config.ProjectID}
		if a, ok := archived[p.ID]; ok {
			e.Archived, e.ArchivedBy = true, a.By
			if !a.At.IsZero() {
				e.ArchivedAt = a.At.UTC().Format("2006-01-02T15:04:05Z")
			}
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	result("project", map[string]any{"projects": out}, func() {
		for _, p := range out {
			mark := "  "
			if p.Here {
				mark = "* "
			}
			note := ""
			if p.Archived {
				note = "  (archived)"
			}
			fmt.Printf("%s%-24s %s%s\n", mark, p.Name, p.ID, note)
		}
	})
	return nil
}
