package remote

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/jsonx"
)

// Milestones: a version given a name ("Sent to the label, v1"), for the
// whole team, kept at projects/<pid>/milestones/<id>.json. A milestone only
// names a version: the version is kept for good (storage cleanup never
// deletes one), so a milestone always leads to it. Renaming or removing a
// milestone touches nothing else. Like branch records, milestones only add
// to what a team stores, and are built in Nightly only (BranchRecords).

// Milestone is a version given a name.
type Milestone struct {
	Version string    `json:"version"`
	Name    string    `json:"name"`
	Note    string    `json:"note,omitempty"`
	By      string    `json:"by,omitempty"` // member id
	Time    time.Time `json:"time"`
	// Extra: fields a newer R3V wrote, kept when this one rewrites the record.
	Extra jsonx.Extra `json:"-"`
}

func (v *Milestone) UnmarshalJSON(b []byte) error {
	type plain Milestone
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v Milestone) MarshalJSON() ([]byte, error) {
	type plain Milestone
	return jsonx.Encode(plain(v), v.Extra)
}

// MaxMilestoneNote is the most characters of a milestone's note.
const MaxMilestoneNote = 1000

// MilestoneStore is storage that keeps milestones.
type MilestoneStore interface {
	// Milestones maps project pid's milestone ids to milestones.
	Milestones(pid string) (map[string]Milestone, error)
	PutMilestone(pid, id string, m Milestone) error
	DeleteMilestone(pid, id string) error
}

// MilestonesOf returns b's milestone store, if its team keeps them (where
// it keeps branch records: the same channel and service).
func MilestonesOf(b Backend) (MilestoneStore, bool) {
	if _, ok := BranchRecordsOf(b); !ok {
		return nil, false
	}
	s, ok := b.(*BucketBackend)
	return s, ok
}

func milestoneKey(pid, id string) string { return projectDir(pid) + "milestones/" + id + ".json" }

func (s *BucketBackend) Milestones(pid string) (map[string]Milestone, error) {
	if !validHex(pid, 32) {
		return nil, errors.New("invalid project id")
	}
	got, err := s.readDir(projectDir(pid)+"milestones/", "")
	if err != nil {
		return nil, err
	}
	out := map[string]Milestone{}
	for k, data := range got {
		id := strings.TrimSuffix(path.Base(k), ".json")
		var m Milestone
		if !validHex(id, 32) || json.Unmarshal(data, &m) != nil || !validHex(m.Version, 64) {
			continue // one that can't be read isn't shown
		}
		out[id] = m
	}
	return out, nil
}

func (s *BucketBackend) PutMilestone(pid, id string, m Milestone) error {
	if !BranchRecords {
		return errors.New("milestones need R3V's Nightly channel")
	}
	if !validHex(pid, 32) || !validHex(id, 32) {
		return errors.New("invalid milestone")
	}
	data, _ := json.Marshal(m)
	return s.put(milestoneKey(pid, id), data)
}

func (s *BucketBackend) DeleteMilestone(pid, id string) error {
	if !BranchRecords {
		return errors.New("milestones need R3V's Nightly channel")
	}
	if !validHex(pid, 32) || !validHex(id, 32) {
		return errors.New("invalid milestone")
	}
	return s.delete(milestoneKey(pid, id))
}
