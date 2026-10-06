package desktop

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// Big files go up to the team's storage in the background once they stop
// changing (project.Preupload): one at a time, never while the project is
// busy, and without holding it up (only recording one takes its lock).

const preuploadCheck = time.Minute

// preuploadRetry: how soon a project asked for at once is tried again when
// it was busy.
var preuploadRetry = 3 * time.Second

// Preupload is a file going up in the background (event "preupload": Done
// false while it goes, true once it's up or stopped), and the project's big
// files waiting to go after it.
type Preupload struct {
	Root    string          `json:"root"`
	Path    string          `json:"path"`
	Bytes   int64           `json:"bytes"`
	Total   int64           `json:"total"`
	Done    bool            `json:"done"`
	Waiting []PreuploadFile `json:"waiting"`
}

// PreuploadFile is a big file waiting to go up.
type PreuploadFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

var (
	preuploadMu  sync.Mutex
	preuploading = map[string]Preupload{} // root
)

// Preuploads lists the files going up in the background now.
func (a *App) Preuploads() []Preupload {
	preuploadMu.Lock()
	defer preuploadMu.Unlock()
	out := []Preupload{}
	for _, p := range preuploading {
		out = append(out, p)
	}
	return out
}

// SetPreupload turns the background upload of big files on or off for
// team teamID on this computer.
func (a *App) SetPreupload(teamID string, on bool) error {
	_, err := updateTeam(teamID, func(_ *teams.Store, t *teams.Team) error {
		t.NoPreupload = !on
		return nil
	})
	return err
}

func (a *App) notePreupload(p Preupload) {
	preuploadMu.Lock()
	if p.Done {
		delete(preuploading, p.Root)
	} else {
		preuploading[p.Root] = p
	}
	preuploadMu.Unlock()
	if a.emit != nil {
		a.emit("preupload", p)
	}
}

// busy: an action holds the project now.
func (a *App) busy(root string) bool {
	a.mu.Lock()
	l, ok := a.locks[root]
	a.mu.Unlock()
	if !ok {
		return false
	}
	if l.TryLock() {
		l.Unlock()
		return false
	}
	return true
}

// preuploadOnSchedule looks for big files to put up early: when the app
// starts, every minute after, and at once for a project just added
// (preuploadSoon). Files still changing wait (PreuploadStable), the rest
// needn't.
func (a *App) preuploadOnSchedule(ctx context.Context) {
	if store, err := teams.Load(); err == nil {
		for _, root := range store.Roots() {
			if r, err := project.Open(root); err == nil {
				r.CleanPreuploads() // left by an upload the app closed on
			}
		}
	}
	only := "" // a project to look at now ("" all of them)
	for {
		store, err := teams.Load()
		if err == nil {
			for _, t := range store.Teams {
				if t.NoPreupload || !t.Remote.IsStorage() {
					continue
				}
				for key, root := range store.Projects {
					if strings.HasPrefix(key, t.ID+"/") && (only == "" || root == only) && ctx.Err() == nil {
						if !a.preuploadProject(ctx, root) && root == only {
							// Busy (its team watch reads it as it is added): again
							// in a moment rather than at the next round.
							time.AfterFunc(preuploadRetry, func() { a.preuploadSoon(root) })
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(preuploadCheck):
			only = ""
		case only = <-a.preuploadNow:
		}
	}
}

// preuploadSoon asks for root's big files to be looked at now.
func (a *App) preuploadSoon(root string) {
	select {
	case a.preuploadNow <- root:
	default: // the next round looks anyway
	}
}

// preuploadProject puts root's big files up; false when the project was
// busy and nothing was looked at.
func (a *App) preuploadProject(ctx context.Context, root string) bool {
	if a.busy(root) {
		return false
	}
	r, err := project.Open(root)
	if err != nil || r.Config.Remote == nil {
		return true
	}
	cands, err := r.PreuploadCandidates(time.Now())
	if err != nil || len(cands) == 0 {
		return true
	}
	c, err := r.Client()
	if err != nil {
		return true
	}
	for i, cand := range cands {
		if ctx.Err() != nil || a.busy(root) {
			return true
		}
		waiting := []PreuploadFile{}
		for _, w := range cands[i+1:] {
			waiting = append(waiting, PreuploadFile{Path: w.Path, Size: w.Size})
		}
		p := Preupload{Root: root, Path: cand.Path, Total: cand.Size, Waiting: waiting}
		a.notePreupload(p)
		// It stops when the file goes away, is left out (by the rules, or a
		// folder it is in) or changes, or the app closes.
		var mod time.Time
		if fi, err := os.Stat(r.Abs(cand.Path)); err == nil {
			mod = fi.ModTime()
		}
		still := func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return r.StillWanted(cand, mod)
		}
		var last time.Time
		err := r.Preupload(c, cand, func(done, total int64) {
			if time.Since(last) > 500*time.Millisecond {
				last = time.Now()
				p.Bytes, p.Total = done, total
				a.notePreupload(p)
			}
		}, still)
		if err == nil {
			err = a.recordPreupload(root, cand.Hash)
		}
		p.Done = true
		a.notePreupload(p)
		if err != nil && !errors.Is(err, project.ErrChangedSince) && !errors.Is(err, project.ErrPreuploadStopped) {
			log.Printf("preupload %s %s: %v", root, cand.Path, err)
			return true // tried again next time
		}
	}
	return true
}

// recordPreupload notes the upload, with the project locked.
func (a *App) recordPreupload(root, hash string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	return r.NotePreuploaded(hash)
}
