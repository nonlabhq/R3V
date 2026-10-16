package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/flock"
	"github.com/nonlabhq/r3v/internal/store"
)

// ErrBusy: another R3V (the app, the command line, another edition) is
// working on the project. The error Lock returns is a *BusyError, which
// says who when it can.
var ErrBusy = errors.New("another R3V window or command is working on this project; try again when it's done")

// LockProgram and LockCommand describe this program in the holder record
// of the locks it takes ("app" or "cli", and the command), so another one
// refused can say who has the project.
var LockProgram, LockCommand string

// holderFile is beside the lock file, not in it: on Windows no one else
// can open the lock file while it is held.
const holderFile = "lock-holder.json"

// Holder is who holds a project's lock, as it recorded itself.
type Holder struct {
	Program string    `json:"program"` // "app", "cli" ("" another program)
	Command string    `json:"command,omitempty"`
	PID     int       `json:"pid"`
	Since   time.Time `json:"since"`
	// Running is not stored: whether the process was still there when read
	// (not: the record was left by one that ended, and whoever holds the
	// lock now didn't say, an older R3V say).
	Running bool `json:"-"`
}

// String names the holder for a message: "r3v commit (pid 1234) since
// 14:02:11".
func (h *Holder) String() string {
	var who string
	switch h.Program {
	case "app":
		who = "the R3V app"
	case "cli":
		who = strings.TrimSpace("r3v " + h.Command)
	default:
		who = strings.TrimSpace("R3V " + h.Command)
	}
	pid := fmt.Sprintf("pid %d", h.PID)
	if !h.Running {
		pid += ", no longer running"
	}
	since := h.Since.Local()
	format := "15:04:05"
	if y, m, d := since.Date(); y != time.Now().Year() || m != time.Now().Month() || d != time.Now().Day() {
		format = "2006-01-02 15:04:05"
	}
	return fmt.Sprintf("%s (%s) since %s", who, pid, since.Format(format))
}

// BusyError is ErrBusy with who holds the lock (Holder nil: not known).
type BusyError struct {
	Holder *Holder
}

func (e *BusyError) Error() string {
	if e.Holder == nil {
		return ErrBusy.Error()
	}
	if !e.Holder.Running {
		return fmt.Sprintf("another R3V window or command is working on this project (the last one recorded, %s, has ended: an older R3V may hold it); try again when it's done", e.Holder)
	}
	return fmt.Sprintf("another R3V is working on this project: %s; try again when it's done", e.Holder)
}

func (e *BusyError) Is(target error) bool { return target == ErrBusy }

// Lock keeps other programs from changing the project until the returned
// function is called (or this program ends), waiting up to wait for one
// that has it. Within one program, callers take turns themselves.
func (r *Repo) Lock(wait time.Duration) (func(), error) {
	unlock, err := flock.Lock(filepath.Join(r.Dir, "lock"), wait)
	if errors.Is(err, flock.ErrBusy) {
		return nil, &BusyError{Holder: r.LockHolder()}
	}
	if err != nil {
		return nil, err
	}
	// Best effort: without it, a program refused only says the project is busy.
	path := filepath.Join(r.Dir, holderFile)
	h := Holder{Program: LockProgram, Command: LockCommand, PID: os.Getpid(), Since: time.Now().UTC().Truncate(time.Second)}
	if data, err := json.Marshal(h); err == nil {
		store.WriteAtomic(path, strings.NewReader(string(data)+"\n"))
	}
	return func() {
		store.Remove(path) // (while still held: the next holder's record stays)
		unlock()
	}, nil
}

// LockHolder is who recorded holding the project's lock last (nil: none
// did, or it was let go).
func (r *Repo) LockHolder() *Holder {
	data, err := os.ReadFile(filepath.Join(r.Dir, holderFile))
	if err != nil {
		return nil
	}
	var h Holder
	if json.Unmarshal(data, &h) != nil || h.PID == 0 {
		return nil
	}
	h.Running = processRunning(h.PID)
	return &h
}
