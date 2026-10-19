package desktop

import (
	"slices"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/teams"
)

func memberName(t *testing.T, team teams.Team, id string) string {
	t.Helper()
	b, err := team.Open()
	if err != nil {
		t.Fatal(err)
	}
	ms, err := b.Members()
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.IndexFunc(ms, func(m remote.Member) bool { return m.ID == id }); i >= 0 {
		return ms[i].Name
	}
	return ""
}

// A name changed on an R3V Cloud team: on the service (PATCH /v1/me), then
// in the person's member record there (a member writes their own) and in
// the team list; a self-hosted team keeps its own name. The service not
// reached: said plainly, nothing changed.
func TestSetCloudName(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	t.Cleanup(waitTidy)
	f := cloudtest.NewFake(t)
	f.AddMember("kai", kaiID, false)
	t.Setenv("R3V_CLOUD_TOKEN", "kai")
	a := NewApp()
	teamSong(t, a)
	store, _ := teams.Load()
	own := store.Teams[0]
	if _, err := a.SetIdentity(own.ID, "", "Kai at home"); err != nil {
		t.Fatal(err)
	}
	hosted, err := a.ConnectTeam(f.Address)
	if err != nil {
		t.Fatal(err)
	}
	teams.Update(func(s *teams.Store) error {
		s.Find(hosted.ID).MemberID, s.Find(hosted.ID).MemberName = kaiID, "kai"
		return nil
	})

	got, err := a.SetCloudName(hosted.ID, "  Kai Ito ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Team.MemberName != "Kai Ito" || len(got.NotUpdated) != 0 {
		t.Errorf("renamed: %+v", got)
	}
	if n := f.UserName("kai"); n != "Kai Ito" {
		t.Errorf("the service has %q", n)
	}
	store, _ = teams.Load()
	if n := memberName(t, *store.Find(hosted.ID), kaiID); n != "Kai Ito" {
		t.Errorf("the member record says %q", n)
	}
	if s := store.Find(own.ID); s.MemberName != "Kai at home" || memberName(t, *s, s.MemberID) != "Kai at home" {
		t.Errorf("the self-hosted team changed: %q", s.MemberName)
	}

	// Another member's record isn't ours to write (the fake refuses, as
	// the service does).
	b, _ := store.Find(hosted.ID).Open()
	if err := remote.RenameMember(b, cloudtest.Owner, "Not me"); err == nil {
		t.Error("wrote someone else's record")
	}

	f.Down.Store(true)
	if _, err := a.SetCloudName(hosted.ID, "Offline"); err == nil || !strings.Contains(err.Error(), "can't be reached") {
		t.Errorf("offline: %v", err)
	}
	f.Down.Store(false)
	store, _ = teams.Load()
	if f.UserName("kai") != "Kai Ito" || store.Find(hosted.ID).MemberName != "Kai Ito" {
		t.Error("changed while offline")
	}
	if _, err := a.SetCloudName(hosted.ID, " "); err == nil {
		t.Error("an empty name was taken")
	}
	if _, err := a.SetCloudName(own.ID, "Kai"); err == nil {
		t.Error("a self-hosted team's name went to R3V Cloud")
	}
}
