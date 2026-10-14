package desktop

import (
	"net/url"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/teams"
)

// r3v:// links (R3V Cloud's web app, Nightly while new): the app opens a
// team, a project or a version of one when Windows hands it a link, at
// start or while it runs:
//
//	r3v://team/<team id>
//	r3v://project/<project id>
//	r3v://project/<project id>/version/<version id>
//
// A team id is R3V Cloud's (the hosted team's address ends with it), or
// this computer's own for a team on storage.

// webLinks: this build opens r3v:// links (links_nightly.go).
var webLinks = false

// Link is an r3v:// link, read.
type Link struct {
	Team, Project, Version string
}

// ParseLink reads an r3v:// link (false: not one this R3V opens).
func ParseLink(s string) (Link, bool) {
	u, err := url.Parse(s)
	if err != nil || !strings.EqualFold(u.Scheme, "r3v") {
		return Link{}, false
	}
	parts := append([]string{u.Host}, strings.Split(strings.Trim(u.Path, "/"), "/")...)
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	for i := 0; i < len(parts); i += 2 { // (team, project, version: in any case)
		parts[i] = strings.ToLower(parts[i])
	}
	id := func(s string) bool {
		return s != "" && len(s) <= 64 && strings.Trim(strings.ToLower(s), "0123456789abcdef") == ""
	}
	switch {
	case len(parts) == 2 && parts[0] == "team" && id(parts[1]):
		return Link{Team: strings.ToLower(parts[1])}, true
	case len(parts) == 2 && parts[0] == "project" && id(parts[1]):
		return Link{Project: strings.ToLower(parts[1])}, true
	case len(parts) == 4 && parts[0] == "project" && id(parts[1]) && parts[2] == "version" && id(parts[3]):
		return Link{Project: strings.ToLower(parts[1]), Version: strings.ToLower(parts[3])}, true
	}
	return Link{}, false
}

// LinkTarget is where a link leads on this computer, for the page: the
// team (this computer's id) and the project and version to show; Missing
// says what this computer doesn't have ("team", "project").
type LinkTarget struct {
	Team    string `json:"team"`
	Project string `json:"project"`
	Version string `json:"version"`
	Missing string `json:"missing"`
}

// resolveLink finds a link's team and project among this computer's.
func resolveLink(l Link) LinkTarget {
	out := LinkTarget{Project: l.Project, Version: l.Version}
	s, err := teams.Load()
	if err != nil {
		out.Missing = "team"
		return out
	}
	if l.Team != "" {
		for _, t := range s.Teams {
			if _, hosted := cloud.Hosted(t); (hosted && strings.EqualFold(cloud.TeamID(t.Remote.URL), l.Team)) || t.ID == l.Team {
				out.Team = t.ID
				return out
			}
		}
		out.Missing = "team"
		return out
	}
	// A project: the team that has it here (downloaded), else a team whose
	// list has it.
	for key := range s.Projects {
		if team, pid, ok := strings.Cut(key, "/"); ok && pid == l.Project {
			out.Team = team
			return out
		}
	}
	for _, t := range s.Teams {
		for _, p := range lastTeamProjects(t.ID) {
			if p.ID == l.Project {
				out.Team = t.ID
				return out
			}
		}
	}
	out.Missing = "project"
	return out
}

// pendingLinks: links handed over but not yet taken by the page; a few at
// most (a page that never takes them doesn't make them pile up).
const maxPendingLinks = 8

var pendingLinks struct {
	sync.Mutex
	list []Link
}

// handLinks takes the r3v:// links among a launch's arguments (the first
// launch's, or another's while the app runs); it reports whether there
// were any.
func (a *App) handLinks(args []string) bool {
	if !webLinks {
		return false
	}
	found := false
	for _, arg := range args {
		if l, ok := ParseLink(arg); ok {
			pendingLinks.Lock()
			if len(pendingLinks.list) == maxPendingLinks {
				pendingLinks.list = pendingLinks.list[1:] // (the oldest goes)
			}
			pendingLinks.list = append(pendingLinks.list, l)
			pendingLinks.Unlock()
			found = true
		}
	}
	if found && a.emit != nil {
		a.emit("open-link", nil)
	}
	return found
}

// TakeLink is the next link handed over, where it leads here (nil: none).
func (a *App) TakeLink() *LinkTarget {
	pendingLinks.Lock()
	defer pendingLinks.Unlock()
	if len(pendingLinks.list) == 0 {
		return nil
	}
	l := pendingLinks.list[0]
	pendingLinks.list = pendingLinks.list[1:]
	t := resolveLink(l)
	return &t
}
