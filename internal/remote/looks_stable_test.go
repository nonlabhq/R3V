//go:build !nightly

package remote_test

import (
	"errors"
	"image/color"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Stable has no looks: it writes none and reads none, and works with a
// team that has them as before (it rewrites records whole: renaming keeps
// a look Nightly set).
func TestStableHasNoPictures(t *testing.T) {
	if remote.Looks {
		t.Fatal("member pictures are on in Stable")
	}
	f, b := newTeam(t)
	if err := remote.SetMemberLook(b, alice, "1", picture(t, 128, color.White)); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("SetMemberLook: %v", err)
	}
	if _, err := b.PutPicture(alice, picture(t, 128, color.White)); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("PutPicture: %v", err)
	}
	b.PutProject(remote.Project{ID: song, Name: "Song"})
	if err := remote.SetProjectLook(b, song, "drum", "palette-1"); !errors.Is(err, remote.ErrLooksNotInBuild) {
		t.Errorf("SetProjectLook: %v", err)
	}
	if keys := pictureKeys(t, f); len(keys) != 0 {
		t.Errorf("stored: %v", keys)
	}
	if info, _ := b.Info(); len(info.Features) != 0 {
		t.Errorf("features turned on: %v", info.Features)
	}
	// A team a test build turned the looks feature on for still opens.
	if err := remote.Supports(remote.TeamInfo{Features: []string{remote.FeatureLooks}}); err != nil {
		t.Errorf("a team with looks: %v", err)
	}
}
