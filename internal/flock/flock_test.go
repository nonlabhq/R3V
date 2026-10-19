package flock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A lock someone holds is busy (try again); one R3V may not open is not.
func TestHeldIsBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	unlock, err := Lock(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(path, 50*time.Millisecond); !errors.Is(err, ErrBusy) || errors.Is(err, ErrNoAccess) {
		t.Fatalf("held: %v, want ErrBusy", err)
	}
	unlock()
	again, err := Lock(path, time.Second)
	if err != nil {
		t.Fatalf("let go, then: %v", err)
	}
	again()
}

// A folder where the lock file must be: no one holds it, and waiting
// longer changes nothing.
func TestCantOpenIsNoAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := Lock(path, 10*time.Second)
	var na *NoAccessError
	if !errors.Is(err, ErrNoAccess) || errors.Is(err, ErrBusy) || !errors.As(err, &na) || na.Path != path {
		t.Fatalf("got %v, want a NoAccessError for %s", err, path)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("waited %v for a lock no one holds", time.Since(start))
	}
}

// A file where the folder must be (an unwritable settings folder, say).
func TestNoAccessFileForFolder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "R3V")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := NoAccess(file, os.MkdirAll(file, 0o700))
	if !errors.Is(err, ErrNoAccess) {
		t.Fatalf("got %v, want ErrNoAccess", err)
	}
	if err := NoAccess(file, errors.New("other")); errors.Is(err, ErrNoAccess) {
		t.Fatal("any error taken for no access")
	}
	if NoAccess(file, nil) != nil {
		t.Fatal("nil isn't an error")
	}
}
