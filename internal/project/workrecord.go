package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
)

// Who is working where (docs/design/branch-tree.md): each copy of a project
// keeps a workspace record in the team's storage (see takeback.go), with
// the work it has that the team hasn't: changes not committed, versions not
// shared, changes parked. A last known state with its time, written when it
// changes, at most every few minutes (at once when the branch changes), so
// archiving a branch can say "Kai is on Test: 3 changes not committed".

// noteEvery is how often at most the record is written for the same branch.
var noteEvery = 5 * time.Minute

// notedFile keeps the record last written, to tell when it changes.
const notedFile = "workspace-noted.json"

// Workspace is another copy of the project, as it last told the team.
type Workspace struct {
	Member   string    // its member id
	Branch   string    // the branch it is on
	Changes  int       // files changed, not committed
	Unshared int       // versions committed, not shared
	Parked   []string  // branches it has changes parked on
	Time     time.Time // when it said so
}

func (r *Repo) noted() (workspaceRecord, bool) {
	var rec workspaceRecord
	data, err := os.ReadFile(filepath.Join(r.Dir, notedFile))
	return rec, err == nil && json.Unmarshal(data, &rec) == nil
}

// workspaceID is this copy's id in the team's storage (made the first time).
func (r *Repo) workspaceID() string {
	if r.Config.WorkspaceID == "" {
		b := make([]byte, 16)
		rand.Read(b)
		r.Config.WorkspaceID = hex.EncodeToString(b)
		if r.SaveConfig() != nil {
			return ""
		}
	}
	return r.Config.WorkspaceID
}

// putWorkspace writes rec as this copy's record (as well as it can: a
// record that didn't go up is written next time).
func (r *Repo) putWorkspace(c remote.Backend, rec workspaceRecord) {
	if rec.ID = r.workspaceID(); rec.ID == "" {
		return
	}
	member, _ := r.Identity()
	rec.Member, rec.Time = member, time.Now().UTC()
	if c.PutWorkspace(r.Config.ProjectID, rec.ID, rec) != nil {
		return
	}
	if data, err := json.Marshal(rec); err == nil {
		os.WriteFile(filepath.Join(r.Dir, notedFile), data, 0o644)
	}
}

// NoteWork tells the team what work this copy has (changes: files changed,
// not committed, from Status), when that changed since it last said so. No
// team, nothing written.
func (r *Repo) NoteWork(changes int) {
	if r.Config.Remote == nil || r.Config.ProjectID == "" {
		return
	}
	branch := r.BranchName()
	rec := workspaceRecord{Branch: branch, Has: r.seenHeads()[branch], Changes: changes, Unshared: r.unsharedHere(branch)}
	for _, p := range r.ParkedSets() {
		if !slices.Contains(rec.Parked, p.Branch) {
			rec.Parked = append(rec.Parked, p.Branch)
		}
	}
	sort.Strings(rec.Parked)
	if old, ok := r.noted(); ok {
		if old.Branch == rec.Branch && old.Has == rec.Has && old.Changes == rec.Changes &&
			old.Unshared == rec.Unshared && slices.Equal(old.Parked, rec.Parked) {
			return
		}
		if old.Branch == rec.Branch && time.Since(old.Time) < noteEvery {
			return
		}
	}
	c, err := r.Client()
	if err != nil {
		return
	}
	r.putWorkspace(c, rec)
}

// unsharedHere counts the versions here the team's branch doesn't have (as
// last seen).
func (r *Repo) unsharedHere(branch string) int {
	head := r.Latest()
	if head == "" {
		return 0
	}
	mine, err := r.ancestors(head)
	if err != nil {
		return 0
	}
	if seen := r.seenHeads()[branch]; seen != "" {
		if theirs, err := r.ancestors(seen); err == nil {
			for id := range theirs {
				delete(mine, id)
			}
		}
	}
	return len(mine)
}

// Workspaces lists the other copies of the project, as they last told the
// team, newest first.
func (r *Repo) Workspaces() ([]Workspace, error) {
	c, err := r.Client()
	if err != nil {
		return nil, err
	}
	var recs []workspaceRecord
	if err := c.Workspaces(r.Config.ProjectID, &recs); err != nil {
		return nil, err
	}
	var out []Workspace
	for _, rec := range recs {
		if rec.ID == r.Config.WorkspaceID {
			continue
		}
		out = append(out, Workspace{Member: rec.Member, Branch: rec.Branch, Changes: rec.Changes, Unshared: rec.Unshared,
			Parked: rec.Parked, Time: rec.Time})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}
