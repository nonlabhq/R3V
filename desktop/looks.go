package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/store"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Looks (Nightly, see remote.Looks): the user's colour and picture, shared
// with every team they're in; teammates' as their teams give them; a
// project's icon and colour.

// Profile is the user as the settings panel shows them.
type Profile struct {
	// Available: this build has looks (Nightly).
	Available bool   `json:"available"`
	Name      string `json:"name"`
	MemberID  string `json:"memberId"` // in the current team ("" for none)
	Color     string `json:"color"`
	Picture   string `json:"picture"` // a data: URL, "" for none
	// NotShared names the teams that didn't take the last change (couldn't
	// be reached).
	NotShared []string `json:"notShared"`
}

// MemberLook is how a member shows: a palette name, a data: URL.
type MemberLook struct {
	Color   string `json:"color"`
	Picture string `json:"picture"`
}

// Profile returns the user's name and look.
func (a *App) Profile() (Profile, error) {
	s, err := teams.Load()
	if err != nil {
		return Profile{}, err
	}
	return profileOf(s), nil
}

func profileOf(s *teams.Store) Profile {
	p := Profile{Available: remote.Looks, Name: s.Author, NotShared: []string{}}
	if t := s.Find(s.Current); t != nil && t.MemberID != "" {
		p.Name, p.MemberID = t.MemberName, t.MemberID
	}
	if s.Look != nil {
		p.Color = s.Look.Color
		if s.Look.Picture != "" {
			p.Picture = dataURL(readPicture(s.Look.Picture))
		}
	}
	return p
}

// SetProfileColor sets the user's colour (a palette name) for all their
// teams.
func (a *App) SetProfileColor(color string) (Profile, error) {
	if !remote.Looks {
		return Profile{}, remote.ErrLooksNotInBuild
	}
	if !remote.ValidLookName(color) {
		return Profile{}, errors.New("unknown colour")
	}
	return a.setLook(func(l *teams.Look) { l.Color = color })
}

// SetProfilePicture sets the user's picture (a data: URL of a small square
// PNG or JPEG; "" for none) for all their teams.
func (a *App) SetProfilePicture(url string) (Profile, error) {
	if !remote.Looks {
		return Profile{}, remote.ErrLooksNotInBuild
	}
	sum := ""
	if url != "" {
		data, err := fromDataURL(url)
		if err != nil {
			return Profile{}, err
		}
		if err := remote.CheckPicture(data); err != nil {
			return Profile{}, err
		}
		if sum, err = keepPicture(data); err != nil {
			return Profile{}, err
		}
	}
	return a.setLook(func(l *teams.Look) { l.Picture = sum })
}

// setLook changes the user's look here, then in each of their teams.
func (a *App) setLook(change func(*teams.Look)) (Profile, error) {
	s, err := teams.Update(func(s *teams.Store) error {
		if s.Look == nil {
			s.Look = &teams.Look{}
		}
		change(s.Look)
		return nil
	})
	if err != nil {
		return Profile{}, err
	}
	p := profileOf(s)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range s.Teams {
		if t.MemberID == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := shareLook(t, *s.Look); err != nil {
				mu.Lock()
				p.NotShared = append(p.NotShared, t.Name)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return p, nil
}

// shareLook gives team t the user's look.
func shareLook(t teams.Team, l teams.Look) error {
	var pic []byte
	if l.Picture != "" {
		if pic = readPicture(l.Picture); pic == nil {
			return errors.New("the picture is missing")
		}
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if _, hosted := cloud.Hosted(t); hosted {
		// A hosted team keeps its people in the service: the member's record
		// holds their name and look, written by them the first time.
		ms, err := b.Members()
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(ms, func(m remote.Member) bool { return m.ID == t.MemberID }) {
			if err := b.PutMember(remote.Member{ID: t.MemberID, Name: t.MemberName}); err != nil {
				return err
			}
		}
	}
	err = remote.SetMemberLook(b, t.MemberID, l.Color, pic)
	looksCache.forget(t.Remote.URL)
	return err
}

// shareLookWith gives a team the user joined their look, if they have one
// (none would clear a look this member set on another computer).
func shareLookWith(t teams.Team) {
	s, err := teams.Load()
	if err != nil || s.Look == nil || *s.Look == (teams.Look{}) || !remote.Looks || t.MemberID == "" {
		return
	}
	shareLook(t, *s.Look)
}

// SetProjectLook sets a team project's icon and colour (names) for the
// whole team.
func (a *App) SetProjectLook(teamID, projectID, icon, color string) error {
	if !remote.Looks {
		return remote.ErrLooksNotInBuild
	}
	s, err := teams.Load()
	if err != nil {
		return err
	}
	t := s.Find(teamID)
	if t == nil {
		return errors.New("unknown team")
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if err := remote.SetProjectLook(b, projectID, icon, color); errors.Is(err, remote.ErrNotFound) {
		return errors.New("the team doesn't have this project yet: share a first version, then choose its icon")
	} else if err != nil {
		return err
	}
	if ps, err := b.Projects(); err == nil {
		rememberTeamProjects(teamID, ps)
	}
	return nil
}

// looksCache keeps each team's looks for a minute, so the history doesn't
// ask the team on every render; the team watch forgets them sooner.
var looksCache = cachedLooks{byTeam: map[string]teamLooks{}}

type cachedLooks struct {
	sync.Mutex
	byTeam map[string]teamLooks
}

type teamLooks struct {
	at    time.Time
	looks map[string]MemberLook
}

func (c *cachedLooks) forget(teamURL string) {
	c.Lock()
	delete(c.byTeam, teamURL)
	c.Unlock()
}

// MemberLooks maps the members of a project's team to how they show (none
// for a team that keeps no looks: the app shows initials).
func (a *App) MemberLooks(root string) (map[string]MemberLook, error) {
	out := map[string]MemberLook{}
	if !remote.Looks {
		return out, nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	t, err := r.Team()
	if err != nil {
		return out, nil // not a team's
	}
	key := t.Remote.URL
	looksCache.Lock()
	c, ok := looksCache.byTeam[key]
	looksCache.Unlock()
	if ok && time.Since(c.at) < time.Minute {
		return c.looks, nil
	}
	looks, err := fetchLooks(*t)
	if err != nil {
		if ok {
			return c.looks, nil // as last seen
		}
		return out, nil
	}
	looksCache.Lock()
	looksCache.byTeam[key] = teamLooks{at: time.Now(), looks: looks}
	looksCache.Unlock()
	return looks, nil
}

func fetchLooks(t teams.Team) (map[string]MemberLook, error) {
	out := map[string]MemberLook{}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	ms, err := b.Members()
	if err != nil {
		return nil, err
	}
	ps, _ := b.(remote.PictureStore)
	for _, m := range ms {
		l := MemberLook{Color: m.Color}
		if m.Picture != "" && ps != nil {
			data := readPicture(m.Picture)
			if data == nil {
				if data, err = ps.GetPicture(m.ID, m.Picture); err == nil {
					keepPicture(data)
				}
			}
			l.Picture = dataURL(data)
		}
		if l != (MemberLook{}) {
			out[m.ID] = l
		}
	}
	return out, nil
}

// keepPicture stores a picture in PicturesDir under its SHA-256 (whole or
// not at all), and returns the sum.
func keepPicture(data []byte) (string, error) {
	h := sha256.Sum256(data)
	sum := hex.EncodeToString(h[:])
	path := filepath.Join(teams.PicturesDir(), sum)
	if _, err := os.Stat(path); err == nil {
		return sum, nil
	}
	return sum, store.WriteAtomic(path, bytes.NewReader(data))
}

// readPicture reads a kept picture, nil when it's missing or not what its
// name says. (The name comes from the team's records: only a SHA-256 in
// lowercase hex names a file.)
func readPicture(sum string) []byte {
	if !isSum(sum) {
		return nil
	}
	path := filepath.Join(teams.PicturesDir(), sum)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if h := sha256.Sum256(data); hex.EncodeToString(h[:]) != sum {
		return nil
	}
	now := time.Now()
	os.Chtimes(path, now, now) // in use: prunePictures keeps it
	return data
}

func isSum(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// pictureAge is how long a kept picture nothing showed stays: teammates'
// come again from their teams when needed.
const pictureAge = 30 * 24 * time.Hour

// prunePictures deletes the kept pictures not shown for pictureAge, except
// the user's own, and what a write stopped half-way left.
func prunePictures() {
	s, err := teams.Load()
	if err != nil {
		return
	}
	own := ""
	if s.Look != nil {
		own = s.Look.Picture
	}
	dir := teams.PicturesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || e.Name() == own || time.Since(info.ModTime()) < pictureAge {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
}

func dataURL(data []byte) string {
	if data == nil {
		return ""
	}
	return "data:" + http.DetectContentType(data) + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func fromDataURL(url string) ([]byte, error) {
	_, b64, ok := strings.Cut(url, ";base64,")
	if !ok || !strings.HasPrefix(url, "data:image/") {
		return nil, remote.ErrBadPicture
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, remote.ErrBadPicture
	}
	return data, nil
}
