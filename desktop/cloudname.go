package desktop

import (
	"errors"
	"net/url"
	"strings"
	"sync"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// CloudRename is a name changed on R3V Cloud: the team's summary now, and
// the teams whose member record couldn't be written (not reached).
type CloudRename struct {
	Team       TeamSummary `json:"team"`
	NotUpdated []string    `json:"notUpdated"`
}

// SetCloudName changes the person's name on R3V Cloud (the account's, which
// their teams there get when they next ask who they are), then their
// member record in each of those teams, so teammates see it on their
// versions now. Nothing changes when the service refuses or can't be
// reached; a team not reached keeps the old name in its record
// (NotUpdated) until the name is saved again.
func (a *App) SetCloudName(teamID, name string) (CloudRename, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return CloudRename{}, errors.New("a name needs 1 to 100 characters")
	}
	store, err := teams.Load()
	if err != nil {
		return CloudRename{}, err
	}
	t := store.Find(teamID)
	if t == nil {
		return CloudRename{}, errors.New("unknown team")
	}
	svc, ok := cloud.Hosted(*t)
	if !ok {
		return CloudRename{}, errors.New("not an R3V Cloud team")
	}
	if err := cloud.SetName(svc, name); err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return CloudRename{}, errors.New("R3V Cloud can't be reached: check the connection and try again")
		}
		return CloudRename{}, err
	}
	var mine []teams.Team
	for _, h := range store.Teams {
		if s, ok := cloud.Hosted(h); ok && strings.EqualFold(s, svc) && !h.NoAccess && h.MemberID != "" {
			mine = append(mine, h)
		}
	}
	// (each team its own call: one not reached doesn't hold up the rest)
	written := make([]bool, len(mine))
	var wg sync.WaitGroup
	for i, h := range mine {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := h.Open()
			if err == nil {
				err = remote.RenameMember(b, h.MemberID, name)
			}
			written[i] = err == nil
		}()
	}
	wg.Wait()
	if _, err := teams.Update(func(s *teams.Store) error {
		for i, h := range mine {
			if st := s.Find(h.ID); written[i] && st != nil {
				st.MemberName = name
			}
		}
		return nil
	}); err != nil {
		return CloudRename{}, err
	}
	out := CloudRename{NotUpdated: []string{}}
	for i, h := range mine {
		if written[i] {
			forgetNames(h.Remote.URL)
			looksCache.forget(h.Remote.URL)
		} else {
			out.NotUpdated = append(out.NotUpdated, h.Name)
		}
	}
	if store, err := teams.Load(); err == nil {
		if t := store.Find(teamID); t != nil {
			out.Team = teamSummary(*t)
		}
	}
	return out, nil
}
