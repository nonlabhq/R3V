package desktop

import (
	"strings"
	"sync"

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
// switches to it. It returns the branch's key.
func (a *App) CreateBranch(root, name, color string) (string, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return "", err
	}
	defer unlock()
	key, err := r.CreateBranchNamed(strings.TrimSpace(name), color)
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
		br := Branch{Name: b.Name, Label: recs[b.Name].Name, Color: recs[b.Name].Color, Current: b.Current}
		if b.Latest != nil {
			v := toVersion(b.Latest, nil)
			br.Latest = &v
		}
		out.Branches = append(out.Branches, br)
	}
	return out, nil
}
