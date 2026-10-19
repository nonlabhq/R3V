package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonlabhq/r3v/internal/agentlock"
	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/store"
)

// Agent locks (docs/design/agent-workspaces.md, "Locks"): between the
// workspaces of a project on this computer, a file that can't be merged is
// changed by one at a time. An agent takes the lock when it first touches
// such a file (the hook) or commits it; the person's uncommitted changes
// count as theirs. The list itself is package agentlock.

// AgentLocks: agent locks are in this build (the Nightly channel while
// they're new; agentlocks_nightly.go). Off, nothing is checked at commit.
var AgentLocks = false

// agentTool is the agent tool this repo works for (Repo.Actor: the CLI's,
// from R3V_ACTOR or the tool's own mark), "" for a person. The app never
// sets it: whoever opened it, the app is a person's.
func (r *Repo) agentTool() string {
	if r.Actor != nil {
		return r.Actor.Tool
	}
	return ""
}

// ErrLockedByAgent is a commit refused because another workspace (an
// agent's, or the person's changes) holds files it changes.
type ErrLockedByAgent struct {
	Locks []agentlock.Lock
}

func (e *ErrLockedByAgent) Error() string {
	return "files held by another workspace: " + (&agentlock.Refused{Locks: e.Locks}).Error()
}

// AgentLockList is the project's agent lock list on this computer, beside
// the store every workspace shares.
func (r *Repo) AgentLockList() *agentlock.List { return agentlock.Open(r.CommonDir()) }

// teamLocksFile kept the team's lock settings before workflow settings
// (workflow.go): read when there's nothing newer, removed once there is.
const teamLocksFile = "team-locks.json"

// RefreshAgentTeamLocks reads the team's settings into this computer's copy
// (RefreshWorkflow).
func (r *Repo) RefreshAgentTeamLocks() { r.RefreshWorkflow() }

// AgentLocking is what locks between workspaces (unless the team or the
// project turned that off): the project's kinds and paths when it set its
// own file locking, else the team's kinds when it locks files, else every
// kind that can't be merged.
func (r *Repo) AgentLocking() profile.Locking {
	if r.agentLocking != nil {
		return *r.agentLocking
	}
	w := r.Workflow()
	var l profile.Locking
	if w.AgentLocksOn() {
		team := profile.TeamLocks{On: true, Kinds: profile.AgentLockKinds()}
		if w.Locks.On {
			team.Kinds = w.Locks.Kinds
		}
		var f profile.FileLocks
		if p := w.Project.Locks; p != nil {
			if p.Kinds != nil {
				team.Kinds = p.Kinds
			}
			f = profile.FileLocks{AutoOff: p.AutoOff, Add: p.Add, Remove: p.Remove}
		}
		l = profile.ResolveLocking(team, f)
	} else {
		l = profile.ResolveLocking(profile.TeamLocks{}, profile.FileLocks{})
	}
	r.agentLocking = &l
	return l
}

// AgentLocked says whether rel (slash separated) is a file workspaces take
// turns on: tracked, of a locking kind, and not code (code merges hunk by
// hunk, so it never locks).
func (r *Repo) AgentLocked(rel string) bool {
	if strings.HasSuffix(rel, "/") || !r.AgentLocking().AutoLocks(rel) {
		return false
	}
	if rules := r.rules(); rules != nil {
		if rules.Ignored(rel, false) || rules.Handler(rel).Merge == "text" {
			return false
		}
	}
	return true
}

// AgentHolder is this copy as a lock's holder.
func (r *Repo) AgentHolder(session string) agentlock.Holder {
	return agentlock.Holder{Workspace: r.workspaceID(), Branch: r.BranchName(), Session: session, Actor: r.agentTool()}
}

// RelPath turns a path on disk (absolute, or relative to the working
// folder) into one relative to the project ("" when outside it).
func (r *Repo) RelPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(r.Root, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == metaDir || strings.HasPrefix(rel, metaDir+"/") {
		return ""
	}
	return rel
}

// TakeAgentLocks takes paths (relative, slash separated; a folder's ending
// in "/") for this workspace, all or none (*agentlock.Refused). An agent is
// also refused files the person changed and hasn't committed.
func (r *Repo) TakeAgentLocks(paths []string, kind, session string) (agentlock.Result, error) {
	var person []agentlock.Lock
	if kind == agentlock.KindLock && r.agentTool() != "" {
		if m := r.personCopy(); m != nil {
			person = r.personHeld(m, paths)
		}
	}
	return r.AgentLockList().Take(r.AgentHolder(session), paths, kind, person)
}

// TakeOverAgentLocks locks paths for this workspace, taking over the locks
// of others in the way (agentlock.List.TakeOver): their holder is gone.
// The person's uncommitted changes still refuse an agent.
func (r *Repo) TakeOverAgentLocks(paths []string, reason, session string) (agentlock.Result, error) {
	var person []agentlock.Lock
	if r.agentTool() != "" {
		if m := r.personCopy(); m != nil {
			person = r.personHeld(m, paths)
		}
	}
	return r.AgentLockList().TakeOver(r.AgentHolder(session), paths, reason, person)
}

// UnlockAgent takes back this workspace's own locks and claims on paths.
func (r *Repo) UnlockAgent(paths []string) ([]agentlock.Lock, error) {
	return r.AgentLockList().Unlock(r.workspaceID(), paths)
}

// BreakAgentLock takes back anyone's locks on path: the person's call.
func (r *Repo) BreakAgentLock(path string) ([]agentlock.Lock, error) {
	return r.AgentLockList().Break(path)
}

// AllAgentLocks is every agent lock of the project on this computer, with
// the files the person holds (changed in their copy, not committed).
func (r *Repo) AllAgentLocks() ([]agentlock.Lock, error) {
	locks, err := r.AgentLockList().All()
	if err != nil {
		return nil, err
	}
	m := r.personCopy()
	if m == nil && r.agentTool() == "" {
		m = r // the person asks, in their own copy
	}
	if m == nil {
		return locks, nil
	}
	ix := m.loadIndex()
	files, err := m.workingFiles(ix)
	if err != nil {
		return locks, nil // their copy can't be read now: the list is all we know
	}
	changes, err := m.statusOf(files)
	if err != nil {
		return locks, nil
	}
	for _, c := range changes {
		for _, p := range []string{c.Path, c.From} {
			if p != "" && c.Status != "untracked" && r.AgentLocked(p) {
				locks = append(locks, m.personLock(p))
			}
		}
	}
	return locks, nil
}

// mainCopy is the folder of the project's main copy (whose .r3v is the
// common one); tests point it elsewhere.
var mainCopy = func(r *Repo) string { return filepath.Dir(r.CommonDir()) }

// personCopy is the copy the person works in: the main copy, whose .r3v is
// the common one. Nil when this is it and an agent works here (nothing to
// tell apart), or it can't be read.
func (r *Repo) personCopy() *Repo {
	main := mainCopy(r)
	if strings.EqualFold(filepath.Clean(main), filepath.Clean(r.Root)) {
		if r.agentTool() != "" {
			return nil
		}
		return r
	}
	m, err := Open(main)
	if err != nil || !strings.EqualFold(m.Root, filepath.Clean(main)) {
		return nil
	}
	return m
}

func (m *Repo) personLock(rel string) agentlock.Lock {
	return agentlock.Lock{Path: rel, Kind: agentlock.KindLock, Workspace: m.Config.WorkspaceID,
		Branch: m.BranchName(), Person: true}
}

// personHeld is the files among paths (a folder: those in it) the person
// changed in m and hasn't committed. Cheap for a file (one look, a hash
// only when its time changed): the hook asks on every first touch.
func (r *Repo) personHeld(m *Repo, paths []string) []agentlock.Lock {
	var head map[string]FileEntry
	if id := m.Head(); id != "" {
		if mf, err := m.Load(id); err == nil {
			head = mf.FileMap()
		}
	}
	var ix *index
	var out []agentlock.Lock
	for _, p := range paths {
		var files []string
		if strings.HasSuffix(p, "/") {
			files = m.filesUnder(p, head)
		} else {
			files = []string{p}
		}
		for _, f := range files {
			if !r.AgentLocked(f) {
				continue
			}
			if ix == nil {
				ix = m.loadIndex()
			}
			if m.changedHere(f, head, ix) {
				out = append(out, m.personLock(f))
			}
		}
	}
	return out
}

// filesUnder is the files in folder (slash, ending "/"): the version's and
// those on disk.
func (m *Repo) filesUnder(folder string, head map[string]FileEntry) []string {
	seen := map[string]bool{}
	var out []string
	for p := range head {
		if strings.HasPrefix(strings.ToLower(p), strings.ToLower(folder)) {
			seen[p] = true
			out = append(out, p)
		}
	}
	filepath.WalkDir(m.Abs(folder), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if rel := m.RelPath(path); rel != "" && !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
		return nil
	})
	return out
}

// changedHere says whether rel differs from the version m is on.
func (m *Repo) changedHere(rel string, head map[string]FileEntry, ix *index) bool {
	abs := m.Abs(rel)
	fi, err := os.Stat(abs)
	e, inHead := head[rel]
	switch {
	case err != nil:
		return inHead // deleted
	case !inHead:
		return true // added
	case fi.Size() != e.Size:
		return true
	}
	if c, ok := ix.entries[rel]; ok && c.Size == fi.Size() && c.Mtime == fi.ModTime().UnixNano() {
		return c.Hash != e.Hash
	}
	h, _, err := store.HashFile(abs)
	return err == nil && h != e.Hash
}

// checkAgentLocks refuses an agent's commit of files another workspace
// holds; the files it may commit become its own (as a first touch does).
// A person's commit is never refused: people always win.
func (r *Repo) checkAgentLocks(m *Manifest) error {
	if !AgentLocks || r.agentTool() == "" {
		return nil
	}
	var prev map[string]FileEntry
	if len(m.Parents) > 0 {
		pm, err := r.Load(m.Parents[0])
		if err != nil {
			return err
		}
		prev = pm.FileMap()
	}
	var changed []string
	now := m.FileMap()
	for p, f := range now {
		if old, ok := prev[p]; (!ok || old.Hash != f.Hash) && r.AgentLocked(p) {
			changed = append(changed, p)
		}
	}
	for p := range prev {
		if _, ok := now[p]; !ok && r.AgentLocked(p) {
			changed = append(changed, p)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	_, err := r.TakeAgentLocks(changed, agentlock.KindLock, "")
	var refused *agentlock.Refused
	if errors.As(err, &refused) {
		return &ErrLockedByAgent{Locks: refused.Locks}
	}
	return err
}

// releaseMergedLocks frees the locks held for a branch merged here: its
// conflicts were settled by the merge. Not a base branch's: a workspace
// merging its base in is catching up, not landing, and a base branch's
// locks guard uncommitted work only (releaseCommittedLocks).
func (r *Repo) releaseMergedLocks(branch string) {
	if !AgentLocks || branch == "" || r.isBaseBranch(branch) {
		return
	}
	r.AgentLockList().ReleaseBranch(branch)
}

// isBaseBranch says whether workspaces merge into branch: main, or the
// branch any workspace of the project here was made from. A lock held on
// a base branch is never released by a merge (nothing merges it into
// anything), so a commit there releases it.
func (r *Repo) isBaseBranch(branch string) bool {
	if branch == "main" {
		return true
	}
	list, _ := r.LocalWorkspaces()
	for _, e := range list {
		if e.Base == branch {
			return true
		}
	}
	return false
}

// releaseCommittedLocks frees, after a commit on a base branch, this
// copy's locks on files now as committed: a lock there protects
// uncommitted work, and once it's a version on the branch the others merge
// from, there's nothing left to protect (a workspace changing it later
// merges it in first, or decides at the merge). Folder locks stay until
// unlocked. A stop before this leaves the locks, freed by the next commit
// here or gone stale.
func (r *Repo) releaseCommittedLocks(m *Manifest, ix *index) {
	branch := r.BranchName()
	if !AgentLocks || !r.isBaseBranch(branch) {
		return
	}
	list := r.AgentLockList()
	all, err := list.All()
	if err != nil {
		return
	}
	me := r.workspaceID()
	files := m.FileMap()
	var done []string
	for _, lk := range all {
		if lk.Workspace != me || lk.Branch != branch || lk.Kind != agentlock.KindLock || strings.HasSuffix(lk.Path, "/") {
			continue
		}
		if !r.changedHere(lk.Path, files, ix) {
			done = append(done, lk.Path)
		}
	}
	if len(done) > 0 {
		list.Unlock(me, done)
	}
}
