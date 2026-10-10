package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/cloud"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// File locks (docs/design/locks.md), on hosted teams that turned them on:
// who is changing which file that can't be merged. The service keeps them
// and enforces them on shares; the app shows them, takes them (by hand,
// or by itself when a file of an auto-locked kind changes), frees them
// (by hand, or when the changes are discarded) and tells at once when a
// change can't be shared because someone else holds the file. Off (the
// team's switch, the project's .r3v.yaml, a team on its own storage, a
// Stable build), nothing about locks shows or happens.

// LocksView is a project's file locks, as the app shows them.
type LocksView struct {
	// On: locks exist in this project; nothing below matters otherwise.
	On bool `json:"on"`
	// Me: your member id in the team; Admin: you may break others' locks.
	Me    string `json:"me"`
	Admin bool   `json:"admin"`
	// AutoLock: some kinds of file lock by themselves when they change.
	AutoLock bool       `json:"autoLock"`
	Items    []LockItem `json:"items"`
	// Waiting: files that changed while the team couldn't be reached, to
	// lock once it can.
	Waiting []string `json:"waiting"`
	// Offline: why the locks couldn't be read now ("" when they were).
	Offline string `json:"offline"`
}

// LockItem is a lock: a file, or a folder (Prefix, Path ending in "/").
type LockItem struct {
	Path     string `json:"path"`
	Prefix   bool   `json:"prefix"`
	MemberID string `json:"memberId"`
	Name     string `json:"name"` // the holder's name ("" when not known)
	Since    string `json:"since"`
	Mine     bool   `json:"mine"`
}

// HeldLock is a path someone else holds, for telling why something was
// refused.
type HeldLock struct {
	Path     string `json:"path"`
	MemberID string `json:"memberId"`
	Name     string `json:"name"`
}

// lockTarget is what locking a project needs: the team's address, the
// project, this copy, you.
type lockTarget struct {
	r         *project.Repo
	team      *teams.Team
	address   string
	workspace string
	locking   profile.Locking
}

// teamLocksCache keeps each hosted team's switch a minute (every page of a
// project asks).
var teamLocksCache = struct {
	sync.Mutex
	byURL map[string]cachedTeamLocks
}{byURL: map[string]cachedTeamLocks{}}

type cachedTeamLocks struct {
	at time.Time
	s  remote.LockSettings
}

func forgetTeamLocks(url string) {
	teamLocksCache.Lock()
	delete(teamLocksCache.byURL, url)
	teamLocksCache.Unlock()
}

// teamLockSettings reads a hosted team's lock settings (off for any other).
func teamLockSettings(t *teams.Team, fresh bool) (remote.LockSettings, error) {
	off := remote.LockSettings{Kinds: []string{}}
	if !remote.FileLocks {
		return off, nil
	}
	if _, ok := cloud.Hosted(*t); !ok {
		return off, nil
	}
	teamLocksCache.Lock()
	c, ok := teamLocksCache.byURL[t.Remote.URL]
	teamLocksCache.Unlock()
	if ok && !fresh && time.Since(c.at) < time.Minute {
		return c.s, nil
	}
	b, err := t.Open()
	if err != nil {
		return off, err
	}
	info, err := b.Info()
	if err != nil {
		if ok {
			return c.s, err // (the last known, said to be old)
		}
		return off, err
	}
	s := remote.LocksOf(info)
	teamLocksCache.Lock()
	teamLocksCache.byURL[t.Remote.URL] = cachedTeamLocks{time.Now(), s}
	teamLocksCache.Unlock()
	return s, nil
}

// lockTargetOf is a project's locking, nil when it has none.
func lockTargetOf(root string) (*lockTarget, error) {
	if !remote.FileLocks {
		return nil, nil
	}
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	if r.Config.Remote == nil {
		return nil, nil
	}
	t, err := r.Team()
	if err != nil {
		return nil, nil
	}
	if _, ok := cloud.Hosted(*t); !ok || t.NoAccess {
		return nil, nil
	}
	s, err := teamLockSettings(t, false)
	if err != nil && !s.On {
		return nil, err
	}
	rules, _ := r.Profile()
	l := profile.ResolveLocking(profile.TeamLocks{On: s.On, Kinds: s.Kinds}, profile.FileLocks{})
	if rules != nil {
		l = rules.Locking(profile.TeamLocks{On: s.On, Kinds: s.Kinds})
	}
	if !l.On {
		return nil, nil
	}
	return &lockTarget{r: r, team: t, address: t.Remote.URL, workspace: workspaceOf(r), locking: l}, nil
}

// workspaceOf names this copy of the project to the service: its id in the
// team, or (none yet) one made from the project and its folder.
func workspaceOf(r *project.Repo) string {
	if r.Config.WorkspaceID != "" {
		return r.Config.WorkspaceID
	}
	s := sha256.Sum256([]byte(r.Config.ProjectID + "\x00" + strings.ToLower(r.Root)))
	return hex.EncodeToString(s[:16])
}

// roles keeps your role in each hosted team a few minutes (an admin may
// break locks).
var roles = struct {
	sync.Mutex
	byTeam map[string]cachedRole
}{byTeam: map[string]cachedRole{}}

type cachedRole struct {
	at   time.Time
	role string
}

func myRole(t *teams.Team) string {
	svc, ok := cloud.Hosted(*t)
	if !ok {
		return ""
	}
	team := cloud.TeamID(t.Remote.URL)
	roles.Lock()
	c, ok := roles.byTeam[t.Remote.URL]
	roles.Unlock()
	if ok && time.Since(c.at) < 5*time.Minute {
		return c.role
	}
	me, err := cloud.GetMe(svc)
	if err != nil {
		return c.role
	}
	role := ""
	for _, mt := range me.Teams {
		if mt.ID == team {
			role = mt.Role
		}
	}
	roles.Lock()
	roles.byTeam[t.Remote.URL] = cachedRole{time.Now(), role}
	roles.Unlock()
	return role
}

func isAdmin(role string) bool { return role == "owner" || role == "admin" }

// lastLocks keeps each project's locks as last read, for the notices of
// a change to a file someone else holds (no request each time a file
// changes) and while the team can't be reached.
var lastLocks = struct {
	sync.Mutex
	byRoot map[string][]cloud.Lock
}{byRoot: map[string][]cloud.Lock{}}

func (lt *lockTarget) read() ([]cloud.Lock, error) {
	ls, err := cloud.Locks(lt.address, lt.r.Config.ProjectID)
	lastLocks.Lock()
	defer lastLocks.Unlock()
	if err != nil {
		return lastLocks.byRoot[lt.r.Root], err
	}
	lastLocks.byRoot[lt.r.Root] = ls
	return ls, nil
}

// forgetLocks: the locks changed elsewhere (a share, someone's notice):
// the next look asks the service.
func forgetLocks(root string) {
	lastLocks.Lock()
	delete(lastLocks.byRoot, root)
	lastLocks.Unlock()
}

// afterShare: a share freed your locks on what it changed.
func (a *App) afterShare(root string) { a.locksChanged(root) }

func cachedLocks(root string) []cloud.Lock {
	lastLocks.Lock()
	defer lastLocks.Unlock()
	return lastLocks.byRoot[root]
}

// ProjectLocks reads a project's file locks (On false when it has none).
func (a *App) ProjectLocks(root string) (*LocksView, error) {
	out := &LocksView{Items: []LockItem{}, Waiting: []string{}}
	if !knownProject(root) {
		return out, nil
	}
	lt, err := lockTargetOf(root)
	if err != nil {
		out.Offline = err.Error()
		return out, nil
	}
	if lt == nil {
		return out, nil
	}
	out.On, out.Me, out.AutoLock = true, lt.team.MemberID, len(lt.locking.Auto) > 0
	out.Admin = isAdmin(myRole(lt.team))
	out.Waiting = lt.r.WaitingLocks()
	ls, err := lt.read()
	if err != nil {
		out.Offline = err.Error()
	}
	names := a.memberNames(lt.r)
	for _, l := range ls {
		out.Items = append(out.Items, LockItem{Path: l.Path, Prefix: l.Prefix, MemberID: l.MemberID, Name: names[l.MemberID],
			Since: l.Since.UTC().Format(time.RFC3339), Mine: l.MemberID == out.Me})
	}
	return out, nil
}

// LockOutcome is what locking did: what is now yours, and what someone
// else holds.
type LockOutcome struct {
	Locked  []string   `json:"locked"`
	Refused []HeldLock `json:"refused"`
}

func (a *App) held(r *project.Repo, hs []remote.LockHolder) []HeldLock {
	out := []HeldLock{}
	if len(hs) == 0 {
		return out
	}
	names := a.memberNames(r)
	for _, h := range hs {
		out = append(out, HeldLock{Path: h.Path, MemberID: h.MemberID, Name: names[h.MemberID]})
	}
	return out
}

// LockFiles locks files, or folders (a path ending in "/": everything in
// it, files added later too).
func (a *App) LockFiles(root string, paths []string) (*LockOutcome, error) {
	lt, err := lockTargetOf(root)
	if err != nil {
		return nil, err
	}
	if lt == nil {
		return nil, errors.New("file locking is off for this project")
	}
	res, err := cloud.SetLocks(lt.address, lt.r.Config.ProjectID, lt.workspace, paths, nil)
	if err != nil {
		return nil, err
	}
	a.locksChanged(root)
	return &LockOutcome{Locked: res.Locked, Refused: a.held(lt.r, res.Refused)}, nil
}

// UnlockFiles frees your locks on paths.
func (a *App) UnlockFiles(root string, paths []string) error {
	lt, err := lockTargetOf(root)
	if err != nil || lt == nil {
		return err
	}
	if _, err := cloud.SetLocks(lt.address, lt.r.Config.ProjectID, lt.workspace, nil, paths); err != nil {
		return err
	}
	a.locksChanged(root)
	return nil
}

// BreakLock frees someone else's lock (admins; the service checks).
func (a *App) BreakLock(root, path string) error {
	lt, err := lockTargetOf(root)
	if err != nil || lt == nil {
		return err
	}
	if err := cloud.BreakLock(lt.address, lt.r.Config.ProjectID, path); err != nil {
		return err
	}
	a.locksChanged(root)
	return nil
}

// LocksEvent: a project's locks changed: the page reads them again.
type LocksEvent struct {
	Root string `json:"root"`
	// Path and Name: a lock taken or freed by someone (live notices; ""
	// when not told); Locked, taken; Shared, freed by its holder's share.
	Path   string `json:"path"`
	Name   string `json:"name"`
	Locked bool   `json:"locked"`
	Shared bool   `json:"shared"`
	Mine   bool   `json:"mine"`
}

func (a *App) locksChanged(root string) {
	forgetLocks(root)
	a.emitLocks(root)
}

// emitLocks tells the page to read the locks again (the ones kept here are
// current).
func (a *App) emitLocks(root string) {
	if a.emit != nil {
		a.emit("locks", LocksEvent{Root: root})
	}
}

// onLock follows a lock taken or freed in a hosted team (live notices).
func (a *App) onLock(service, team string, n cloud.LockNotice) {
	store, err := teams.Load()
	if err != nil {
		return
	}
	t := store.FindByURL(cloud.TeamAddress(service, team))
	if t == nil {
		return
	}
	root := store.Projects[t.ID+"/"+n.Project]
	if root == "" {
		return
	}
	forgetLocks(root)
	name := ""
	if r, err := project.Open(root); err == nil {
		name = a.memberNames(r)[n.MemberID]
	}
	if a.emit != nil {
		a.emit("locks", LocksEvent{Root: root, Path: n.Path, Name: name, Locked: n.Locked, Shared: n.Shared,
			Mine: n.MemberID != "" && n.MemberID == t.MemberID})
	}
}

// --- taking locks by themselves -------------------------------------------------

// told: the notices given (root + path + holder), so a file changed again
// and again is told of once.
var told = struct {
	sync.Mutex
	m map[string]bool
}{m: map[string]bool{}}

// lockQueue serializes the waiting list of each project.
var lockQueue sync.Mutex

// autoLock locks the files among changed (slash paths) of an auto-locked
// kind that aren't yours yet, with any waiting from before (best effort;
// the team out of reach: they wait). A file someone else holds is told of
// at once.
func (a *App) autoLock(root string, changed []string) {
	lt, err := lockTargetOf(root)
	if err != nil || lt == nil {
		return
	}
	lockQueue.Lock()
	defer lockQueue.Unlock()
	waiting := lt.r.WaitingLocks()
	rules, _ := lt.r.Profile()
	var want []string
	for _, p := range append(slices.Clone(waiting), changed...) {
		if slices.Contains(want, p) || !lt.locking.AutoLocks(p) || (rules != nil && rules.Ignored(p, false)) {
			continue
		}
		want = append(want, p)
	}
	if len(want) == 0 {
		return
	}
	// Mine already, or someone else's (told, not asked for).
	ls := cachedLocks(root)
	var ask []string
	var others []remote.LockHolder
	for _, p := range want {
		holder := ""
		for _, l := range ls {
			if remote.Covers(l.Path, p) {
				holder = l.MemberID
				if holder == lt.team.MemberID {
					break
				}
			}
		}
		switch {
		case holder == lt.team.MemberID && holder != "":
		case holder != "":
			others = append(others, remote.LockHolder{Path: p, MemberID: holder})
		default:
			ask = append(ask, p)
		}
	}
	if len(ask) > 0 {
		res, err := cloud.SetLocks(lt.address, lt.r.Config.ProjectID, lt.workspace, ask, nil)
		if err != nil {
			if remote.Transient(err) {
				// Out of reach: they wait (said in the Files tab), tried again next time.
				keep := slices.Clone(waiting)
				for _, p := range ask {
					if !slices.Contains(keep, p) {
						keep = append(keep, p)
					}
				}
				if err := lt.r.SetWaitingLocks(keep); err != nil {
					log.Printf("locks waiting %s: %v", root, err)
				}
				a.emitLocks(root)
			}
			return
		}
		others = append(others, res.Refused...)
		lt.read() // (what is held now, for the next change)
	}
	// Nothing waits any more: locked, or someone else holds it.
	if len(waiting) > 0 {
		lt.r.SetWaitingLocks(nil)
	}
	for _, h := range others {
		a.tellHeld(lt, h, slices.Contains(waiting, h.Path))
	}
	if len(ask) > 0 || len(waiting) > 0 {
		a.emitLocks(root)
	}
}

// tellHeld says at once that a change can't be shared: someone else holds
// the file (meanwhile: they took it while the team couldn't be reached).
func (a *App) tellHeld(lt *lockTarget, h remote.LockHolder, meanwhile bool) {
	key := lt.r.Root + "\x00" + h.Path + "\x00" + h.MemberID
	told.Lock()
	again := told.m[key]
	told.m[key] = true
	told.Unlock()
	if again {
		return
	}
	who := a.memberNames(lt.r)[h.MemberID]
	if who == "" {
		who = "Someone"
	}
	name := filepath.Base(filepath.FromSlash(h.Path))
	if a.emit != nil {
		a.emit("lock-held", HeldEvent{Root: lt.r.Root, Path: h.Path, File: name, Name: who, Meanwhile: meanwhile})
	}
	if a.notify != nil && a.hidden.Load() {
		a.notify(fmt.Sprintf("%s is editing %s", who, name), "Your change can't be shared until it's unlocked.")
	}
}

// HeldEvent: you changed a file someone else holds.
type HeldEvent struct {
	Root string `json:"root"`
	Path string `json:"path"`
	File string `json:"file"` // its name
	Name string `json:"name"` // who holds it
	// Meanwhile: they took it while your change waited to be locked.
	Meanwhile bool `json:"meanwhile"`
}

// dropWaiting: changes discarded don't wait to be locked any more. Done
// with the discard, not after (unlockDiscarded runs in the background: a
// change made again meanwhile must keep waiting).
func (a *App) dropWaiting(r *project.Repo, paths []string) {
	lockQueue.Lock()
	defer lockQueue.Unlock()
	if w := r.WaitingLocks(); len(w) > 0 {
		r.SetWaitingLocks(slices.DeleteFunc(w, func(p string) bool { return slices.Contains(paths, p) }))
	}
}

// unlockDiscarded frees your own file locks on paths whose changes were
// discarded (folder locks stay: they were taken by hand).
func (a *App) unlockDiscarded(root string, paths []string) {
	lt, err := lockTargetOf(root)
	if err != nil || lt == nil || len(paths) == 0 {
		return
	}
	ls, _ := lt.read()
	var free []string
	for _, l := range ls {
		if !l.Prefix && l.MemberID == lt.team.MemberID && slices.Contains(paths, l.Path) {
			free = append(free, l.Path)
		}
	}
	if len(free) > 0 {
		cloud.SetLocks(lt.address, lt.r.Config.ProjectID, lt.workspace, nil, free)
	}
	a.locksChanged(root)
}

// lockedResult is a share refused because someone else holds paths it
// changes: the version stays committed here, shared once they're freed.
func lockedResult(r *project.Repo, err error) (*Result, bool) {
	var l *remote.ErrLocked
	if !errors.As(err, &l) {
		return nil, false
	}
	names := cachedMemberNames(r)
	if names == nil {
		names = r.MemberNames()
	}
	out := syncResult(nil)
	out.Action = "locked"
	out.Locks = []HeldLock{}
	for _, h := range l.Locks {
		out.Locks = append(out.Locks, HeldLock{Path: h.Path, MemberID: h.MemberID, Name: names[h.MemberID]})
	}
	return out, true
}

// --- the team's switch ------------------------------------------------------------

// TeamLocks is a hosted team's file locks settings, for its settings page.
type TeamLocks struct {
	// Available: this build and team can have them (hosted, Nightly).
	Available bool               `json:"available"`
	On        bool               `json:"on"`
	Kinds     []string           `json:"kinds"`
	Admin     bool               `json:"admin"`
	All       []profile.LockKind `json:"all"` // the kinds to pick from
}

// TeamLocks reads a team's lock settings.
func (a *App) TeamLocks(teamID string) (*TeamLocks, error) {
	out := &TeamLocks{Kinds: []string{}, All: profile.LockKinds}
	if !remote.FileLocks {
		return out, nil
	}
	store, err := teams.Load()
	if err != nil {
		return nil, err
	}
	t := store.Find(teamID)
	if t == nil {
		return nil, errors.New("unknown team")
	}
	if _, ok := cloud.Hosted(*t); !ok {
		return out, nil
	}
	s, err := teamLockSettings(t, true)
	if err != nil {
		return nil, err
	}
	out.Available, out.On, out.Kinds, out.Admin = true, s.On, s.Kinds, isAdmin(myRole(t))
	if !s.On && len(s.Kinds) == 0 {
		out.Kinds = profile.DefaultLockKinds() // (what turning it on starts with)
	}
	return out, nil
}

// SetTeamLocks turns file locking on or off for the team and picks the
// kinds of file that lock by themselves (admins). On, R3Vs without locks
// must update before sharing.
func (a *App) SetTeamLocks(teamID string, on bool, kinds []string) error {
	store, err := teams.Load()
	if err != nil {
		return err
	}
	t := store.Find(teamID)
	if t == nil {
		return errors.New("unknown team")
	}
	if _, ok := cloud.Hosted(*t); !ok {
		return errors.New("file locks are for R3V-Cloud teams")
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if kinds == nil {
		kinds = []string{}
	}
	if err := remote.SetLocks(b, remote.LockSettings{On: on, Kinds: kinds}); err != nil {
		return err
	}
	forgetTeamLocks(t.Remote.URL)
	teams.Update(func(s *teams.Store) error {
		if tm := s.Find(teamID); tm != nil {
			tm.LocksAsked = true // (decided: not offered again)
		}
		return nil
	})
	for key, root := range store.Projects {
		if strings.HasPrefix(key, t.ID+"/") {
			a.locksChanged(root)
		}
	}
	return nil
}

// DismissLocksOffer: the team's admin said no to turning locks on (not
// offered again).
func (a *App) DismissLocksOffer(teamID string) error {
	_, err := teams.Update(func(s *teams.Store) error {
		if tm := s.Find(teamID); tm != nil {
			tm.LocksAsked = true
		}
		return nil
	})
	return err
}

// locksOffer says whether to offer turning locks on for a project's team:
// a hosted team without them, never asked, you its admin, and a project of
// a kind that benefits (Unreal, Unity, Godot, Blender).
func locksOffer(r *project.Repo) bool {
	if !remote.FileLocks || r.Config.Remote == nil {
		return false
	}
	t, err := r.Team()
	if err != nil || t.LocksAsked || t.NoAccess {
		return false
	}
	if _, ok := cloud.Hosted(*t); !ok {
		return false
	}
	if s, err := teamLockSettings(t, false); err != nil || s.On {
		return false
	}
	return isAdmin(myRole(t)) && benefits(r)
}

// benefits: a project with files that can't be merged and are worked on
// for long (a game engine's, or Blender's).
func benefits(r *project.Repo) bool {
	rules, _ := r.Profile()
	if rules != nil {
		for _, ap := range rules.Applied() {
			switch ap.Preset {
			case "unreal", "unity", "godot":
				return true
			}
		}
	}
	found := false
	filepath.WalkDir(r.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if d.IsDir() && (d.Name() == ".r3v" || strings.Count(filepath.ToSlash(strings.TrimPrefix(p, r.Root)), "/") > 3) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".blend") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
