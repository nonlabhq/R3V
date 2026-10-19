package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Archived projects (docs/design/archive.md): put away for the whole team,
// nothing deleted. On this computer the folder stays as it is; the app
// stops watching it (no polling, no notices) and the left panel leaves it
// out; the team's home lists it under Archived. On an R3V Cloud team its
// admins archive; on a team's own storage, anyone in it (as Workflow).

// ArchiveInfo is what archiving a project asks first.
type ArchiveInfo struct {
	// CanArchive: this computer may (an R3V Cloud team's admins only).
	CanArchive bool `json:"canArchive"`
	Archived   bool `json:"archived"`
	// Changes and Unshared: the copy here has uncommitted changes, or
	// versions the team hasn't got (archiving asks first).
	Changes  int  `json:"changes"`
	Unshared bool `json:"unshared"`
}

// ArchiveInfo reads whether a project is archived and what archiving it
// would leave behind here.
func (a *App) ArchiveInfo(teamID, projectID string) (*ArchiveInfo, error) {
	store, t, err := findTeam(teamID)
	if err != nil {
		return nil, err
	}
	out := &ArchiveInfo{CanArchive: canEditWorkflow(t)}
	b, err := t.Open()
	if err != nil {
		return nil, err
	}
	got, err := remote.ArchivedProjects(b)
	if err != nil {
		return nil, err
	}
	_, out.Archived = got[projectID]
	if root := store.ProjectRoot(teamID, projectID); root != "" {
		out.Changes, out.Unshared, err = a.notShared(root)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// notShared counts the uncommitted changes in the copy at root, and says
// whether it has versions the team hasn't got.
func (a *App) notShared(root string) (int, bool, error) {
	r, unlock, _, err := a.openRead(root)
	if err != nil {
		if errors.Is(err, project.ErrNotRepo) || os.IsNotExist(err) {
			return 0, false, nil // (the folder is gone: nothing here to lose)
		}
		return 0, false, err
	}
	defer unlock()
	changes, err := r.Status()
	if err != nil {
		return 0, false, err
	}
	shared, err := r.Shared()
	if err != nil {
		return 0, false, err
	}
	return len(changes), !shared, nil
}

// ArchiveProject archives a project for the whole team. Uncommitted
// changes or versions not shared in the copy here refuse it unless force
// (the person confirmed).
func (a *App) ArchiveProject(teamID, projectID string, force bool) error {
	store, t, err := findTeam(teamID)
	if err != nil {
		return err
	}
	if !canEditWorkflow(t) {
		return remote.ErrNotAdmin
	}
	root := store.ProjectRoot(teamID, projectID)
	if root != "" && !force {
		changes, unshared, err := a.notShared(root)
		if err != nil {
			return err
		}
		if changes > 0 {
			return fmt.Errorf("%w (%d file(s)): commit them first, or archive it all the same", project.ErrDirty, changes)
		}
		if unshared {
			return project.ErrUnshared
		}
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if _, err := remote.SetProjectArchived(b, projectID, true); err != nil {
		return err
	}
	markArchived(teamID, projectID, true)
	if root != "" {
		a.stopWatch(root)
	}
	return nil
}

// UnarchiveProject brings an archived project back for the whole team.
func (a *App) UnarchiveProject(teamID, projectID string) error {
	store, t, err := findTeam(teamID)
	if err != nil {
		return err
	}
	if !canEditWorkflow(t) {
		return remote.ErrNotAdmin
	}
	b, err := t.Open()
	if err != nil {
		return err
	}
	if _, err := remote.SetProjectArchived(b, projectID, false); err != nil {
		return err
	}
	markArchived(teamID, projectID, false)
	if root := store.ProjectRoot(teamID, projectID); root != "" {
		a.startWatch(root)
	}
	return nil
}

// followArchived takes on the team's archived projects as just read: the
// watches of those archived since stop, those brought back start.
func (a *App) followArchived(store *teams.Store, teamID string, now map[string]remote.ArchivedProject) {
	was := lastArchived(teamID)
	ids := make([]string, 0, len(now))
	for id := range now {
		ids = append(ids, id)
	}
	rememberArchived(teamID, ids)
	for id := range now {
		if root := store.ProjectRoot(teamID, id); root != "" && !was[id] {
			a.stopWatch(root)
		}
	}
	for id := range was {
		if _, still := now[id]; !still {
			if root := store.ProjectRoot(teamID, id); root != "" {
				a.startWatch(root)
			}
		}
	}
}

// isArchivedHere says whether the project at r is one its team archived,
// as this computer last read it.
func isArchivedHere(store *teams.Store, r *project.Repo) bool {
	if !remote.ProjectArchive || r.Config.Remote == nil {
		return false
	}
	t := store.FindByURL(r.Config.Remote.URL)
	return t != nil && lastArchived(t.ID)[r.Config.ProjectID]
}

// team-archived.json keeps each team's archived projects as last read, so
// the app knows them at once (what not to watch when it starts), and while
// the team can't be reached.
var archivedMu sync.Mutex

func archivedFile() string { return filepath.Join(teams.Dir(), "team-archived.json") }

func readArchived() map[string][]string {
	all := map[string][]string{}
	if data, err := os.ReadFile(archivedFile()); err == nil {
		json.Unmarshal(data, &all)
	}
	return all
}

func lastArchived(teamID string) map[string]bool {
	archivedMu.Lock()
	defer archivedMu.Unlock()
	out := map[string]bool{}
	for _, id := range readArchived()[teamID] {
		out[id] = true
	}
	return out
}

func rememberArchived(teamID string, ids []string) {
	archivedMu.Lock()
	defer archivedMu.Unlock()
	slices.Sort(ids)
	all := readArchived()
	if slices.Equal(all[teamID], ids) || len(ids) == 0 && all[teamID] == nil {
		return // unchanged: not written every minute
	}
	if len(ids) == 0 {
		delete(all, teamID)
	} else {
		all[teamID] = ids
	}
	writeArchived(all)
}

func markArchived(teamID, projectID string, on bool) {
	archivedMu.Lock()
	defer archivedMu.Unlock()
	all := readArchived()
	ids := slices.DeleteFunc(all[teamID], func(id string) bool { return id == projectID })
	if on {
		ids = append(ids, projectID)
		slices.Sort(ids)
	}
	if len(ids) == 0 {
		delete(all, teamID)
	} else {
		all[teamID] = ids
	}
	writeArchived(all)
}

// writeArchived writes the file whole, then renames it (stopped half-way,
// the old one stays).
func writeArchived(all map[string][]string) {
	data, err := json.Marshal(all)
	if err != nil {
		return
	}
	tmp := archivedFile() + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		if os.Rename(tmp, archivedFile()) != nil {
			os.Remove(tmp)
		}
	}
}

// archivedOf marks the projects in ps the team archived.
func archivedOf(ps []TeamProject, archived map[string]bool) {
	for i := range ps {
		if !ps[i].Workspace && ps[i].ID != "" && archived[ps[i].ID] {
			ps[i].Archived = true
		}
	}
}
