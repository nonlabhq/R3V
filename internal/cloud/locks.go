package cloud

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
)

// File locks on a hosted team (docs/design/locks.md): who holds which
// paths of a project, taking and freeing them, and an admin breaking one.
// The service keeps them; a lock lasts until it's freed (heartbeats only
// say who is online).

// Lock is a path someone holds: a file, or a folder (Prefix: Path ends in
// "/", and holds everything under it).
type Lock struct {
	Path      string `json:"path"`
	Prefix    bool   `json:"prefix"`
	MemberID  string `json:"memberId"`
	Workspace string `json:"workspace"`
	Since     Time   `json:"since"` // when it was taken
}

// Time is a time the service sends as Unix milliseconds or RFC 3339.
type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		t.Time = time.UnixMilli(ms)
		return nil
	}
	v, err := time.Parse(time.RFC3339, s)
	if err == nil {
		t.Time = v
	}
	return nil // (a time it can't read shows as none)
}

func (t Time) MarshalJSON() ([]byte, error) { return []byte(strconv.FormatInt(t.UnixMilli(), 10)), nil }

// LockResult is what taking and freeing locks did: the paths now held, and
// the ones refused (someone else holds them, or a folder over them).
type LockResult struct {
	Locked  []string            `json:"locked"`
	Refused []remote.LockHolder `json:"refused"`
}

func locksPath(address, pid string) (service, path string, err error) {
	if !remote.FileLocks {
		return "", "", remote.ErrLocksNotInBuild
	}
	service, ok := remote.BrokerService(address)
	if !ok {
		return "", "", errors.New("file locks are for R3V-Cloud teams")
	}
	return service, teamPath(TeamID(address)) + "/projects/" + url.PathEscape(pid) + "/locks", nil
}

// lockErr turns the service's locks_off into remote.ErrLocksOff.
func lockErr(err error) error {
	var se *serviceError
	if errors.As(err, &se) && se.Code == "locks_off" {
		return remote.ErrLocksOff
	}
	return err
}

// Locks lists a project's locks (address: the hosted team's).
func Locks(address, pid string) ([]Lock, error) {
	svc, p, err := locksPath(address, pid)
	if err != nil {
		return nil, err
	}
	out, err := list[Lock](svc, p)
	return out, lockErr(err)
}

// SetLocks takes the locks on lock and frees those on unlock, for this
// copy of the project (workspace). A path ending in "/" is a folder's.
func SetLocks(address, pid, workspace string, lock, unlock []string) (*LockResult, error) {
	svc, p, err := locksPath(address, pid)
	if err != nil {
		return nil, err
	}
	tok, err := authed(svc)
	if err != nil {
		return nil, err
	}
	if lock == nil {
		lock = []string{}
	}
	if unlock == nil {
		unlock = []string{}
	}
	var out LockResult
	err = call(svc, tok, "POST", p, map[string]any{"lock": lock, "unlock": unlock, "workspace": workspace}, &out)
	if err != nil {
		return nil, lockErr(err)
	}
	if out.Locked == nil {
		out.Locked = []string{}
	}
	if out.Refused == nil {
		out.Refused = []remote.LockHolder{}
	}
	return &out, nil
}

// LockHeartbeat says this copy of the project is online (presence only:
// nothing expires).
func LockHeartbeat(address, pid, workspace string) error {
	svc, p, err := locksPath(address, pid)
	if err != nil {
		return err
	}
	return lockErr(do(svc, "POST", p+"/heartbeat", map[string]string{"workspace": workspace}))
}

// BreakLock frees someone's lock (admins).
func BreakLock(address, pid, path string) error {
	svc, p, err := locksPath(address, pid)
	if err != nil {
		return err
	}
	return lockErr(do(svc, "DELETE", p+"/"+escapePath(path), nil))
}

// escapePath escapes each part of a path, keeping its slashes (and a
// folder's ending one).
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
