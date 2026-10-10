package desktop

import (
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
)

// Branch names and colours (Nightly, see remote.BranchRecord): asked with
// the team's state (TeamState), kept for the times the page is shown
// without asking (State).

var branchRecordCache = struct {
	sync.Mutex
	byRoot map[string]map[string]remote.BranchRecord
}{byRoot: map[string]map[string]remote.BranchRecord{}}

// branchRecords returns project r's branch records: asked from the team
// when ask, else as last asked.
func branchRecords(r *project.Repo, ask bool) map[string]remote.BranchRecord {
	branchRecordCache.Lock()
	last := branchRecordCache.byRoot[r.Root]
	branchRecordCache.Unlock()
	if !ask || r.Config.Remote == nil {
		return last
	}
	recs, err := r.BranchRecords()
	if err != nil {
		return last // as last seen
	}
	branchRecordCache.Lock()
	branchRecordCache.byRoot[r.Root] = recs
	branchRecordCache.Unlock()
	return recs
}

// keepsBranchRecords reports whether r's team keeps branch names and
// colours (the app offers renaming and colours only then).
func keepsBranchRecords(r *project.Repo) bool {
	if r.Config.Remote == nil {
		return false
	}
	c, err := r.Client()
	if err != nil {
		return false
	}
	_, ok := remote.BranchRecordsOf(c)
	return ok
}

// CreateBranch starts a branch called name (any text where the team keeps
// branch records) with color (a palette number, "" for the app's pick), and
// switches to it when switchTo is set. It returns the branch's key.
func (a *App) CreateBranch(root, name, color string, switchTo bool) (string, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return "", err
	}
	defer unlock()
	key, err := r.NewBranch(strings.TrimSpace(name), color, switchTo)
	if err == nil {
		branchRecords(r, true)
	}
	return key, err
}

// SetBranchRecord renames branch key and sets its colour, for the whole
// team.
func (a *App) SetBranchRecord(root, key, name, color string) error {
	r, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := r.SetBranchRecord(key, name, color); err != nil {
		return err
	}
	branchRecords(r, true)
	return nil
}

// BranchList is a project's branches for its settings.
type BranchList struct {
	Branches []Branch `json:"branches"`
	// Names: the team keeps branch names and colours (they can be changed).
	Names bool `json:"names"`
}

// BranchList lists project root's branches with their names and colours,
// from what the team said last (asking it when it hasn't yet).
func (a *App) BranchList(root string) (*BranchList, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	out := &BranchList{Branches: []Branch{}, Names: keepsBranchRecords(r)}
	if r.Config.Remote == nil {
		return out, nil
	}
	view, _ := a.cachedTeam(root)
	if view == nil {
		if view, err = r.FetchTeam(); err != nil {
			return nil, err
		}
		a.storeTeam(root, view, nil)
	}
	recs := branchRecords(r, true)
	for _, b := range r.BranchesFrom(view.Heads) {
		br := Branch{Name: b.Name, Label: recs[b.Name].Name, Color: recs[b.Name].Color, Parent: recs[b.Name].Parent, Current: b.Current}
		if b.Latest != nil {
			v := toVersion(b.Latest, nil)
			br.Latest = &v
		}
		out.Branches = append(out.Branches, br)
	}
	return out, nil
}

// ArchivedBranch is a branch archived, for getting it back.
type ArchivedBranch struct {
	Name  string `json:"name"` // its key
	Label string `json:"label"`
	Color string `json:"color"`
	By    string `json:"by"` // who archived it ("" unknown)
	Time  string `json:"time"`
	// Latest: its latest version then (nil when not on this computer).
	Latest *Version `json:"latest"`
}

func (a *App) archivedBranches(r *project.Repo) ([]ArchivedBranch, error) {
	gone, err := r.ArchivedBranches()
	if err != nil {
		return nil, err
	}
	recs := branchRecords(r, true)
	names := a.memberNames(r)
	out := []ArchivedBranch{}
	for _, d := range gone {
		db := ArchivedBranch{Name: d.Key, Label: recs[d.Key].Name, Color: recs[d.Key].Color, By: names[d.By],
			Time: d.Time.Format(time.RFC3339)}
		if r.HasSnapshot(d.Head) {
			if m, err := r.Load(d.Head); err == nil {
				v := toVersion(m, nil)
				db.Latest = &v
			}
		}
		out = append(out, db)
	}
	return out, nil
}

// ArchivedBranches lists project root's archived branches, the last
// archived first.
func (a *App) ArchivedBranches(root string) ([]ArchivedBranch, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	return a.archivedBranches(r)
}

// VersionsOnlyOnBranch counts the versions of branch key no other branch
// has (what archiving it leaves on no branch: they stay recoverable).
func (a *App) VersionsOnlyOnBranch(root, key string) (int, error) {
	r, err := project.Open(root)
	if err != nil {
		return 0, err
	}
	return r.OnlyOnBranch(key)
}

// ArchiveBranches archives branches (a branch and those made from it: not
// main, not the one you are on); keep: those made from them that stay,
// each with the branch it now comes from. They can be got back
// (UnarchiveBranch).
func (a *App) ArchiveBranches(root string, keys []string, keep map[string]string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := r.ArchiveBranches(keys, keep); err != nil {
		return err
	}
	a.refetchTeam(r)
	branchRecords(r, true)
	return nil
}

// DeleteBranch deletes archived branch key for good (listed no more).
func (a *App) DeleteBranch(root, key string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := r.DeleteBranch(key); err != nil {
		return err
	}
	branchRecords(r, true)
	return nil
}

// UnarchiveBranch puts archived branch key back where it was.
func (a *App) UnarchiveBranch(root, key string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := r.UnarchiveBranch(key); err != nil {
		return err
	}
	a.refetchTeam(r)
	branchRecords(r, true)
	return nil
}

// refetchTeam asks the team again after a change of its branches, so the
// page shows them at once.
func (a *App) refetchTeam(r *project.Repo) {
	if view, err := r.FetchTeam(); err == nil {
		a.storeTeam(r.Root, view, nil)
	}
}
