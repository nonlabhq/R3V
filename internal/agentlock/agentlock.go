// Package agentlock keeps the agent locks of one computer: which workspace
// of a project (an AI agent's copy, or the person's) is changing a file
// that can't be merged, so that two agents don't both change it (see
// docs/design/agent-workspaces.md, "Locks").
//
// The list lives beside the store every workspace shares
// (Repo.CommonDir()/agent-locks), one file per lock, each written whole
// (a temp file renamed over it): a reader sees a lock or none, never half
// of one. Taking is done under a file lock of the folder, so two agents
// asking at once take turns; reading needs none.
package agentlock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/flock"
	"github.com/nonlabhq/r3v/internal/store"
)

// DirName is the folder of the list, in the project's common .r3v folder.
const DirName = "agent-locks"

// StaleAfter: a lock whose workspace hasn't been heard of for this long is
// stale (its agent was killed, or stopped long ago). Every hook call is a
// heartbeat, and agents call tools every few seconds while working, so a
// quarter of an hour without one means no one is there; still short
// enough that a lock left over doesn't stop the others for long.
const StaleAfter = 15 * time.Minute

// Kinds of lock.
const (
	KindLock  = "lock"  // refuses other workspaces
	KindClaim = "claim" // only warns them: "I'm on src/renderer/"
)

// Lock is a path a workspace holds.
type Lock struct {
	// Path is slash separated, relative to the project folder; a folder's
	// ends with "/" and covers what's under it.
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Workspace string `json:"workspace"`
	Branch    string `json:"branch"`
	Session   string `json:"session,omitempty"`
	// Actor: the agent tool that took it (R3V_ACTOR), "" for the person.
	Actor string    `json:"actor,omitempty"`
	Since time.Time `json:"since"`
	// Heartbeat: when its workspace was last heard of (filled when read).
	Heartbeat time.Time `json:"heartbeat"`
	// Stale: no heartbeat for StaleAfter (filled when read).
	Stale bool `json:"stale"`
	// Person: a file the person changed and hasn't committed (not stored:
	// worked out from their copy, see project).
	Person bool `json:"person,omitempty"`
	// Reason and TakenFrom: a lock taken over from a holder that wasn't
	// stale (TakeOver): why, and who held it, so the list says so.
	Reason    string `json:"reason,omitempty"`
	TakenFrom string `json:"taken_from,omitempty"`
}

// Holder is who asks: a workspace (this copy), its branch, the agent's
// session.
type Holder struct {
	Workspace string
	Branch    string
	Session   string
	Actor     string
}

// Refused is a lock asked for that someone else holds: nothing was taken.
type Refused struct {
	Locks []Lock // the holders, one per path refused
}

func (e *Refused) Error() string {
	var b strings.Builder
	for i, l := range e.Locks {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(l.Path + " is held by " + l.Who())
	}
	return b.String()
}

// Who names a lock's holder for people and agents.
func (l Lock) Who() string {
	if l.Person {
		return "the person (changed, not committed)"
	}
	who := "workspace " + short(l.Workspace)
	if l.Actor != "" {
		who = l.Actor + " in " + who
	}
	if l.Branch != "" {
		who += " on branch " + l.Branch
	}
	return who
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Clean makes p a lock path: slash separated, no leading "./" or "/", a
// folder's ending in "/". "" for a path that leaves the project.
func Clean(p string, folder bool) string {
	p = strings.ReplaceAll(p, `\`, "/")
	dir := folder || strings.HasSuffix(p, "/")
	p = strings.Trim(filepathClean(p), "/")
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	if dir {
		p += "/"
	}
	return p
}

func filepathClean(p string) string { return filepath.ToSlash(filepath.Clean(filepath.FromSlash(p))) }

// Covers says whether a lock on lockPath covers p (either may be a folder):
// they overlap. Case is ignored: the computer's drive is (Windows, macOS).
func Covers(lockPath, p string) bool {
	a, b := strings.ToLower(lockPath), strings.ToLower(p)
	switch {
	case a == b:
		return true
	case strings.HasSuffix(a, "/") && strings.HasPrefix(b, a):
		return true
	case strings.HasSuffix(b, "/") && strings.HasPrefix(a, b):
		return true
	}
	return false
}

// coversDown: folder holds p (p is in it), not the other way round.
func coversDown(folder, p string) bool {
	return strings.HasSuffix(folder, "/") && strings.HasPrefix(strings.ToLower(p), strings.ToLower(folder))
}

// List is the agent locks of one project on this computer.
type List struct {
	dir string
	now func() time.Time
}

// Open is the list kept in commonDir (made when first written).
func Open(commonDir string) *List {
	return &List{dir: filepath.Join(commonDir, DirName), now: time.Now}
}

// SetClock replaces the clock (tests).
func (l *List) SetClock(now func() time.Time) { l.now = now }

// Dir is the list's folder.
func (l *List) Dir() string { return l.dir }

// file names a lock's file: one per locked path (whoever holds it), one
// per claim and workspace (claims don't exclude each other).
func (l *List) file(lk Lock) string {
	key := strings.ToLower(lk.Path)
	prefix := "l-"
	if lk.Kind == KindClaim {
		key += "\x00" + lk.Workspace
		prefix = "c-"
	}
	s := sha256.Sum256([]byte(key))
	return filepath.Join(l.dir, prefix+hex.EncodeToString(s[:12])+".json")
}

func (l *List) heartbeatFile(workspace string) string {
	s := sha256.Sum256([]byte(workspace))
	return filepath.Join(l.dir, "hb-"+hex.EncodeToString(s[:12]))
}

// Heartbeat says the workspace is there (its locks aren't stale): the
// time of a small file, cheap enough for every tool call.
func (l *List) Heartbeat(workspace string) error {
	p := l.heartbeatFile(workspace)
	now := l.now()
	if err := os.Chtimes(p, now, now); err == nil {
		return nil
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		return err
	}
	return os.Chtimes(p, now, now)
}

func (l *List) lastHeard(workspace string, cache map[string]time.Time) time.Time {
	if t, ok := cache[workspace]; ok {
		return t
	}
	var t time.Time
	if fi, err := os.Stat(l.heartbeatFile(workspace)); err == nil {
		t = fi.ModTime().UTC()
	}
	cache[workspace] = t
	return t
}

// All reads every lock, the stale marked; sorted by path.
func (l *List) All() ([]Lock, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	heard := map[string]time.Time{}
	now := l.now()
	var out []Lock
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".json") || !(strings.HasPrefix(n, "l-") || strings.HasPrefix(n, "c-")) {
			continue // a heartbeat, or a temp file being written
		}
		data, err := os.ReadFile(filepath.Join(l.dir, n))
		if err != nil {
			continue // taken back meanwhile
		}
		var lk Lock
		if json.Unmarshal(data, &lk) != nil || lk.Path == "" {
			continue
		}
		lk.Heartbeat = l.lastHeard(lk.Workspace, heard)
		if lk.Heartbeat.Before(lk.Since) {
			lk.Heartbeat = lk.Since
		}
		lk.Stale = now.Sub(lk.Heartbeat) > StaleAfter
		out = append(out, lk)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Workspace < out[j].Workspace
	})
	return out, nil
}

// Over is the locks (of any holder) that overlap p.
func Over(locks []Lock, p string) []Lock {
	var out []Lock
	for _, lk := range locks {
		if Covers(lk.Path, p) {
			out = append(out, lk)
		}
	}
	return out
}

// guard takes turns with other takers on this computer.
func (l *List) guard() (func(), error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return nil, err
	}
	return flock.Lock(filepath.Join(l.dir, "lock"), 5*time.Second)
}

// Result is what Take did.
type Result struct {
	Taken []Lock // the locks now held (new or already ours)
	// Others: other workspaces' claims (and, for a claim, their locks)
	// over the paths: told of, never in the way.
	Others []Lock
	// TakenOver: stale locks of others that were replaced.
	TakenOver []Lock
}

// Take takes paths for h, all or none: a lock another workspace holds
// (not stale) over any of them refuses the whole set (*Refused) and
// leaves the list as it was. extra are holders the caller found that the
// list doesn't keep (the person's changes): they refuse as well. A claim
// never refuses. A stale lock in the way is taken over.
func (l *List) Take(h Holder, paths []string, kind string, extra []Lock) (Result, error) {
	return l.take(h, paths, kind, extra, "")
}

// TakeOver locks paths for h as Take does, taking over others' locks in
// the way, stale or not: for a holder that is gone before its locks went
// stale (its session ended, a copy that won't commit them). The reason is
// kept in the new locks with who held them. extra (the person's changes)
// still refuses: people always win.
func (l *List) TakeOver(h Holder, paths []string, reason string, extra []Lock) (Result, error) {
	if strings.TrimSpace(reason) == "" {
		return Result{}, errors.New("a takeover needs a reason")
	}
	return l.take(h, paths, KindLock, extra, strings.TrimSpace(reason))
}

func (l *List) take(h Holder, paths []string, kind string, extra []Lock, reason string) (Result, error) {
	var res Result
	if kind != KindLock && kind != KindClaim {
		return res, fmt.Errorf("unknown kind of lock %q", kind)
	}
	if h.Workspace == "" {
		return res, errors.New("no workspace to hold the lock")
	}
	release, err := l.guard()
	if err != nil {
		return res, err
	}
	defer release()
	all, err := l.All()
	if err != nil {
		return res, err
	}
	var refused []Lock
	var write []Lock
	var stale []Lock
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		var mine *Lock
		for _, o := range Over(all, p) {
			switch {
			case o.Workspace == h.Workspace:
				if o.Kind == kind && mine == nil && (strings.EqualFold(o.Path, p) || coversDown(o.Path, p)) {
					o := o
					mine = &o
				}
			case o.Kind == KindClaim || kind == KindClaim:
				res.Others = append(res.Others, o)
			case o.Stale || reason != "":
				stale = append(stale, o)
			default:
				refused = append(refused, o)
			}
		}
		if kind == KindLock {
			for _, o := range Over(extra, p) {
				refused = append(refused, o)
			}
		}
		if mine != nil {
			res.Taken = append(res.Taken, *mine)
			continue
		}
		write = append(write, Lock{Path: p, Kind: kind, Workspace: h.Workspace, Branch: h.Branch,
			Session: h.Session, Actor: h.Actor, Since: l.now().UTC()})
	}
	if reason != "" {
		for i := range write {
			var from []string
			for _, o := range dedupe(stale) {
				if Covers(o.Path, write[i].Path) {
					from = append(from, o.Who())
				}
			}
			if len(from) > 0 {
				write[i].Reason, write[i].TakenFrom = reason, strings.Join(from, ", ")
			}
		}
	}
	if len(refused) > 0 {
		return Result{Others: res.Others}, &Refused{Locks: dedupe(refused)}
	}
	// Stale locks in the way go first: a lock over them would otherwise be
	// read as two holders.
	for _, o := range dedupe(stale) {
		if err := store.Remove(l.file(o)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{Others: res.Others}, err
		}
		res.TakenOver = append(res.TakenOver, o)
	}
	var made []Lock
	for _, lk := range write {
		if err := l.write(lk); err != nil {
			for _, m := range made { // all or none
				store.Remove(l.file(m))
			}
			return Result{Others: res.Others}, err
		}
		made = append(made, lk)
	}
	res.Taken = append(res.Taken, made...)
	l.Heartbeat(h.Workspace)
	l.sweep()
	return res, nil
}

func dedupe(locks []Lock) []Lock {
	var out []Lock
	seen := map[string]bool{}
	for _, lk := range locks {
		k := strings.ToLower(lk.Path) + "\x00" + lk.Workspace + "\x00" + lk.Kind
		if !seen[k] {
			seen[k] = true
			out = append(out, lk)
		}
	}
	return out
}

// write writes a lock whole: a temp file renamed over its name.
func (l *List) write(lk Lock) error {
	lk.Heartbeat, lk.Stale, lk.Person = time.Time{}, false, false
	data, err := json.MarshalIndent(lk, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteAtomic(l.file(lk), strings.NewReader(string(data)+"\n"))
}

// sweep removes temp files left by a writer that stopped half-way.
func (l *List) sweep() {
	entries, _ := os.ReadDir(l.dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".r3v-") {
			continue
		}
		if fi, err := e.Info(); err == nil && l.now().Sub(fi.ModTime()) > time.Minute {
			os.Remove(filepath.Join(l.dir, e.Name()))
		}
	}
}

// Untold is the keys among keys not told to workspace before, now noted
// as told: a claim is said once to each agent, not on every tool call.
func (l *List) Untold(workspace string, keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	s := sha256.Sum256([]byte(workspace))
	path := filepath.Join(l.dir, "told-"+hex.EncodeToString(s[:12])+".json")
	told := map[string]bool{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &told)
	}
	var out []string
	for _, k := range keys {
		if !told[k] {
			told[k] = true
			out = append(out, k)
		}
	}
	if len(out) > 0 {
		if data, err := json.Marshal(told); err == nil {
			store.WriteAtomic(path, strings.NewReader(string(data)))
		}
	}
	return out
}

// remove takes back the locks for which drop says so; returns them.
func (l *List) remove(drop func(Lock) bool) ([]Lock, error) {
	release, err := l.guard()
	if err != nil {
		return nil, err
	}
	defer release()
	all, err := l.All()
	if err != nil {
		return nil, err
	}
	var out []Lock
	for _, lk := range all {
		if !drop(lk) {
			continue
		}
		if err := store.Remove(l.file(lk)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return out, err
		}
		out = append(out, lk)
	}
	return out, nil
}

// Unlock takes back the workspace's own locks and claims on paths (as
// asked: a folder's lock by its folder).
func (l *List) Unlock(workspace string, paths []string) ([]Lock, error) {
	return l.remove(func(lk Lock) bool {
		if lk.Workspace != workspace {
			return false
		}
		for _, p := range paths {
			if strings.EqualFold(lk.Path, p) {
				return true
			}
		}
		return false
	})
}

// Break takes back whoever's locks and claims are on path (for the
// person: people always win).
func (l *List) Break(path string) ([]Lock, error) {
	return l.remove(func(lk Lock) bool { return strings.EqualFold(lk.Path, path) })
}

// ReleaseBranch takes back every lock and claim held for branch: its work
// was merged into its base, where conflicts were settled.
func (l *List) ReleaseBranch(branch string) ([]Lock, error) {
	return l.remove(func(lk Lock) bool { return lk.Branch == branch })
}

// ReleaseWorkspace takes back every lock and claim of a workspace (one
// removed).
func (l *List) ReleaseWorkspace(workspace string) ([]Lock, error) {
	return l.remove(func(lk Lock) bool { return lk.Workspace == workspace })
}
