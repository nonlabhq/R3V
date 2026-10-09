package desktop

import (
	"github.com/nonlabhq/r3v/internal/project"
)

// Parked changes (Nightly, see docs/design/parking.md): switching keeps
// uncommitted changes with the place they were made.

func parkedSet(p *project.Parked) *ParkedSet {
	if p == nil {
		return nil
	}
	return &ParkedSet{Branch: p.Branch, At: p.At, Base: p.Base, Version: p.Version, Files: p.Files, Bytes: p.Bytes, Since: p.Since}
}

func parkedSets(r *project.Repo) []ParkedSet {
	out := []ParkedSet{}
	for _, p := range r.ParkedSets() {
		out = append(out, *parkedSet(&p))
	}
	return out
}

func parkingOf(o *project.ParkOutcome) *Parking {
	if o == nil {
		return nil
	}
	return &Parking{Parked: parkedSet(o.Parked), Restored: parkedSet(o.Restored), Merged: o.Merged, Waiting: parkedSet(o.Waiting)}
}

// BringParked brings changes parked at a place (branch, at: an older
// version, or "" for its latest) into the project where it is, merged with
// what's here; uncommitted. Conflicts come back as decisions (call again
// with resolutions); nothing changes until then.
func (a *App) BringParked(root, branch, at string, resolutions map[string]string, force bool) (*Result, error) {
	defer a.tidyLater(root)
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if set := liveGuard(r, force); set != "" {
		return blocked(set), nil
	}
	p := r.ParkedAt(branch, at)
	notes, err := r.BringParked(branch, at, opts(resolutions))
	if err != nil {
		return conflictResult(r, err)
	}
	return &Result{Action: "brought", Log: []string{}, Relinked: nonNil(notes), Conflicts: []Conflict{},
		Park: &Parking{Restored: parkedSet(p), Merged: true}}, nil
}

// DiscardParked drops changes parked at a place: they are gone.
func (a *App) DiscardParked(root, branch, at string) error {
	r, unlock, err := a.open(root)
	if err != nil {
		return err
	}
	defer unlock()
	return r.DiscardParked(branch, at)
}
