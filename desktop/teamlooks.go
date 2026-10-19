package desktop

import (
	"errors"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// How a team and its projects show, for the whole team (as the members'
// looks: looks.go): an icon and a colour, or a picture of their
// own over the icon. Pictures come as data: URLs of small square PNG or
// JPEG (the page makes them, picture.ts) and are kept here too, so the
// sidebar shows them without asking the team.

// fetchPicture keeps picture sum of dir here, read from the team when it
// isn't yet ("" or unreadable: nothing).
func fetchPicture(b remote.Backend, dir, sum string) {
	if sum == "" || dir == "" || readPicture(sum) != nil {
		return
	}
	ps, ok := b.(remote.PictureStore)
	if !ok {
		return
	}
	if data, err := ps.GetPictureIn(dir, sum); err == nil {
		keepPicture(data)
	}
}

// pictureOf decodes a data: URL the page made ("" for none: nil).
func pictureOf(url string) ([]byte, error) {
	if url == "" {
		return nil, nil
	}
	data, err := fromDataURL(url)
	if err != nil {
		return nil, err
	}
	if err := remote.CheckPicture(data); err != nil {
		return nil, err
	}
	if _, err := keepPicture(data); err != nil {
		return nil, err
	}
	return data, nil
}

func openTeam(teamID string) (remote.Backend, error) {
	if !remote.Looks {
		return nil, remote.ErrLooksNotInBuild
	}
	s, err := teams.Load()
	if err != nil {
		return nil, err
	}
	t := s.Find(teamID)
	if t == nil {
		return nil, errors.New("unknown team")
	}
	return t.Open()
}

// SetProjectPicture sets a team project's own picture (a data: URL; "" for
// none: its icon shows) for the whole team.
func (a *App) SetProjectPicture(teamID, projectID, url string) error {
	b, err := openTeam(teamID) // (Stable: refused before anything is kept)
	if err != nil {
		return err
	}
	data, err := pictureOf(url)
	if err != nil {
		return err
	}
	if err := remote.SetProjectPicture(b, projectID, data); errors.Is(err, remote.ErrNotFound) {
		return errors.New("the team doesn't have this project yet: share a first version, then choose its picture")
	} else if err != nil {
		return err
	}
	if ps, err := b.Projects(); err == nil {
		rememberTeamProjects(teamID, ps)
	}
	return nil
}

// SetTeamLook sets the team's icon and colour (names) for the whole team.
func (a *App) SetTeamLook(teamID, icon, color string) error {
	b, err := openTeam(teamID)
	if err != nil {
		return err
	}
	if err := remote.SetTeamLook(b, icon, color); err != nil {
		return err
	}
	return syncTeamLook(teamID, b)
}

// SetTeamPicture sets the team's own picture (a data: URL; "" for none) for
// the whole team.
func (a *App) SetTeamPicture(teamID, url string) error {
	b, err := openTeam(teamID) // (Stable: refused before anything is kept)
	if err != nil {
		return err
	}
	data, err := pictureOf(url)
	if err != nil {
		return err
	}
	if err := remote.SetTeamPicture(b, data); err != nil {
		return err
	}
	return syncTeamLook(teamID, b)
}

// syncTeamLook keeps the team's look here as it is now.
func syncTeamLook(teamID string, b remote.Backend) error {
	info, err := b.Info()
	if err != nil {
		return err
	}
	teams.SyncTeamLook(teamID, teams.TeamLook{Icon: info.Icon, Color: info.Color, Picture: info.Picture})
	return nil
}

// CanSetTeamLook reports whether the team's look may be changed from here:
// on a hosted team, by its owners and admins (as its name); on storage, by
// anyone in it.
func (a *App) CanSetTeamLook(teamID string) bool {
	if !remote.Looks {
		return false
	}
	s, err := teams.Load()
	if err != nil {
		return false
	}
	t := s.Find(teamID)
	if t == nil {
		return false
	}
	if _, hosted := cloud.Hosted(*t); hosted {
		return isAdmin(myRole(t))
	}
	return true
}
