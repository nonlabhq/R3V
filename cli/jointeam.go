package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team this computer has joined, named instead of its connection code
// (init --team, remote --team, clone --team): an agent starts a project
// on the person's team without ever handling the code, which holds the
// storage's keys. The keys stay where joining the team put them.

// teamByName: the --team options are in this build (Nightly while new).
var teamByName = false

const teamByNameUsage = `
a team this computer has joined, by name (no connection code; Nightly):
  init --team NAME [--preset code|none]  make the folder a project of that team (r3v teams lists them)
  remote --team NAME                     connect this project to that team
  clone --team NAME <project> [folder]   download one of its projects
`

// joinedTeam is the team this computer joined named name (or with that id).
func joinedTeam(name string) (*teams.Team, error) {
	if !teamByName {
		return nil, errors.New("--team needs R3V's Nightly channel; use a connection code")
	}
	store, err := teams.Load()
	if err != nil {
		return nil, err
	}
	for i, t := range store.Teams {
		if t.ID == name || strings.EqualFold(t.Name, name) {
			return &store.Teams[i], nil
		}
	}
	return nil, usageError("this computer hasn't joined a team %q (r3v teams lists the ones it has; joining one is the person's, in R3V)", name)
}

// joinTeam makes project r a project of team t (as R3V's Add project
// does); the caller holds r's lock.
func joinTeam(r *project.Repo, t *teams.Team) error {
	if r.Config.Remote != nil {
		if teams.NormalizeURL(r.Config.Remote.URL) == t.Remote.URL {
			return nil
		}
		return fmt.Errorf("this project already belongs to another team (%s)", r.Config.Remote.URL)
	}
	if err := r.JoinTeam(t); err != nil {
		return err
	}
	ensureRules(r)
	return nil
}
