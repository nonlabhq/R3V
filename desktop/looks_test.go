package desktop

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
	"github.com/nonlabhq/r3v/internal/teams"
)

func pngURL(t *testing.T, side int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestLooks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("one")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	team, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "one", AccessKey: "k", SecretKey: "s"}, "One")
	if err != nil {
		t.Fatal(err)
	}
	me, err := a.SetIdentity(team.ID, "", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if !me.Looks {
		t.Error("a storage team keeps no looks")
	}
	root := newSong(t)
	tp, err := a.AddProjectToTeam(team.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	// A hosted team that can't be reached (not signed in) is named; the
	// others take it.
	teams.Update(func(s *teams.Store) error {
		h := s.Upsert(remote.Config{URL: cloud.TeamAddress("https://cloud.example", strings.Repeat("1", 32))}, "Hosted")
		h.MemberID, h.MemberName = strings.Repeat("a", 32), "Alice"
		s.Current = team.ID
		return nil
	})

	p, err := a.SetProfileColor("b2")
	if err != nil || p.Color != "b2" || p.Name != "Alice" || !slices.Equal(p.NotShared, []string{"Hosted"}) {
		t.Fatalf("SetProfileColor: %+v %v", p, err)
	}
	if p, err = a.SetProfilePicture(pngURL(t, 128)); err != nil || !strings.HasPrefix(p.Picture, "data:image/png;base64,") {
		t.Fatalf("SetProfilePicture: %+v %v", p, err)
	}
	looks, err := a.MemberLooks(root)
	if l := looks[me.MemberID]; err != nil || l.Color != "b2" || l.Picture != p.Picture {
		t.Fatalf("MemberLooks: %+v %v", looks, err)
	}

	// Another computer: nothing kept yet, the team gives the picture.
	os.RemoveAll(teams.PicturesDir())
	looksCache.forget(fake.URL)
	looksCache.byTeam = map[string]teamLooks{}
	if looks, _ := a.MemberLooks(root); looks[me.MemberID].Picture != p.Picture {
		t.Errorf("from the team: %+v", looks)
	}
	if ents, _ := os.ReadDir(teams.PicturesDir()); len(ents) != 1 {
		t.Errorf("kept: %v", ents)
	}

	for _, bad := range []string{"not a picture", "data:image/png;base64,!!", pngURL(t, 400)} {
		if _, err := a.SetProfilePicture(bad); err == nil {
			t.Errorf("%.30s: taken", bad)
		}
	}
	if p, err = a.SetProfilePicture(""); err != nil || p.Picture != "" || p.Color != "b2" {
		t.Errorf("removing the picture: %+v %v", p, err)
	}

	if err := a.SetProjectLook(team.ID, tp.ID, "drum", "b4"); err == nil {
		t.Error("a look for a project the team doesn't have yet")
	}
	s, _ := teams.Load()
	c, _ := s.Find(team.ID).Open()
	c.PutProject(remote.Project{ID: tp.ID, Name: tp.Name}) // its first version shared
	if err := a.SetProjectLook(team.ID, tp.ID, "drum", "b4"); err != nil {
		t.Fatal(err)
	}
	for _, overview := range []func() (*Overview, error){a.Overview, a.LocalOverview} {
		ov, err := overview()
		if err != nil || len(ov.Projects) != 1 || ov.Projects[0].Icon != "drum" || ov.Projects[0].Color != "b4" {
			t.Errorf("overview: %+v %v", ov, err)
		}
	}

	// A picture of its own, over its icon; the team's icon and picture.
	if err := a.SetProjectPicture(team.ID, tp.ID, pngURL(t, 64)); err != nil {
		t.Fatal(err)
	}
	if !a.CanSetTeamLook(team.ID) {
		t.Error("a storage team's look can't be changed by its member")
	}
	if err := a.SetTeamLook(team.ID, "e-1f3b8", "b6"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetTeamPicture(team.ID, pngURL(t, 32)); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(teams.PicturesDir()) // (another computer: read from the team)
	ov, err := a.Overview()
	if err != nil || !strings.HasPrefix(ov.Projects[0].Picture, "data:image/png") || ov.Projects[0].Icon != "drum" {
		t.Errorf("the project's picture: %+v %v", ov.Projects, err)
	}
	if tm := ov.Teams[0]; tm.Icon != "e-1f3b8" || tm.Color != "b6" || !strings.HasPrefix(tm.Picture, "data:image/png") {
		t.Errorf("the team's look: %+v", tm)
	}
	if err := a.SetTeamPicture(team.ID, ""); err != nil {
		t.Fatal(err)
	}
	if ov, _ := a.LocalOverview(); ov.Teams[0].Picture != "" || ov.Teams[0].Icon != "e-1f3b8" {
		t.Errorf("the team's picture removed: %+v", ov.Teams[0])
	}
	if err := a.SetProjectPicture(team.ID, tp.ID, "data:image/png;base64,bm90"); err == nil {
		t.Error("not a picture: taken")
	}
}

// Kept pictures: named by their sum only; ones nothing showed for a while
// go, the user's own stays.
func TestKeptPictures(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	own, _ := keepPicture([]byte("own"))
	used, _ := keepPicture([]byte("used"))
	old, _ := keepPicture([]byte("old"))
	teams.Update(func(s *teams.Store) error {
		s.Look = &teams.Look{Picture: own}
		return nil
	})
	long := time.Now().Add(-2 * pictureAge)
	for _, sum := range []string{own, used, old} {
		os.Chtimes(filepath.Join(teams.PicturesDir(), sum), long, long)
	}
	if readPicture(used) == nil {
		t.Fatal("a kept picture wasn't read")
	}
	prunePictures()
	for sum, want := range map[string]bool{own: true, used: true, old: false} {
		if _, err := os.Stat(filepath.Join(teams.PicturesDir(), sum)); (err == nil) != want {
			t.Errorf("%s kept: %v, want %v", sum[:8], err == nil, want)
		}
	}
	for _, bad := range []string{strings.ToUpper(used), used[:62] + ":x", "../" + used[3:]} {
		if readPicture(bad) != nil {
			t.Errorf("%q was read", bad)
		}
	}
}
