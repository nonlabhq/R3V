package cli

import (
	"fmt"
	"strings"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// A team this computer has joined, named instead of its connection code
// (init --team, remote --team, clone --team): an agent starts a project
// on the person's team without ever handling the code, which holds the
// storage's keys. The keys stay where joining the team put them. In
// every build since 0.2 (R3V Cloud teams are Stable).

// joinedTeam is the team this computer joined named name (or with that id).
func joinedTeam(name string) (*teams.Team, error) {
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
// does); the caller holds r's lock. A project of another team: only one
// that team no longer has, moved to t (project.FollowTo).
func joinTeam(r *project.Repo, t *teams.Team) error {
	if r.Config.Remote != nil {
		if teams.NormalizeURL(r.Config.Remote.URL) == t.Remote.URL {
			return nil
		}
		if err := r.FollowTo(t); err != nil {
			return fmt.Errorf("this project belongs to another team: %w", err)
		}
		return nil
	}
	if err := r.JoinTeam(t); err != nil {
		return err
	}
	ensureRules(r)
	return nil
}
