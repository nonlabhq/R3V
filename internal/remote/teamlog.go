package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

// The team log (docs/design/team-settings.md, Activity): what happened to
// the team that its storage keeps no record of otherwise: a project added
// or removed, a member joining. One key per event
// (teamlog/<time>-<random>.json), never changed, written after the step
// succeeded, as well as can be (as the branch log is). An older R3V writes
// none, and never reads them: no team feature.

// Team log kinds.
const (
	EventProjectAdded   = "project-added"
	EventProjectRemoved = "project-removed"
	EventMemberJoined   = "member-joined"
	// A project archived for the team, or brought back (archived.go).
	EventProjectArchived   = "project-archived"
	EventProjectUnarchived = "project-unarchived"
	// The team's sections changed (sections.go): Change says how.
	EventSectionsChanged = "sections-changed"
)

// TeamEvent is one thing that happened to the team.
type TeamEvent struct {
	Kind    string    `json:"kind"`
	Project string    `json:"project,omitempty"` // its id
	Name    string    `json:"name,omitempty"`    // the project's or member's name then
	Member  string    `json:"member,omitempty"`  // the member who joined
	By      string    `json:"by,omitempty"`      // who did it (a member id)
	Time    time.Time `json:"time"`
	// Section and Change: for sections-changed, the section's name (then)
	// and what happened to it (SectionMade, ...); Name is the project's,
	// or a renamed section's name before.
	Section string `json:"section,omitempty"`
	Change  string `json:"change,omitempty"`
}

// TeamLogger is a backend that keeps the team log.
type TeamLogger interface {
	// TeamLog lists the team's events since a time, oldest first.
	TeamLog(since time.Time) ([]TeamEvent, error)
}

var _ TeamLogger = (*BucketBackend)(nil)

const teamLogDir = "teamlog/"

// logKeyTime names a log record by its time, so names sort by it.
const logKeyTime = "20060102T150405.000000000Z"

// LogTeamEvent adds ev to the team log of b, when b keeps one (best effort).
func LogTeamEvent(b Backend, ev TeamEvent) {
	if s, ok := b.(*BucketBackend); ok {
		s.logTeam(ev)
	}
}

func (s *BucketBackend) logTeam(ev TeamEvent) {
	if ev.Time.IsZero() {
		ev.Time = branchLogNow().UTC()
	}
	if ev.By == "" {
		ev.By = s.actor
	}
	if ev.By == "" {
		return // (no one to name: this computer isn't a member yet)
	}
	id := make([]byte, 4)
	rand.Read(id)
	data, _ := json.Marshal(ev)
	// (created, never overwritten: an R3V Cloud team takes only that, and
	// only by the member it names)
	s.b.Put(teamLogDir+ev.Time.UTC().Format(logKeyTime)+"-"+hex.EncodeToString(id)+".json", bytes.NewReader(data), int64(len(data)), "", "*")
}

func (s *BucketBackend) TeamLog(since time.Time) ([]TeamEvent, error) {
	var out []TeamEvent
	err := s.readSince(teamLogDir, since, func(data []byte) {
		var ev TeamEvent
		if json.Unmarshal(data, &ev) == nil && ev.Kind != "" {
			out = append(out, ev)
		}
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, err
}

// BranchLogSince lists a project's branch moves since a time, oldest first
// (only the records named after it are read).
func (s *BucketBackend) BranchLogSince(pid string, since time.Time) ([]BranchMove, error) {
	if !validHex(pid, 32) {
		return nil, errors.New("invalid project id")
	}
	var out []BranchMove
	err := s.readSince(branchLogDir(pid), since, func(data []byte) {
		var m BranchMove
		if json.Unmarshal(data, &m) == nil {
			out = append(out, m)
		}
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, err
}

// readSince reads the log records in dir named at since or after.
func (s *BucketBackend) readSince(dir string, since time.Time, fn func(data []byte)) error {
	keys, err := s.list(dir)
	if err != nil {
		return err
	}
	from := dir + since.UTC().Format(logKeyTime)
	var recent []string
	for _, k := range keys {
		if strings.HasSuffix(k, ".json") && k >= from {
			recent = append(recent, k)
		}
	}
	got := make([][]byte, len(recent))
	idx := map[string]int{}
	for i, k := range recent {
		idx[k] = i
	}
	err = parallel(recent, func(key string) error {
		data, err := s.get(key)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		got[idx[key]] = data
		return nil
	})
	for _, d := range got {
		if d != nil {
			fn(d)
		}
	}
	return err
}
