//go:build !nightly

package desktop

import (
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Stable has no file locks: nothing shows, nothing can be turned on.
func TestStableAppHasNoFileLocks(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	if remote.FileLocks {
		t.Fatal("file locks are on in Stable")
	}
	a := NewApp()
	if tl, err := a.TeamLocks("any"); err != nil || tl.Available || tl.On {
		t.Errorf("team locks: %+v %v", tl, err)
	}
	if err := a.SetTeamLocks("any", true, nil); err == nil {
		t.Error("turned locks on")
	}
	root := newSong(t)
	if v, err := a.ProjectLocks(root); err != nil || v.On {
		t.Errorf("project locks: %+v %v", v, err)
	}
	if _, err := a.LockFiles(root, []string{"Song.als"}); err == nil {
		t.Error("locked a file")
	}
}
