package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/store"
)

// Parked changes (docs/design/parking.md): going to another branch or
// version with uncommitted changes keeps them with the place they were made
// (a branch's latest version, or an older version of it), and coming back
// brings them back. They stay on this computer: a version never shared,
// named by a record in .r3v/parked.

// Parking: switches keep uncommitted changes instead of refusing (the
// Nightly channel while it's new; parking_nightly.go). Off, a switch with
// changes fails with ErrDirty, as before.
var Parking = false

const parkedDir = "parked"

// arrivedFile names a parked set being brought back by a switch: its record
// goes once the switch is done (tidyParked).
const arrivedFile = "arrived"

// Parked is a parked set: uncommitted changes kept while the project is
// elsewhere.
type Parked struct {
	// Version holds the files as they were (never shared); Base is the
	// version they were made on.
	Version string `json:"version"`
	Base    string `json:"base"`
	// Branch is the branch's key; At the older version they were made on
	// ("" for the branch's latest).
	Branch string `json:"branch"`
	At     string `json:"at,omitempty"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"` // new content kept for them
	Since  string `json:"since"`
}

// ParkOutcome is what a switch did with uncommitted changes.
type ParkOutcome struct {
	Parked   *Parked // the changes left behind, kept
	Restored *Parked // the place's own changes, back in the folder
	Merged   bool    // brought back merged with the versions since
	// Waiting: the place's changes couldn't be merged with the versions
	// since without choosing: still parked.
	Waiting *Parked
}

// ErrParkedHere: leaving a place with changes when changes are parked there
// already (they came back with a conflict, and are still waiting).
var ErrParkedHere = errors.New("changes are parked here already: bring them back or discard them first")

func parkedFile(branch, at string) string {
	if at == "" {
		return branch + ".json"
	}
	return branch + "@" + at + ".json"
}

func (r *Repo) parkedPath(branch, at string) string {
	return filepath.Join(r.Dir, parkedDir, parkedFile(branch, at))
}

// ParkedSets lists the parked sets, oldest first.
func (r *Repo) ParkedSets() []Parked {
	entries, err := os.ReadDir(filepath.Join(r.Dir, parkedDir))
	if err != nil {
		return nil
	}
	var out []Parked
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.Dir, parkedDir, e.Name()))
		if err != nil {
			continue
		}
		var p Parked
		if json.Unmarshal(data, &p) != nil || p.Version == "" || !r.HasSnapshot(p.Version) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since < out[j].Since })
	return out
}

// ParkedAt is the set parked at a place, if any.
func (r *Repo) ParkedAt(branch, at string) *Parked {
	for _, p := range r.ParkedSets() {
		if p.Branch == branch && p.At == at {
			return &p
		}
	}
	return nil
}

func (r *Repo) writeParked(p Parked) error {
	if err := os.MkdirAll(filepath.Join(r.Dir, parkedDir), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteAtomic(r.parkedPath(p.Branch, p.At), bytes.NewReader(append(data, '\n')))
}

func (r *Repo) dropParked(p Parked) { os.Remove(r.parkedPath(p.Branch, p.At)) }

// parkedIDs are the versions parked sets hold: local cleanup keeps them.
func (r *Repo) parkedIDs() map[string]bool {
	ids := map[string]bool{}
	for _, p := range r.ParkedSets() {
		ids[p.Version] = true
	}
	return ids
}

// parkedHashes is the content parked sets hold that their bases don't:
// kept here (PruneObjects leaves it), whatever the team's storage has.
func (r *Repo) parkedHashes() map[string]bool {
	out := map[string]bool{}
	for _, p := range r.ParkedSets() {
		m, err := r.Load(p.Version)
		if err != nil {
			continue
		}
		known := map[string]bool{}
		if b, err := r.Load(p.Base); err == nil {
			for _, f := range b.Files {
				known[f.Hash] = true
			}
		}
		for _, f := range m.Files {
			if !known[f.Hash] {
				out[f.Hash] = true
			}
		}
	}
	return out
}

// place is where the project is: its branch, and the older version it's on
// ("" on the latest).
func (r *Repo) place() (branch, at string) {
	if r.OnOlderVersion() {
		return r.BranchName(), r.Head()
	}
	return r.BranchName(), ""
}

// parkChanges keeps the uncommitted changes as a parked set at the place
// the project is (nil when there are none). The project folder is left as
// it is: the switch that follows replaces it.
func (r *Repo) parkChanges() (*Parked, error) {
	branch, at := r.place()
	only := r.Only
	r.Only = nil // all of them, whatever a commit would pick
	m, ix, err := r.workingManifest("Parked changes")
	r.Only = only
	if errors.Is(err, ErrNothingToSnapshot) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if r.ParkedAt(branch, at) != nil {
		return nil, ErrParkedHere
	}
	base := r.Head()
	known := map[string]FileEntry{}
	if base != "" {
		b, err := r.Load(base)
		if err != nil {
			return nil, err
		}
		for _, f := range b.Files {
			known[f.Path] = f
		}
	}
	// Every new content is kept here, even what the team's storage has (a
	// preupload no version names yet may be cleaned up there).
	hashes := map[string]bool{}
	for _, f := range known {
		hashes[f.Hash] = true
	}
	p := Parked{Base: base, Branch: branch, At: at, Since: time.Now().UTC().Format(time.RFC3339)}
	for _, f := range m.Files {
		if k, ok := known[f.Path]; !ok || k.Hash != f.Hash {
			p.Files++
		}
		delete(known, f.Path)
		if hashes[f.Hash] {
			continue
		}
		hashes[f.Hash] = true
		p.Bytes += f.Size
		if !r.Store.Has(f.Hash) {
			if h, _, err := r.storeFile(r.Abs(f.Path)); err != nil {
				return nil, fmt.Errorf("%s: %w", f.Path, err)
			} else if h != f.Hash {
				return nil, fmt.Errorf("%s changed while being parked: try again", f.Path)
			}
		}
	}
	p.Files += len(known) // deleted
	if err := r.save(m); err != nil {
		return nil, err
	}
	if err := ix.save(); err != nil {
		return nil, err
	}
	p.Version = m.ID
	if err := r.writeParked(p); err != nil {
		return nil, err
	}
	return &p, nil
}

// arriving is the files to put in place for a version the project goes to
// (target), with the set parked there brought back: as it is when made on
// target, else merged with the versions since. waiting when that merge
// needs choices: the set stays parked and target's own files are put.
func (r *Repo) arriving(p *Parked, target *Manifest) (files *Manifest, merged, waiting bool, err error) {
	set, err := r.Load(p.Version)
	if err != nil {
		return nil, false, false, err
	}
	if p.Base == target.ID {
		return set, false, false, nil
	}
	base, err := r.Load(p.Base)
	if err != nil {
		return nil, false, false, err
	}
	r.knowSizes(base, target)
	if err := r.ensureHashes(setHashes(base, target, set)); err != nil {
		return nil, false, false, err
	}
	m, _, err := r.mergeManifests(base, set, target, Strategy("fail"))
	var mc *MergeConflictError
	if errors.As(err, &mc) {
		return target, false, true, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	m.Parents, m.Message = []string{target.ID}, "Parked changes with the versions since merged in"
	if err := r.save(m); err != nil {
		return nil, false, false, err
	}
	return m, true, false, nil
}

// switchKeeping puts the project on version id at the place (branch, at),
// the changes here parked and the place's own brought back. head is the
// version the project is on afterwards (id).
func (r *Repo) switchKeeping(target *Manifest, branch, at string) (*ParkOutcome, []string, error) {
	r.tidyParked()
	out := &ParkOutcome{}
	files := target
	back := r.ParkedAt(branch, at)
	if back != nil {
		m, merged, waiting, err := r.arriving(back, target)
		if err != nil {
			return nil, nil, err
		}
		if waiting {
			out.Waiting, back = back, nil
		} else {
			files, out.Merged = m, merged
		}
	}
	parked, err := r.parkChanges()
	if err != nil {
		return nil, nil, err
	}
	out.Parked = parked
	// Stopped halfway, the files go back as they were: the changes, from
	// their parked set (whose record RecoverSwitch then drops).
	marker := target.ID
	if parked != nil {
		marker = "work " + parked.Version
	}
	if back != nil {
		os.WriteFile(filepath.Join(r.Dir, parkedDir, arrivedFile), []byte(parkedFile(back.Branch, back.At)+" "+target.ID+"\n"), 0o644)
	}
	notes, err := r.putFiles(files, target.ID, marker)
	if err != nil {
		return nil, nil, err
	}
	if back != nil {
		keep := []string{back.Version}
		if out.Merged {
			keep = append(keep, files.ID)
		}
		r.keepWork(keep...)
		out.Restored = back
	}
	return out, notes, nil
}

// arrived finishes bringing a set back, once the project is where it went:
// its record goes (its changes are in the folder). Called when the switch's
// place is saved (the branch, an older version).
func (r *Repo) arrived() {
	path := filepath.Join(r.Dir, parkedDir, arrivedFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if f := strings.Fields(string(data)); len(f) == 2 && f[1] == r.Head() && r.UnfinishedSwitch() == "" {
		os.Remove(filepath.Join(r.Dir, parkedDir, f[0]))
	}
	os.Remove(path)
}

// tidyParked finishes what a switch left (nothing while one is unfinished:
// RecoverSwitch first): a set brought back whose record is still there
// (stopped before arrived), and a set parked here whose changes are still
// in the folder (stopped before the files changed).
func (r *Repo) tidyParked() {
	if r.UnfinishedSwitch() != "" {
		return
	}
	r.arrived()
	p := r.ParkedAt(r.place())
	if p == nil || p.Base != r.Head() {
		return
	}
	set, err := r.Load(p.Version)
	if err != nil {
		return
	}
	ix := r.loadIndex()
	files, err := r.workingFiles(ix)
	if err != nil || len(files) != len(set.Files) {
		return
	}
	want := set.FileMap()
	for _, f := range files {
		if want[f.Path].Hash != f.Hash {
			return // a set waiting here (it needed choices), or changed since
		}
	}
	r.dropParked(*p)
}

// TidyParked: see tidyParked (the app, on reading a project).
func (r *Repo) TidyParked() { r.tidyParked() }

// keepWork keeps versions that stand for the uncommitted changes (the
// working files' contents, relinked sets mapping to them) until other work
// is kept: only the last kept work stays. Parked sets' versions stay.
func (r *Repo) keepWork(ids ...string) {
	keep := strings.Join(ids, "\n") + "\n"
	parked := r.parkedIDs()
	if old, err := os.ReadFile(filepath.Join(r.Dir, keptWorkFile)); err == nil {
		for _, id := range strings.Fields(string(old)) {
			if !strings.Contains(keep, id) && !parked[id] {
				os.Remove(r.snapshotPath(id))
			}
		}
	}
	os.WriteFile(filepath.Join(r.Dir, keptWorkFile), []byte(keep), 0o644)
}

// BringParked brings a parked set into the project where it is (changes
// made on the wrong branch, a set whose branch is gone): merged with the
// files here, uncommitted changes and all. Its record goes.
func (r *Repo) BringParked(branch, at string, opts MergeOptions) ([]string, error) {
	p := r.ParkedAt(branch, at)
	if p == nil {
		return nil, errors.New("nothing is parked there")
	}
	r.tidyParked()
	set, err := r.Load(p.Version)
	if err != nil {
		return nil, err
	}
	base, err := r.Load(p.Base)
	if err != nil {
		return nil, err
	}
	only := r.Only
	r.Only = nil
	work, ix, err := r.workingManifest("Uncommitted work, kept while bringing parked changes")
	r.Only = only
	var keep []string // never a committed version: kept work goes in time
	if errors.Is(err, ErrNothingToSnapshot) {
		work, err = r.Load(r.Head())
	} else if err == nil {
		if err = r.save(work); err == nil {
			err = ix.save()
			keep = append(keep, work.ID)
		}
	}
	if err != nil {
		return nil, err
	}
	r.knowSizes(base, set, work)
	if err := r.ensureHashes(setHashes(base, set, work)); err != nil {
		return nil, err
	}
	merged, _, err := r.mergeManifests(base, work, set, opts)
	if err != nil { // nothing changed: the work is where it was
		if len(keep) > 0 {
			os.Remove(r.snapshotPath(work.ID))
		}
		var mc *MergeConflictError
		if errors.As(err, &mc) {
			mc.Work = true
		}
		return nil, err
	}
	merged.Parents, merged.Message = []string{r.Head()}, "Uncommitted work with parked changes brought in"
	if err := r.save(merged); err != nil {
		return nil, err
	}
	// Stopped halfway, the files go back as they were, the set still parked.
	notes, err := r.putFiles(merged, r.Head(), "work "+work.ID)
	if err != nil {
		return nil, err
	}
	r.dropParked(*p)
	r.keepWork(append(keep, merged.ID)...)
	os.Remove(r.snapshotPath(p.Version))
	return notes, nil
}

// DiscardParked drops a parked set: its changes are gone.
func (r *Repo) DiscardParked(branch, at string) error {
	p := r.ParkedAt(branch, at)
	if p == nil {
		return errors.New("nothing is parked there")
	}
	r.dropParked(*p)
	os.Remove(r.snapshotPath(p.Version))
	return nil
}
