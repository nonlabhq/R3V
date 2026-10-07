package desktop

import (
	"sync/atomic"

	"github.com/nonlabhq/r3v/internal/project"
)

// A commit (or a share) can be cancelled while it reads, stores and uploads
// the files, until the team has the version: see project.Repo.Save.

// cancellable lets CancelSave stop what r does for root; the returned func
// ends that.
func (a *App) cancellable(root string, r *project.Repo) func() {
	stop := &atomic.Bool{}
	r.Cancel = func() error {
		if stop.Load() {
			return project.ErrCancelled
		}
		return nil
	}
	a.mu.Lock()
	if a.saving == nil {
		a.saving = map[string]*atomic.Bool{}
	}
	a.saving[root] = stop
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		if a.saving[root] == stop {
			delete(a.saving, root)
		}
		a.mu.Unlock()
	}
}

// CancelSave stops root's commit or share under way, if it can still stop
// (false when there is none).
func (a *App) CancelSave(root string) bool {
	a.mu.Lock()
	stop := a.saving[root]
	a.mu.Unlock()
	if stop == nil {
		return false
	}
	stop.Store(true)
	return true
}

func (a *App) canCancel(root string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.saving[root] != nil
}

// cancelled is the result of a commit stopped: "cancelled" when nothing
// was committed (the changes are as they were), "cancelled-kept" when
// versions are committed here, not shared yet.
func cancelled(res *project.SyncResult, kept bool) *Result {
	out := syncResult(res)
	out.Action = "cancelled"
	if kept {
		out.Action = "cancelled-kept"
	}
	return out
}
