package teams

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/flock"
)

// %APPDATA%\R3V denied by a sandbox (Windows' ERROR_ACCESS_DENIED): no
// access, naming the lock file, not busy.
func TestDeniedDirIsNoAccess(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("icacls", dir, "/deny", "*S-1-1-0:(OI)(CI)(W,D)").CombinedOutput(); err != nil {
		t.Skipf("icacls: %v %s", err, out)
	}
	t.Cleanup(func() { exec.Command("icacls", dir, "/remove:d", "*S-1-1-0").Run() })
	t.Setenv("R3V_CONFIG_DIR", dir)
	_, err := Update(func(*Store) error { return nil })
	if !errors.Is(err, flock.ErrNoAccess) || errors.Is(err, flock.ErrBusy) || !strings.Contains(err.Error(), "teams.lock") {
		t.Fatalf("got %v, want no access to teams.lock", err)
	}
}
