package desktop

import (
	"strconv"

	"github.com/nonlabhq/r3v/internal/project"
)

// SampleSpot is a sample the project's sets use that is missing or only in
// R3V's hidden .r3v folder (see project.SampleSpots).
type SampleSpot struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Size       int64    `json:"size"`
	Missing    bool     `json:"missing"`
	Kept       bool     `json:"kept"`       // in .r3v: gone with it
	Restorable bool     `json:"restorable"` // missing, and R3V has a copy
	Sets       []string `json:"sets"`
}

// SampleSpots lists the samples of the project's sets that are missing or
// only in .r3v. Reads only: no lock.
func (a *App) SampleSpots(root string) ([]SampleSpot, error) {
	r, err := project.Open(root)
	if err != nil {
		return nil, err
	}
	spots, err := r.SampleSpots()
	if err != nil {
		return nil, err
	}
	out := []SampleSpot{}
	for _, s := range spots {
		out = append(out, SampleSpot{Path: s.Path, Name: s.Name, Size: s.Size, Missing: s.Missing, Kept: s.Kept,
			Restorable: s.Restorable(), Sets: nonNil(s.Sets)})
	}
	return out, nil
}

// BringSamplesIn puts samples into the project (Samples/Imported, or back at
// their place) and points the sets at them: missing ones R3V has a copy
// of, and/or those only in .r3v. It rewrites sets, so not while one is
// open in Live (unless force). Result.Log holds how many.
func (a *App) BringSamplesIn(root string, missing, kept, force bool) (*Result, error) {
	r, unlock, err := a.open(root)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if set := liveGuard(r, force); set != "" {
		return blocked(set), nil
	}
	n, err := r.BringSamplesIn(missing, kept)
	if err != nil {
		return nil, err
	}
	out := syncResult(nil)
	out.Action, out.Log = "brought", []string{strconv.Itoa(n)}
	return out, nil
}
