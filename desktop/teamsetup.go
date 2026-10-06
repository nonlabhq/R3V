package desktop

import (
	"errors"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// TeamConnection is how this computer reaches a team: its storage fields.
// A team on a backend an extension added has only its address, shown as is.
type TeamConnection struct {
	Storage  bool           `json:"storage"`
	Settings remote.Storage `json:"settings"`
	Address  string         `json:"address"`
}

func (c TeamConnection) config() (remote.Config, error) {
	if !c.Storage {
		return remote.Config{}, errors.New("only a storage team's connection can be changed here")
	}
	return c.Settings.Config()
}

// CreateStorageTeam sets up a team on storage (e.g. a Cloudflare R2 bucket):
// it checks the bucket and keys work, names the team (unless the bucket
// already holds a named team) and connects this computer to it. The
// connection code for teammates comes from TeamConnectionCode.
func (a *App) CreateStorageTeam(s remote.Storage, name string) (TeamSummary, error) {
	cfg, err := s.Config()
	if err != nil {
		return TeamSummary{}, err
	}
	if err := remote.Check(cfg); err != nil {
		return TeamSummary{}, err
	}
	b, err := remote.Open(cfg)
	if err != nil {
		return TeamSummary{}, err
	}
	info, err := b.Info()
	if err != nil {
		return TeamSummary{}, err
	}
	if err := remote.Supports(info); err != nil {
		return TeamSummary{}, err
	}
	if name = strings.TrimSpace(name); info.Name == "" && name != "" {
		if err := remote.Rename(b, name); err != nil {
			return TeamSummary{}, err
		}
		info.Name = name
	}
	var t teams.Team
	if _, err := teams.Update(func(s *teams.Store) error {
		u := s.Upsert(cfg, info.Name)
		s.Current = u.ID
		t = *u
		return nil
	}); err != nil {
		return TeamSummary{}, err
	}
	return teamSummary(t), nil
}

// TeamConnectionSettings returns how this computer reaches a team, keys
// included (they are this user's own, shown in their settings).
func (a *App) TeamConnectionSettings(teamID string) (TeamConnection, error) {
	store, err := teams.Load()
	if err != nil {
		return TeamConnection{}, err
	}
	t := store.Find(teamID)
	if t == nil {
		return TeamConnection{}, errors.New("unknown team")
	}
	if s, ok := remote.StorageOf(t.Remote); ok {
		return TeamConnection{Storage: true, Settings: s}, nil
	}
	return TeamConnection{Address: t.Remote.Display()}, nil
}

// TeamConnectionCode is the code teammates paste to join a storage team.
func (a *App) TeamConnectionCode(teamID string) (string, error) {
	store, err := teams.Load()
	if err != nil {
		return "", err
	}
	t := store.Find(teamID)
	if t == nil {
		return "", errors.New("unknown team")
	}
	if !t.Remote.IsStorage() {
		return "", errors.New("this team has no connection code")
	}
	return remote.EncodeConnectionCode(t.Remote), nil
}

// UpdateTeamConnection changes how this computer reaches a team (new keys, a
// moved bucket). The new settings are checked first. When the address
// changes, the team's downloaded projects are pointed at it.
func (a *App) UpdateTeamConnection(teamID string, c TeamConnection) (TeamSummary, error) {
	cfg, err := c.config()
	if err != nil {
		return TeamSummary{}, err
	}
	if err := remote.Check(cfg); err != nil {
		return TeamSummary{}, err
	}
	store, err := teams.Load()
	if err != nil {
		return TeamSummary{}, err
	}
	t := store.Find(teamID)
	if t == nil {
		return TeamSummary{}, errors.New("unknown team")
	}
	if other := store.FindByURL(cfg.URL); other != nil && other.ID != t.ID {
		return TeamSummary{}, errors.New("another team on this computer already uses that address")
	}
	oldURL := t.Remote.URL
	teamName := ""
	if b, err := remote.Open(cfg); err == nil {
		if info, err := b.Info(); err == nil {
			teamName = info.Name
		}
	}
	saved, err := updateTeam(teamID, func(s *teams.Store, t *teams.Team) error {
		if other := s.FindByURL(cfg.URL); other != nil && other.ID != t.ID {
			return errors.New("another team on this computer already uses that address")
		}
		t.Remote = cfg
		s.SyncName(t.ID, teamName)
		return nil
	})
	if err != nil {
		return TeamSummary{}, err
	}
	t = &saved
	if teams.NormalizeURL(oldURL) != teams.NormalizeURL(cfg.URL) {
		for key, root := range store.Projects {
			if strings.HasPrefix(key, teamID+"/") {
				a.repoint(root, cfg.URL)
			}
		}
	}
	return teamSummary(*t), nil
}

// repoint makes a downloaded project use the team's new address.
func (a *App) repoint(root, url string) {
	a.stopWatch(root)
	r, unlock, err := a.open(root)
	if err == nil {
		if r.Config.Remote != nil {
			r.Config.Remote = &project.RemoteConfig{URL: url}
			r.SaveConfig()
		}
		unlock()
	}
	a.startWatch(root)
}

// StorageCleanup is what cleaning up a team's storage found (and did).
type StorageCleanup struct {
	Versions int `json:"versions"` // version records read, all projects
	Stored   int `json:"stored"`   // files in storage
	// Unused files: due for deleting now; waiting (all unused, when only
	// checking); deleted.
	Unused       int    `json:"unused"`
	UnusedBytes  int64  `json:"unusedBytes"`
	Due          int    `json:"due"`
	DueBytes     int64  `json:"dueBytes"`
	Deleted      int    `json:"deleted"`
	DeletedBytes int64  `json:"deletedBytes"`
	NextCleanup  string `json:"nextCleanup"` // when waiting files can go (RFC 3339), "" if none
}

// CleanUpStorage finds files in a team's storage that no version of any
// project uses; with remove it deletes those found unused at least a day
// before (and not written in the last week). See remote.CollectGarbage.
func (a *App) CleanUpStorage(teamID string, remove bool) (*StorageCleanup, error) {
	store, err := teams.Load()
	if err != nil {
		return nil, err
	}
	t := store.Find(teamID)
	if t == nil {
		return nil, errors.New("unknown team")
	}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	s3, ok := b.(*remote.BucketBackend)
	if !ok {
		return nil, errors.New("cleanup works on teams that use S3 or R2 storage")
	}
	rep, err := s3.CollectGarbage(remove)
	if err != nil {
		return nil, err
	}
	out := &StorageCleanup{Versions: rep.Versions, Stored: rep.Stored, Due: rep.Due, DueBytes: rep.DueBytes,
		Deleted: rep.Deleted, DeletedBytes: rep.DeletedBytes,
		Unused: rep.Waiting + rep.Deleted, UnusedBytes: rep.WaitingBytes + rep.DeletedBytes}
	if !rep.NextCleanup.IsZero() {
		out.NextCleanup = rep.NextCleanup.UTC().Format(time.RFC3339)
	}
	return out, nil
}
