package remote

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// File locks (docs/design/locks.md): a team kept by the hosted service can
// say, before the work starts, who is changing a file that can't be merged.
// The team's switch is in its info (team.json, written by admins):
// "locks": {"on": bool, "kinds": [...]}. On is all the service reads: it
// then refuses a share whose new versions change a path someone else holds.
// Kinds are the kinds of file that lock by themselves when they change
// (empty: manual locks only); only the app reads them.
//
// Turning locks on is a team feature: an older R3V sends branch moves
// without the paths they change, which the service refuses; it must update
// before sharing. Built in Nightly only (locks_nightly.go), as hosted teams
// are.

// FeatureLocks is the team feature turned on with file locks.
const FeatureLocks = "locks"

// FileLocks: file locks are in this build (the Nightly channel).
var FileLocks = false

// ErrLocksNotInBuild: this build has no file locks (Stable).
var ErrLocksNotInBuild = errors.New("file locks need R3V's Nightly channel")

// ErrLocksOff: the team's file locking is off.
var ErrLocksOff = errors.New("file locking is off for this team")

// LockSettings are a team's file locks settings.
type LockSettings struct {
	// On: file locking is on (manual locks, and the service enforcing them).
	On bool `json:"on"`
	// Kinds lock by themselves when they change (ids of profile.LockKinds;
	// empty: manual locks only).
	Kinds []string `json:"kinds"`
}

// LocksOf reads a team's lock settings from its info (off when it has none).
func LocksOf(info TeamInfo) LockSettings {
	if info.Locks == nil {
		return LockSettings{Kinds: []string{}}
	}
	s := *info.Locks
	if s.Kinds == nil {
		s.Kinds = []string{}
	}
	return s
}

// SetLocks writes a team's lock settings (admins). Turning them on turns
// the team feature on in the same write: older R3Vs stop before sharing.
// (Turning them off leaves the feature listed: an R3V that knows locks
// works either way, and one that doesn't would share without them.)
func SetLocks(b Backend, s LockSettings) error {
	if !FileLocks {
		return ErrLocksNotInBuild
	}
	if s.Kinds == nil {
		s.Kinds = []string{}
	}
	info, err := b.Info()
	if err != nil {
		return err // never write the info back without what it holds
	}
	info.Locks = &s
	if s.On && !slices.Contains(info.Features, FeatureLocks) {
		info.Features = append(info.Features, FeatureLocks)
	}
	return b.SetInfo(info)
}

// LockHolder is a path someone holds: a file, or a folder ("x/").
type LockHolder struct {
	Path     string `json:"path"`
	MemberID string `json:"memberId"`
}

// ErrLocked: a share changes paths someone else holds; nothing was written.
type ErrLocked struct{ Locks []LockHolder }

func (e *ErrLocked) Error() string {
	paths := make([]string, len(e.Locks))
	for i, l := range e.Locks {
		paths[i] = l.Path
	}
	return fmt.Sprintf("someone else is editing %s: your versions stay here, shared once it's unlocked",
		strings.Join(paths, ", "))
}

// ErrUpdateR3V: the team needs a newer R3V to share (its file locks are on
// and this R3V moved a branch without the paths it changes).
var ErrUpdateR3V = errors.New("this team uses file locks: update R3V to share")

// ChangesMover is a backend whose branch moves carry the paths their new
// versions change (a hosted team: the service checks them against locks).
type ChangesMover interface {
	UpdateBranchChanged(pid, name, old, new string, changed []string) error
}

// SendsChanges says whether b's branch moves carry the paths they change.
func SendsChanges(b Backend) bool {
	s, ok := b.(*BucketBackend)
	if !ok {
		return false
	}
	_, ok = s.b.(BranchPutter)
	return ok
}

// MoveBranch moves a branch as UpdateBranch does, with the paths the move's
// new versions change, for a backend that carries them.
func MoveBranch(b Backend, pid, name, old, new string, changed []string) error {
	if m, ok := b.(ChangesMover); ok {
		return m.UpdateBranchChanged(pid, name, old, new, changed)
	}
	return b.UpdateBranch(pid, name, old, new)
}

// BranchPutter is storage whose branch writes carry the paths the move
// changes (the hosted service's branch body).
type BranchPutter interface {
	// PutBranch writes head at key (a branch's), cond as in Bucket.Put.
	PutBranch(key, head string, changed []string, cond string) error
}

// LockKeeper is storage that keeps a team's file locks (the hosted
// service).
type LockKeeper interface{ KeepsLocks() bool }

// Capabilities: a hosted team has locks when its file locking is on.
func (s *BucketBackend) Capabilities() Capabilities {
	if !FileLocks {
		return Capabilities{}
	}
	if k, ok := s.b.(LockKeeper); !ok || !k.KeepsLocks() {
		return Capabilities{}
	}
	info, err := s.Info()
	if err != nil {
		return Capabilities{}
	}
	return Capabilities{Locks: LocksOf(info).On}
}

// LockedPaths says which of paths a lock holds (exactly, or under a folder
// lock "x/"), and by whom: the service's rule, for the fake service and for
// telling at once what a share would be refused for.
func LockedPaths(locks []LockHolder, paths []string) []LockHolder {
	var out []LockHolder
	for _, p := range paths {
		for _, l := range locks {
			if Covers(l.Path, p) {
				out = append(out, LockHolder{Path: p, MemberID: l.MemberID})
				break
			}
		}
	}
	return out
}

// Covers says whether a lock on lockPath (a file, or a folder "x/") holds
// path p (paths as versions name them: case counts).
func Covers(lockPath, p string) bool {
	if strings.HasSuffix(lockPath, "/") {
		return strings.HasPrefix(p, lockPath)
	}
	return lockPath == p
}
