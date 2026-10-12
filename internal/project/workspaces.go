package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/flock"
	"github.com/nonlabhq/r3v/internal/jsonx"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/store"
)

// Workspaces (docs/design/agent-workspaces.md): more copies of a project on
// this computer, each on a team branch of its own, so several agents work
// at once. A workspace is another folder with its own .r3v for what is per
// copy; what every copy shares is in the main copy's .r3v (CommonDir), as
// with git worktree:
//
//	shared (main copy's .r3v)        per copy (its own .r3v)
//	objects/         the store       config.json   its branch, workspace id, tip
//	snapshots/       the versions                  (team, project, name: the main copy's)
//	trees/           their folders   HEAD          the version it is on
//	chunks/          chunk lists     index.bin     its files as last hashed
//	remote-objects   team-only list  switching     a switch under way
//	workspaces.json  the registry    parked/       parked sets (their versions: shared)
//	agent-locks/     agent locks     kept-work     work an update kept (versions: shared)
//	                                 preupload/, preuploaded   files going up early
//	                                 external/     external samples, for its sets
//	                                 lock, locks-waiting.json, team-seen.json,
//	                                 workspace-noted.json
//	                                 workspace.json (a workspace's only): the main copy
//
// Content-addressed things are shared (written whole, never changed); what
// says where a copy is, or what it has under way, is its own. Local
// cleanup in any copy keeps what every copy needs (otherCopies).

// AgentWorkspaces: making workspaces (the Nightly channel while it's new;
// workspaces_nightly.go). Projects with workspaces work either way.
var AgentWorkspaces = false

var (
	// ErrWorkspacesNotInBuild: this build makes no workspaces.
	ErrWorkspacesNotInBuild = errors.New("workspaces are on the Nightly channel")
	// ErrInWorkspace: something only the main copy of a project does.
	ErrInWorkspace = errors.New("this is a workspace of a project: do this in the project's main copy")
	// ErrWorkspaceOrphaned: the main copy a workspace uses is gone.
	ErrWorkspaceOrphaned = errors.New("this workspace's project is gone")
	// ErrWorkspaceNotDone: removing a workspace with work not merged.
	ErrWorkspaceNotDone = errors.New("the workspace has work not merged")
)

const (
	workspaceLinkFile = "workspace.json"  // in a workspace's .r3v
	workspacesFile    = "workspaces.json" // in the common dir
)

// workspaceLink is a workspace's own record: where the shared data is, and
// what it was made from (an add stopped half-way goes on from it).
type workspaceLink struct {
	Common string `json:"common"` // the main copy's .r3v
	Base   string `json:"base"`   // the branch it was made from
	From   string `json:"from"`   // the version it started at
}

// readLink reads a .r3v's workspace record (nil for a main copy).
func readLink(dir string) (*workspaceLink, error) {
	data, err := os.ReadFile(filepath.Join(dir, workspaceLinkFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var l workspaceLink
	if err := json.Unmarshal(data, &l); err != nil || l.Common == "" {
		return nil, fmt.Errorf("%s: damaged workspace record", filepath.Join(dir, workspaceLinkFile))
	}
	return &l, nil
}

// WorkspaceEntry is a workspace in the project's registry.
type WorkspaceEntry struct {
	Folder  string    `json:"folder"` // absolute
	Branch  string    `json:"branch"` // its branch's key
	Base    string    `json:"base"`   // the branch it was made from (merged into)
	Created time.Time `json:"created"`
	// Extra: fields a newer R3V wrote, kept when this one rewrites the list.
	Extra jsonx.Extra `json:"-"`
}

func (e *WorkspaceEntry) UnmarshalJSON(b []byte) error {
	type plain WorkspaceEntry
	return jsonx.Decode(b, (*plain)(e), &e.Extra)
}

func (e WorkspaceEntry) MarshalJSON() ([]byte, error) {
	type plain WorkspaceEntry
	return jsonx.Encode(plain(e), e.Extra)
}

type workspaceList struct {
	Workspaces []WorkspaceEntry `json:"workspaces"`
}

// IsWorkspace: this copy is a workspace (not the project's main copy).
func (r *Repo) IsWorkspace() bool { return r.common != "" }

// MainRoot is the folder of the project's main copy.
func (r *Repo) MainRoot() string { return filepath.Dir(r.CommonDir()) }

// LocalWorkspaces lists the project's workspaces on this computer, as
// registered (Workspaces is the team's other copies).
func (r *Repo) LocalWorkspaces() ([]WorkspaceEntry, error) {
	data, err := os.ReadFile(filepath.Join(r.CommonDir(), workspacesFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var l workspaceList
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", workspacesFile, err)
	}
	return l.Workspaces, nil
}

func (r *Repo) saveWorkspaces(list []WorkspaceEntry) error {
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	if list == nil {
		list = []WorkspaceEntry{}
	}
	return writeJSON(filepath.Join(r.CommonDir(), workspacesFile), workspaceList{list})
}

// changeWorkspaces rewrites the registry, one program at a time.
func (r *Repo) changeWorkspaces(change func([]WorkspaceEntry) ([]WorkspaceEntry, error)) error {
	unlock, err := flock.Lock(filepath.Join(r.CommonDir(), "workspaces.lock"), 10*time.Second)
	if err != nil {
		if errors.Is(err, flock.ErrBusy) {
			return ErrBusy
		}
		return err
	}
	defer unlock()
	list, err := r.LocalWorkspaces()
	if err != nil {
		return err
	}
	if list, err = change(list); err != nil {
		return err
	}
	return r.saveWorkspaces(list)
}

// otherCopies opens the project's other copies on this computer: the main
// one and the registered workspaces, but this one. A folder that is gone or
// isn't one of them any more is left out.
func (r *Repo) otherCopies() []*Repo {
	var roots []string
	if r.IsWorkspace() {
		roots = append(roots, r.MainRoot())
	}
	list, _ := r.LocalWorkspaces()
	for _, e := range list {
		roots = append(roots, e.Folder)
	}
	var out []*Repo
	for _, root := range roots {
		if samePath(root, r.Root) {
			continue
		}
		if o, err := openAt(root); err == nil && samePath(o.CommonDir(), r.CommonDir()) {
			out = append(out, o)
		}
	}
	return out
}

// lockOthers keeps the other copies from changing anything until the
// returned function is called: cleaning the shared store up while another
// copy commits could take files a version is about to name. It doesn't
// wait: a copy at work (ErrBusy) means not now.
func (r *Repo) lockOthers() (func(), error) {
	var held []func()
	release := func() {
		for _, f := range held {
			f()
		}
	}
	for _, o := range r.otherCopies() {
		unlock, err := o.Lock(0)
		if err != nil {
			release()
			return nil, err
		}
		held = append(held, unlock)
	}
	return release, nil
}

// workspaceStep, when set, hears of each step of adding or removing a
// workspace: tests stop there, as a crash would (killed_test.go).
var workspaceStep func(step string)

func stepDone(step string) {
	if workspaceStep != nil {
		workspaceStep(step)
	}
}

// WorkspaceOptions says what a new workspace works on.
type WorkspaceOptions struct {
	Branch string // the name of its branch, made for it
	From   string // a branch or version to start from; "": where this copy is
}

// AddWorkspace makes folder a workspace of the project: a branch of its
// own on the team (made from this copy's branch and version, or From), the
// files of that version taken from this computer (the store, or a copy's
// file with that content; downloaded only when neither has it), its own
// .r3v, then its entry in the registry. Stopped half-way, the folder is a
// leftover: AddWorkspace again finishes it, RemoveWorkspace clears it.
func (r *Repo) AddWorkspace(folder string, opts WorkspaceOptions) (*WorkspaceEntry, error) {
	if !AgentWorkspaces {
		return nil, ErrWorkspacesNotInBuild
	}
	if r.Config.Remote == nil {
		return nil, fmt.Errorf("a workspace works on a branch of the team's: %w", ErrNoRemote)
	}
	name := strings.TrimSpace(opts.Branch)
	if name == "" {
		return nil, errors.New("a workspace needs a name for its branch")
	}
	root, err := filepath.Abs(folder)
	if err != nil {
		return nil, err
	}
	list, err := r.LocalWorkspaces()
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if samePath(e.Folder, root) {
			if e.Branch == name || remote.SameBranchName(r.BranchLabel(e.Branch), name) {
				return &e, nil // made already
			}
			return nil, fmt.Errorf("%s is a workspace already (on %s)", root, r.BranchLabel(e.Branch))
		}
	}
	for _, other := range append([]string{r.MainRoot()}, folders(list)...) {
		if samePath(other, root) || within(root, other) || within(other, root) {
			return nil, fmt.Errorf("a workspace can't be in another copy of the project, or hold one (%s)", other)
		}
	}
	link, err := readLink(filepath.Join(root, metaDir))
	leftover := err == nil && link != nil && samePath(link.Common, r.CommonDir())
	if !leftover {
		if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
			return nil, fmt.Errorf("%s is not empty", root)
		}
	}
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	if !leftover {
		link = &workspaceLink{Common: r.CommonDir()}
		if link.Base, link.From, err = r.workspaceStart(c, opts.From); err != nil {
			return nil, err
		}
	}
	if key, err := r.ResolveBranch(name); err == nil && key == link.Base {
		return nil, fmt.Errorf("a workspace works on a branch of its own: %s is the one it's made from", name)
	}
	m, err := r.Load(link.From)
	if err != nil {
		return nil, err
	}
	ws, err := r.prepareWorkspace(root, link)
	if err != nil {
		return nil, err
	}
	ws.OnProgress = r.OnProgress

	// The files: from the store, or a file here with the same content. A
	// leftover gets them again whole (a stopped add, or remove, left some).
	if h := ws.Head(); h != "" {
		if m, err = ws.Load(h); err != nil {
			return nil, err
		}
	}
	if leftover || ws.Head() == "" {
		ws.removeLeftovers()
		if _, err := ws.putFiles(m, m.ID, m.ID); err != nil {
			return nil, err
		}
	}
	stepDone("files")

	// Build outputs the rules name (workspaces: copy), so it needn't build
	// from nothing.
	if _, err := ws.copyFolders(r); err != nil {
		return nil, fmt.Errorf("copying the build folders: %w", err)
	}
	stepDone("copied")

	// Its branch, at that version.
	key := ws.BranchName()
	if key == link.Base {
		// An agent's branch (kind, owner: folded away in the graph); a team
		// that keeps no branch records gets an ordinary one.
		key, err = ws.NewBranchWith(name, BranchOptions{Switch: true, Agent: true})
		if errors.Is(err, remote.ErrNoBranchRecords) {
			key, err = ws.NewBranch(name, "", true)
		}
		if err != nil {
			k, ok := ws.adoptBranch(c, name, link.Base)
			if !ok {
				return nil, err
			}
			key = k
		}
	} else if key != name && !remote.SameBranchName(ws.BranchLabel(key), name) {
		return nil, fmt.Errorf("%s is a workspace being made for the branch %s: add it with that name, or remove it",
			root, ws.BranchLabel(key))
	}
	stepDone("branch")

	// Registered last: listed, it is whole.
	e := WorkspaceEntry{Folder: root, Branch: key, Base: link.Base, Created: time.Now().UTC()}
	err = r.changeWorkspaces(func(list []WorkspaceEntry) ([]WorkspaceEntry, error) {
		list = slices.DeleteFunc(list, func(o WorkspaceEntry) bool { return samePath(o.Folder, root) })
		return append(list, e), nil
	})
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func folders(list []WorkspaceEntry) []string {
	var out []string
	for _, e := range list {
		out = append(out, e.Folder)
	}
	return out
}

// workspaceStart is where a workspace starts: the branch it is made from
// and the version. from is a branch (its latest version on the team), a
// version, or "" (this copy's branch and version).
func (r *Repo) workspaceStart(c remote.Backend, from string) (base, id string, err error) {
	if from == "" {
		if r.Head() == "" {
			return "", "", errors.New("commit a first version before making workspaces")
		}
		return r.BranchName(), r.Head(), nil
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil && !errors.Is(err, remote.ErrNotFound) {
		return "", "", err
	}
	if key, err := r.ResolveBranch(from); err == nil && heads[key] != "" {
		if err := r.fetchSnapshots(c, heads[key]); err != nil {
			return "", "", err
		}
		return key, heads[key], nil
	}
	if id, err = r.Resolve(from); err != nil {
		return "", "", fmt.Errorf("no branch or version %q", from)
	}
	m, err := r.Header(id)
	if err != nil {
		return "", "", err
	}
	if base = m.Branch; base == "" || heads[base] == "" {
		base = r.BranchName()
	}
	return base, id, nil
}

// prepareWorkspace writes a workspace's .r3v (what isn't there yet: its
// record first, so a stop leaves a folder known as a leftover) and opens it.
func (r *Repo) prepareWorkspace(root string, link *workspaceLink) (*Repo, error) {
	dir := filepath.Join(root, metaDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if l, _ := readLink(dir); l == nil {
		if err := writeJSON(filepath.Join(dir, workspaceLinkFile), link); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		id := make([]byte, 16)
		rand.Read(id)
		cfg := Config{ProjectID: r.Config.ProjectID, Name: r.Config.Name, Author: r.Config.Author,
			Remote: r.Config.Remote, Branch: link.Base, WorkspaceID: hex.EncodeToString(id)}
		if err := writeJSON(filepath.Join(dir, "config.json"), cfg); err != nil {
			return nil, err
		}
	}
	return openAt(root)
}

// adoptBranch takes the branch called name when it is this workspace's own,
// made by an add that stopped before the workspace knew it (at its
// version, not the base).
func (r *Repo) adoptBranch(c remote.Backend, name, base string) (string, bool) {
	key, err := r.ResolveBranch(name)
	if err != nil || key == base {
		return "", false
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil || heads[key] == "" || heads[key] != r.Head() {
		return "", false
	}
	r.Config.Branch, r.Config.Tip = key, ""
	if r.SaveConfig() != nil {
		return "", false
	}
	return key, true
}

// WorkspaceStatus is a workspace as the overview shows it.
type WorkspaceStatus struct {
	WorkspaceEntry
	BranchName string // what its branch is called
	Head       string // the version it is on
	// Ahead: versions on its branch not in the base branch; Behind: in the
	// base branch, not on its branch. Nil when the team can't be reached.
	Ahead, Behind *int
	Changes       int       // files changed, not committed
	Parked        int       // parked sets
	LastActivity  time.Time // its last commit, switch or change not committed
	// Missing: the folder is gone, or no longer this project's workspace
	// (remove takes it off the list).
	Missing bool
}

// WorkspaceStatuses lists the workspaces with where each is (see
// WorkspaceStatus). It reads every workspace's files for its changes.
func (r *Repo) WorkspaceStatuses() ([]WorkspaceStatus, error) {
	list, err := r.LocalWorkspaces()
	if err != nil {
		return nil, err
	}
	var heads map[string]string
	var recs map[string]remote.BranchRecord
	c, err := r.Client()
	if err == nil {
		if heads, err = c.Branches(r.Config.ProjectID); err != nil && !errors.Is(err, remote.ErrNotFound) {
			heads = nil
		} else if heads == nil {
			heads = map[string]string{}
		}
		recs, _ = r.BranchRecords()
	}
	out := []WorkspaceStatus{}
	for _, e := range list {
		s := WorkspaceStatus{WorkspaceEntry: e, BranchName: branchLabel(recs, e.Branch), LastActivity: e.Created}
		ws, err := openAt(e.Folder)
		if err != nil || !samePath(ws.CommonDir(), r.CommonDir()) {
			s.Missing = true
			out = append(out, s)
			continue
		}
		s.Head = ws.Head()
		if fi, err := os.Stat(filepath.Join(ws.Dir, "HEAD")); err == nil && fi.ModTime().After(s.LastActivity) {
			s.LastActivity = fi.ModTime()
		}
		if changes, err := ws.Status(); err == nil {
			s.Changes = len(changes)
			for _, ch := range changes {
				if fi, err := os.Stat(ws.Abs(ch.Path)); err == nil && fi.ModTime().After(s.LastActivity) {
					s.LastActivity = fi.ModTime()
				}
			}
		}
		s.Parked = len(ws.ParkedSets())
		if heads != nil {
			base := heads[e.Base]
			if r.fetchSnapshots(c, base) == nil {
				if ahead, behind, err := r.aheadBehind(ws.tip(), base); err == nil {
					s.Ahead, s.Behind = &ahead, &behind
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// tip is the latest version of the branch this copy is on.
func (r *Repo) tip() string {
	if r.Config.Tip != "" {
		return r.Config.Tip
	}
	return r.Head()
}

// aheadBehind counts the versions a has that b hasn't, and b has that a
// hasn't.
func (r *Repo) aheadBehind(a, b string) (ahead, behind int, err error) {
	ancA, err := r.ancestors(a)
	if err != nil {
		return 0, 0, err
	}
	ancB, err := r.ancestors(b)
	if err != nil {
		return 0, 0, err
	}
	for id := range ancA {
		if !ancB[id] {
			ahead++
		}
	}
	for id := range ancB {
		if !ancA[id] {
			behind++
		}
	}
	return ahead, behind, nil
}

// WorkspaceNotDoneError: a workspace has work its removal would lose.
type WorkspaceNotDoneError struct {
	Folder   string
	Changes  int // files changed, not committed
	Parked   int // parked sets
	Unmerged int // versions of its branch not in the base branch
	Base     string
}

func (e *WorkspaceNotDoneError) Error() string {
	var what []string
	if e.Unmerged > 0 {
		what = append(what, fmt.Sprintf("%d version(s) not merged into %s", e.Unmerged, e.Base))
	}
	if e.Changes > 0 {
		what = append(what, fmt.Sprintf("%d change(s) not committed", e.Changes))
	}
	if e.Parked > 0 {
		what = append(what, fmt.Sprintf("%d set(s) of parked changes", e.Parked))
	}
	return fmt.Sprintf("%s has %s", e.Folder, strings.Join(what, ", "))
}

func (e *WorkspaceNotDoneError) Unwrap() error { return ErrWorkspaceNotDone }

// RemoveWorkspace takes a workspace away: refused while it has work not
// merged into its base branch, changes not committed or parked (unless
// force), or a command is working in it. It is unregistered first, then its
// parked and kept versions go (no other copy needs them), then the folder.
// Stopped half-way, the folder is a leftover RemoveWorkspace clears. Its
// branch stays on the team (its versions are there).
func (r *Repo) RemoveWorkspace(folder string, force bool) error {
	root, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	if samePath(root, r.MainRoot()) {
		return errors.New("that is the project's main copy, not a workspace")
	}
	if samePath(root, r.Root) || within(r.Root, root) {
		return errors.New("a workspace can't remove itself: run it in the project's main copy")
	}
	list, err := r.LocalWorkspaces()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(e WorkspaceEntry) bool { return samePath(e.Folder, root) })
	if i < 0 {
		// A leftover (an add or a remove that stopped): cleared.
		if l, err := readLink(filepath.Join(root, metaDir)); err != nil || l == nil || !samePath(l.Common, r.CommonDir()) {
			return fmt.Errorf("%s is not a workspace of this project", root)
		}
		return removeFolder(root)
	}
	entry := list[i]
	ws, err := openAt(root)
	if err != nil || !samePath(ws.CommonDir(), r.CommonDir()) {
		// Gone, or someone else's now: only taken off the list.
		return r.unregister(root)
	}
	unlock, err := ws.Lock(0)
	if err != nil {
		return err
	}
	if !force {
		if err := r.workspaceDone(ws, entry); err != nil {
			unlock()
			return err
		}
	}
	if err := r.unregister(root); err != nil {
		unlock()
		return err
	}
	stepDone("unregistered")
	drop := ws.parkedIDs()
	for id := range ws.keptWork() {
		drop[id] = true
	}
	held := map[string]bool{ws.Head(): true, ws.tip(): true} // (never a version it is on)
	for _, o := range r.otherCopies() {
		held[o.Head()], held[o.tip()] = true, true
		for id := range o.parkedIDs() {
			held[id] = true
		}
		for id := range o.keptWork() {
			held[id] = true
		}
	}
	for id := range drop {
		if !held[id] {
			os.Remove(r.snapshotPath(id))
		}
	}
	wsID := ws.Config.WorkspaceID
	unlock()
	if err := removeFolder(root); err != nil {
		return err
	}
	// Its agent locks go with it; its branch, all merged, is archived (its
	// versions stay; as well as can be: it may be someone's now).
	if wsID != "" {
		r.AgentLockList().ReleaseWorkspace(wsID)
	}
	if !force {
		r.ArchiveBranch(entry.Branch)
	}
	return nil
}

func (r *Repo) unregister(root string) error {
	return r.changeWorkspaces(func(list []WorkspaceEntry) ([]WorkspaceEntry, error) {
		return slices.DeleteFunc(list, func(e WorkspaceEntry) bool { return samePath(e.Folder, root) }), nil
	})
}

// workspaceDone fails (WorkspaceNotDoneError) when the workspace has work
// removing it would lose: its versions not all in its base branch (asked of
// the team), changes not committed, or parked.
func (r *Repo) workspaceDone(ws *Repo, e WorkspaceEntry) error {
	nd := &WorkspaceNotDoneError{Folder: ws.Root, Base: r.BranchLabel(e.Base)}
	changes, err := ws.Status()
	if err != nil {
		return err
	}
	nd.Changes = len(changes)
	nd.Parked = len(ws.ParkedSets())
	c, err := r.Client()
	if err != nil {
		return err
	}
	heads, err := c.Branches(r.Config.ProjectID)
	if err != nil {
		return fmt.Errorf("can't tell whether its versions are merged: %w", err)
	}
	base := heads[e.Base]
	if err := r.fetchSnapshots(c, base); err != nil {
		return err
	}
	for _, v := range []string{ws.tip(), heads[ws.BranchName()]} {
		if err := r.fetchSnapshots(c, v); err != nil {
			return err
		}
		ahead, _, err := r.aheadBehind(v, base)
		if err != nil {
			return err
		}
		nd.Unmerged = max(nd.Unmerged, ahead)
	}
	if nd.Changes > 0 || nd.Parked > 0 || nd.Unmerged > 0 {
		return nd
	}
	return nil
}

// removeFolder is removeTree, saying what is left when it can't finish.
func removeFolder(root string) error {
	if err := removeTree(root); err != nil {
		return fmt.Errorf("%s is no longer a workspace, but not all of it could be deleted "+
			"(a program has a file of it open?): remove it again: %w", root, err)
	}
	return nil
}

// removeTree deletes a workspace's folder: its files, then its .r3v (so a
// stop part way leaves a folder still known as a leftover), then itself.
// Read-only files go too (store.Remove).
func removeTree(root string) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == metaDir {
			continue
		}
		if err := removeAll(filepath.Join(root, e.Name())); err != nil {
			return err
		}
		stepDone("removed " + e.Name())
	}
	if err := removeAll(filepath.Join(root, metaDir)); err != nil {
		return err
	}
	return store.Remove(root)
}

func removeAll(p string) error {
	var dirs []string
	err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if err := store.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := store.Remove(dirs[i]); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// samePath: a and b are the same folder (by name: case doesn't count on
// Windows).
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// within: p is inside dir (not dir itself).
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(rel)
}
