// Package flock locks a file between processes (and goroutines): the app,
// the command line tool and background work can all reach the same project
// or settings, and only one may change them at a time.
package flock

import (
	"errors"
	"io/fs"
	"time"
)

// ErrBusy: the lock stayed taken for as long as the caller would wait.
var ErrBusy = errors.New("locked by another R3V")

// ErrNoAccess: R3V may not write where it keeps something (a sandbox's
// folder, a read-only one, someone else's): no one holds it, so trying
// again won't help. The error is a *NoAccessError, naming the path.
var ErrNoAccess = errors.New("no write access")

// NoAccessError is ErrNoAccess for one path.
type NoAccessError struct {
	Path string
	Err  error // what the system said
}

func (e *NoAccessError) Error() string {
	return "R3V can't write " + e.Path + " (" + e.Err.Error() + "): it needs write access there"
}

func (e *NoAccessError) Is(target error) bool { return target == ErrNoAccess }
func (e *NoAccessError) Unwrap() error        { return e.Err }

// NoAccess is err as a *NoAccessError when it says R3V may not write path
// (denied, read-only, a file where a folder must be); any other err as it
// is.
func NoAccess(path string, err error) error {
	var na *NoAccessError
	if err == nil || errors.As(err, &na) || !deniedWrite(err) {
		return err
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		path, err = pe.Path, pe.Err
	}
	return &NoAccessError{Path: path, Err: err}
}

func deniedWrite(err error) bool {
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	for _, e := range deniedErrnos {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

var errLocked = errors.New("locked")

// deniedError: the lock file can't be opened for writing. On Windows that
// is also, for a moment, a lock file being deleted: it is tried again for
// a little while before it is said to be a matter of access.
type deniedError struct{ err error }

func (e *deniedError) Error() string { return e.err.Error() }

// deniedFor is how long a lock file R3V may not open is tried again.
var deniedFor = 300 * time.Millisecond

// Lock takes the lock on path (the file is made if needed), waiting up to
// wait for it; the returned func releases it. The lock also ends with the
// process. Held by someone past wait: ErrBusy; R3V may not open it at
// all: a *NoAccessError.
func Lock(path string, wait time.Duration) (func(), error) {
	start := time.Now()
	for {
		unlock, err := lockFile(path)
		if err == nil {
			return unlock, nil
		}
		var denied *deniedError
		switch {
		case errors.As(err, &denied):
			if time.Since(start) >= min(wait, deniedFor) {
				return nil, &NoAccessError{Path: path, Err: denied.err}
			}
		case !errors.Is(err, errLocked):
			return nil, NoAccess(path, err)
		case time.Since(start) > wait:
			return nil, ErrBusy
		}
		time.Sleep(20 * time.Millisecond)
	}
}
