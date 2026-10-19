package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nonlabhq/r3v/internal/jsonx"
)

// Sections (docs/design/sections.md): one level of named groups of a
// team's projects, a project in at most one, the rest ungrouped. The team
// keeps them in one record, sections.json beside team.json:
//
//	{"sections": [{"id": "<16 hex>", "name": "Engine", "projects": ["<project id>", ...]}]}
//
// The order of the sections, and of the projects inside one, is each
// person's own (kept on their computer); the record's order is only where
// a section someone hasn't placed yet goes (new ones last). Not a
// field of the project's record: an older R3V rewriting that would drop
// it. Written whole with a conditional write, the change made again over
// someone else's (as archived.json). An older R3V never reads it and only
// shows the projects ungrouped: no team feature. On an R3V Cloud team the
// service lets only owners and admins write it (ErrNotAdmin), and gives
// the others a copy without the projects they can't reach.

// ProjectSections: this build groups projects in sections (every build
// since 0.2.2; tests turn it off to play an older R3V).
var ProjectSections = true

// ErrSectionsNotInBuild: this build has no sections.
var ErrSectionsNotInBuild = errors.New("sections need a newer R3V")

// ErrNoSections: the team's storage can't keep sections.
var ErrNoSections = errors.New("this team can't keep sections")

// ErrNoSection: no section of that name or id.
var ErrNoSection = errors.New("no such section")

// ErrSectionExists: another section has that name.
var ErrSectionExists = errors.New("a section of that name is there already")

const sectionsKey = "sections.json"

// Section changes, as the team log names them (TeamEvent.Change).
const (
	SectionMade       = "made"
	SectionRenamed    = "renamed"
	SectionDeleted    = "deleted"
	SectionMoved      = "moved"       // its place among the sections (no longer written: the order is each person's; still read)
	SectionProjectIn  = "project-in"  // a project put in it
	SectionProjectOut = "project-out" // a project taken out, ungrouped
)

// Section is one of the team's sections.
type Section struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"` // project ids, in no particular order
	// Extra: fields a newer R3V wrote (a look, say), kept on rewrites.
	Extra jsonx.Extra `json:"-"`
}

func (v *Section) UnmarshalJSON(b []byte) error {
	type plain Section
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v Section) MarshalJSON() ([]byte, error) {
	type plain Section
	if v.Projects == nil {
		v.Projects = []string{}
	}
	return jsonx.Encode(plain(v), v.Extra)
}

// sectionsRecord is sections.json.
type sectionsRecord struct {
	Sections []Section   `json:"sections"`
	Extra    jsonx.Extra `json:"-"`
}

func (v *sectionsRecord) UnmarshalJSON(b []byte) error {
	type plain sectionsRecord
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v sectionsRecord) MarshalJSON() ([]byte, error) {
	type plain sectionsRecord
	if v.Sections == nil {
		v.Sections = []Section{}
	}
	return jsonx.Encode(plain(v), v.Extra)
}

// readSections reads the record and its etag ("" when there is none). A
// project listed twice (two writes crossing, or another tool) stays in
// the first section only.
func (s *BucketBackend) readSections() (sectionsRecord, string, error) {
	var rec sectionsRecord
	data, etag, err := s.b.Get(sectionsKey)
	if errors.Is(err, ErrNotFound) {
		return rec, "", nil
	}
	if err != nil {
		return rec, "", err
	}
	// One that doesn't read is an error, never written over: that would
	// lose every section.
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, "", fmt.Errorf("%s: %w", sectionsKey, err)
	}
	seen := map[string]bool{}
	for i := range rec.Sections {
		rec.Sections[i].Projects = slices.DeleteFunc(rec.Sections[i].Projects, func(id string) bool {
			if seen[id] {
				return true
			}
			seen[id] = true
			return false
		})
	}
	return rec, etag, nil
}

// Sections lists the team's sections in order (none in a build without
// them, or on storage that keeps none).
func Sections(b Backend) ([]Section, error) {
	s, ok := b.(*BucketBackend)
	if !ProjectSections || !ok {
		return []Section{}, nil
	}
	rec, _, err := s.readSections()
	if rec.Sections == nil {
		rec.Sections = []Section{}
	}
	return rec.Sections, err
}

// FindSection finds a section by its id, or by its name (case aside).
func FindSection(ss []Section, key string) int {
	if i := slices.IndexFunc(ss, func(x Section) bool { return x.ID == key }); i >= 0 {
		return i
	}
	return slices.IndexFunc(ss, func(x Section) bool { return strings.EqualFold(x.Name, strings.TrimSpace(key)) })
}

// SectionOf gives the id of the section project pid is in ("" for none).
func SectionOf(ss []Section, pid string) string {
	for _, x := range ss {
		if slices.Contains(x.Projects, pid) {
			return x.ID
		}
	}
	return ""
}

// CleanSectionName trims a section's name and checks it: 1 to 60
// characters, no control characters.
func CleanSectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 60 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("a section's name needs 1 to 60 characters")
	}
	return name, nil
}

// sectionTries is how often a write that met someone else's is made again.
const sectionTries = 8

// editSections makes change on the record as read and writes it with a
// conditional write; when someone else wrote it meanwhile it's read again
// and change made again. change gives the team log's event, or nil when
// there's nothing to change.
func editSections(b Backend, change func(rec *sectionsRecord) (*TeamEvent, error)) (bool, error) {
	if !ProjectSections {
		return false, ErrSectionsNotInBuild
	}
	s, ok := b.(*BucketBackend)
	if !ok {
		return false, ErrNoSections
	}
	for try := 0; try < sectionTries; try++ {
		rec, etag, err := s.readSections()
		if err != nil {
			return false, err
		}
		ev, err := change(&rec)
		if err != nil || ev == nil {
			return false, err
		}
		data, _ := json.Marshal(rec)
		cond := etag
		if cond == "" {
			cond = "*"
		}
		err = s.b.Put(sectionsKey, bytes.NewReader(data), int64(len(data)), "", cond)
		if errors.Is(err, ErrPrecondition) {
			continue // someone else wrote it: read it again
		}
		if errors.Is(err, ErrForbidden) {
			return false, ErrNotAdmin
		}
		if err != nil {
			return false, err
		}
		ev.Kind = EventSectionsChanged
		s.logTeam(*ev)
		return true, nil
	}
	return false, errors.New("the team's sections kept changing: try again")
}

func nameTaken(ss []Section, name, but string) bool {
	return slices.ContainsFunc(ss, func(x Section) bool { return x.ID != but && strings.EqualFold(x.Name, name) })
}

// NewSection adds a section at the end and returns it.
func NewSection(b Backend, name string) (Section, error) {
	name, err := CleanSectionName(name)
	if err != nil {
		return Section{}, err
	}
	id := make([]byte, 8)
	rand.Read(id)
	made := Section{ID: hex.EncodeToString(id), Name: name, Projects: []string{}}
	_, err = editSections(b, func(rec *sectionsRecord) (*TeamEvent, error) {
		if nameTaken(rec.Sections, name, "") {
			return nil, ErrSectionExists
		}
		rec.Sections = append(rec.Sections, made)
		return &TeamEvent{Change: SectionMade, Section: name}, nil
	})
	if err != nil {
		return Section{}, err
	}
	return made, nil
}

// RenameSection renames section id.
func RenameSection(b Backend, id, name string) (bool, error) {
	name, err := CleanSectionName(name)
	if err != nil {
		return false, err
	}
	return editSections(b, func(rec *sectionsRecord) (*TeamEvent, error) {
		i := slices.IndexFunc(rec.Sections, func(x Section) bool { return x.ID == id })
		if i < 0 {
			return nil, ErrNoSection
		}
		if nameTaken(rec.Sections, name, id) {
			return nil, ErrSectionExists
		}
		was := rec.Sections[i].Name
		if was == name {
			return nil, nil
		}
		rec.Sections[i].Name = name
		return &TeamEvent{Change: SectionRenamed, Section: name, Name: was}, nil
	})
}

// DeleteSection removes section id; its projects become ungrouped (nothing
// else of them changes).
func DeleteSection(b Backend, id string) (bool, error) {
	return editSections(b, func(rec *sectionsRecord) (*TeamEvent, error) {
		i := slices.IndexFunc(rec.Sections, func(x Section) bool { return x.ID == id })
		if i < 0 {
			return nil, nil // (gone already)
		}
		name := rec.Sections[i].Name
		rec.Sections = slices.Delete(rec.Sections, i, i+1)
		return &TeamEvent{Change: SectionDeleted, Section: name}, nil
	})
}

// SetProjectSection puts project pid in section id, out of any other, or
// ungrouped (id "").
func SetProjectSection(b Backend, pid, id string) (bool, error) {
	if !ProjectSections {
		return false, ErrSectionsNotInBuild
	}
	if !validHex(pid, 32) {
		return false, errors.New("invalid project id")
	}
	ps, err := b.Projects()
	if err != nil {
		return false, err
	}
	at := slices.IndexFunc(ps, func(p Project) bool { return p.ID == pid })
	if id != "" && at < 0 {
		return false, ErrNotFound
	}
	name := ""
	if at >= 0 {
		name = ps[at].Name
	}
	return editSections(b, func(rec *sectionsRecord) (*TeamEvent, error) {
		to := -1
		if id != "" {
			if to = slices.IndexFunc(rec.Sections, func(x Section) bool { return x.ID == id }); to < 0 {
				return nil, ErrNoSection
			}
		}
		from := slices.IndexFunc(rec.Sections, func(x Section) bool { return slices.Contains(x.Projects, pid) })
		if from == to {
			return nil, nil
		}
		ev := &TeamEvent{Project: pid, Name: name}
		if from >= 0 {
			rec.Sections[from].Projects = slices.DeleteFunc(rec.Sections[from].Projects, func(x string) bool { return x == pid })
			ev.Change, ev.Section = SectionProjectOut, rec.Sections[from].Name
		}
		if to >= 0 {
			rec.Sections[to].Projects = append(rec.Sections[to].Projects, pid)
			ev.Change, ev.Section = SectionProjectIn, rec.Sections[to].Name
		}
		return ev, nil
	})
}
