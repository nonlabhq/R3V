package flock

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// denyWrite makes dir unwritable for everyone (as a sandbox blocking
// %APPDATA%\R3V does), until the test ends.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("icacls", dir, "/deny", "*S-1-1-0:(OI)(CI)(W,D)").CombinedOutput(); err != nil {
		t.Skipf("icacls: %v %s", err, out)
	}
	t.Cleanup(func() { exec.Command("icacls", dir, "/remove:d", "*S-1-1-0").Run() })
}

// A folder R3V may not write (Windows' ERROR_ACCESS_DENIED) is no access,
// not a lock someone holds.
func TestDeniedFolderIsNoAccess(t *testing.T) {
	dir := t.TempDir()
	denyWrite(t, dir)
	path := filepath.Join(dir, "teams.lock")
	_, err := Lock(path, 10*time.Second)
	var na *NoAccessError
	if !errors.As(err, &na) || errors.Is(err, ErrBusy) || na.Path != path {
		t.Fatalf("got %v, want a NoAccessError for %s", err, path)
	}
}
