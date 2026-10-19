package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/flock"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// R3V may not write its lock file: no_write_access with the path, exit 1
// (trying again won't help), never project_busy.
func TestNoWriteAccess(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "first"})
	r, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(r.Dir, "lock")
	os.Remove(lock)
	if err := os.Mkdir(lock, 0o755); err != nil { // (a folder where the lock file goes)
		t.Fatal(err)
	}
	exit, rep := run(t, a, "commit", "-m", "first")
	if exit != exitError || rep.Error == nil || rep.Error.Code != "no_write_access" || !strings.EqualFold(filepath.Clean(rep.Error.Path), lock) ||
		strings.Contains(rep.Error.Hint, "R3V_CONFIG_DIR") || !strings.Contains(rep.Error.Hint, "won't help") {
		t.Fatalf("commit with its lock unwritable: %d %+v", exit, rep.Error)
	}

	// R3V's own settings folder: the hint names R3V_CONFIG_DIR.
	ce := classify(&flock.NoAccessError{Path: filepath.Join(teams.Dir(), "teams.lock"), Err: errors.New("Access is denied.")})
	if ce.Code != "no_write_access" || ce.Exit != exitError || !strings.Contains(ce.Hint, "R3V_CONFIG_DIR") {
		t.Fatalf("settings folder: %+v", ce)
	}
	// A held lock stays project_busy.
	if ce := classify(&project.BusyError{}); ce.Code != "project_busy" || ce.Exit != exitLater {
		t.Fatalf("busy: %+v", ce)
	}
}
