package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/nonlabhq/r3v/internal/remote"
)

// File locks (docs/design/locks.md): a share tells a hosted team which
// paths it gives new content to, and the service refuses it when someone
// else holds one of them.

// changedPaths lists the paths the versions up to head give new content to,
// next to what the team already has (the versions up to the heads in
// shared): each version not among those, compared with its parents. A
// path counts when the version's content there is none of its parents'
// (added, changed, deleted; for a merge, a decision that made something
// new). Content merged in from the team's versions is a parent's, so it
// doesn't count: merging main into a branch brings teammates' changes, not
// the sender's. The sender's own versions under a merge count on their own.
func (r *Repo) changedPaths(head string, shared []string) ([]string, error) {
	have := map[string]bool{}
	for _, h := range shared {
		if h == "" || !r.HasSnapshot(h) {
			continue // (a head not here: none of ours can be in it)
		}
		anc, err := r.ancestors(h)
		if err != nil {
			return nil, err
		}
		for id := range anc {
			have[id] = true
		}
	}
	changed := map[string]bool{}
	seen := map[string]bool{}
	stack := []string{head}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == "" || seen[id] || have[id] {
			continue
		}
		seen[id] = true
		m, err := r.Load(id)
		if err != nil {
			return nil, err
		}
		files := m.FileMap()
		var parents []map[string]FileEntry
		for _, p := range m.Parents {
			pm, err := r.Load(p)
			if err != nil {
				return nil, err
			}
			parents = append(parents, pm.FileMap())
			stack = append(stack, p)
		}
		paths := map[string]bool{}
		for p := range files {
			paths[p] = true
		}
		for _, pf := range parents {
			for p := range pf {
				paths[p] = true
			}
		}
		for p := range paths {
			if changed[p] {
				continue
			}
			mine := files[p].Hash
			fromParent := false
			for _, pf := range parents {
				if pf[p].Hash == mine {
					fromParent = true
					break
				}
			}
			if !fromParent {
				changed[p] = true
			}
		}
	}
	out := make([]string, 0, len(changed))
	for p := range changed {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// SharedChanges lists the paths a share of the current branch would give
// new content to (see changedPaths), next to the team's branches.
func (r *Repo) SharedChanges(teamHeads map[string]string) ([]string, error) {
	var shared []string
	for _, h := range teamHeads {
		shared = append(shared, h)
	}
	return r.changedPaths(r.Head(), shared)
}

// --- locks waiting to be taken -------------------------------------------------

// lockQueueFile lists paths of a locked kind that changed while the team
// couldn't be reached: the app locks them once it can.
const lockQueueFile = "locks-waiting.json"

// WaitingLocks are the paths waiting to be locked.
func (r *Repo) WaitingLocks() []string {
	var out []string
	if data, err := os.ReadFile(filepath.Join(r.Dir, lockQueueFile)); err == nil {
		json.Unmarshal(data, &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// SetWaitingLocks replaces the paths waiting to be locked (written whole,
// then renamed: stopped half-way, the old list stays).
func (r *Repo) SetWaitingLocks(paths []string) error {
	file := filepath.Join(r.Dir, lockQueueFile)
	if len(paths) == 0 {
		err := os.Remove(file)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, _ := json.Marshal(paths)
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// ErrLocked is a share refused because someone else holds paths it changes
// (the versions stay here, shared once the locks go).
type ErrLocked = remote.ErrLocked
