//go:build !nightly

package remote_test

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

// Stable has no file locks: it can't turn them on, shows none, and stops
// before working with a team that has them (it would share without the
// paths a share changes).
func TestStableHasNoFileLocks(t *testing.T) {
	if remote.FileLocks {
		t.Fatal("file locks are on in Stable")
	}
	b := remote.NewBucketBackend(membucket.New())
	if err := remote.SetLocks(b, remote.LockSettings{On: true}); !errors.Is(err, remote.ErrLocksNotInBuild) {
		t.Errorf("SetLocks: %v", err)
	}
	if info, _ := b.Info(); info.Locks != nil || len(info.Features) != 0 {
		t.Errorf("info written: %+v", info)
	}
	if remote.CapabilitiesOf(b).Locks {
		t.Error("locks shown")
	}
	var ef *remote.ErrTeamFeatures
	if err := remote.Supports(remote.TeamInfo{Features: []string{remote.FeatureLocks}}); !errors.As(err, &ef) {
		t.Errorf("a team with file locks: %v", err)
	}
}
