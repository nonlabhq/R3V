//go:build !nightly

package desktop

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Stable has no looks: it changes none, and shows none (the history and
// the projects keep their initials).
func TestStableHasNoLooks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	a := NewApp()
	if p, err := a.Profile(); err != nil || p.Available {
		t.Errorf("Profile: %+v %v", p, err)
	}
	if _, err := a.SetProfileColor("palette-1"); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("SetProfileColor: %v", err)
	}
	if _, err := a.SetProfilePicture("data:image/png;base64,"); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("SetProfilePicture: %v", err)
	}
	if err := a.SetProjectLook("t", "p", "drum", ""); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("SetProjectLook: %v", err)
	}
	if s, _ := teams.Load(); s.Look != nil {
		t.Errorf("look kept: %+v", s.Look)
	}
	if looks, err := a.MemberLooks(t.TempDir()); err != nil || len(looks) != 0 {
		t.Errorf("MemberLooks: %v %v", looks, err)
	}
	if s := teamSummary(teams.Team{Remote: remote.Config{URL: "s3+https://storage.example/bucket/r3v"}}); s.Looks {
		t.Error("a team keeps looks in Stable")
	}
}
