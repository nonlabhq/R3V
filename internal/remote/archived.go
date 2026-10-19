package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nonlabhq/r3v/internal/jsonx"
)

// Archived projects (docs/design/archive.md): a project put away for the
// whole team, hidden from its lists, never deleted (its versions and
// stored files stay). The team keeps one record of them, archived.json
// beside team.json:
//
//	{"projects": {"<project id>": {"by": "<member id>", "at": "<RFC3339>"}}}
//
// Not a field of the project's own record: an older R3V rewriting that
// would drop it. The record is written whole with a conditional write
// (if-match its etag, or if-none-match when there is none), read again and
// the change made again when someone else wrote it meanwhile: stopped
// half-way, it is as before or as after, never in between.
//
// An older R3V never reads it, and only still shows the project: no team
// feature. On an R3V Cloud team the service lets only owners and admins
// write it (ErrNotAdmin); on a team's own storage, anyone in the team.

// ProjectArchive: this build archives projects (every build since 0.2.2;
// tests turn it off to play an older R3V).
var ProjectArchive = true

// ErrArchiveNotInBuild: this build doesn't archive projects.
var ErrArchiveNotInBuild = errors.New("archiving projects needs a newer R3V")

// ErrNoArchive: the team's storage can't keep archived projects.
var ErrNoArchive = errors.New("this team can't archive projects")

const archivedKey = "archived.json"

// ArchivedProject is a project's archiving: who and when.
type ArchivedProject struct {
	By string    `json:"by,omitempty"` // member id
	At time.Time `json:"at"`
	// Extra: fields a newer R3V wrote, kept when this one rewrites the record.
	Extra jsonx.Extra `json:"-"`
}

func (v *ArchivedProject) UnmarshalJSON(b []byte) error {
	type plain ArchivedProject
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v ArchivedProject) MarshalJSON() ([]byte, error) {
	type plain ArchivedProject
	return jsonx.Encode(plain(v), v.Extra)
}

// archivedRecord is archived.json.
type archivedRecord struct {
	Projects map[string]ArchivedProject `json:"projects"`
	Extra    jsonx.Extra                `json:"-"`
}

func (v *archivedRecord) UnmarshalJSON(b []byte) error {
	type plain archivedRecord
	return jsonx.Decode(b, (*plain)(v), &v.Extra)
}

func (v archivedRecord) MarshalJSON() ([]byte, error) {
	type plain archivedRecord
	return jsonx.Encode(plain(v), v.Extra)
}

// readArchived reads the record and its etag ("" when there is none).
func (s *BucketBackend) readArchived() (archivedRecord, string, error) {
	var rec archivedRecord
	data, etag, err := s.b.Get(archivedKey)
	if errors.Is(err, ErrNotFound) {
		return archivedRecord{Projects: map[string]ArchivedProject{}}, "", nil
	}
	if err != nil {
		return rec, "", err
	}
	// One that doesn't read is an error, not an empty one: written back
	// empty, it would bring every archived project back.
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, "", fmt.Errorf("%s: %w", archivedKey, err)
	}
	if rec.Projects == nil {
		rec.Projects = map[string]ArchivedProject{}
	}
	return rec, etag, nil
}

// ArchivedProjects maps the team's archived projects' ids to who archived
// them and when (none in a build without archiving, or on storage that
// keeps none).
func ArchivedProjects(b Backend) (map[string]ArchivedProject, error) {
	s, ok := b.(*BucketBackend)
	if !ProjectArchive || !ok {
		return map[string]ArchivedProject{}, nil
	}
	rec, _, err := s.readArchived()
	return rec.Projects, err
}

// archiveTries is how often a write that met someone else's is made again.
const archiveTries = 8

// SetProjectArchived archives project pid for the whole team (on), or
// brings it back, and logs it in the team log; changed says whether the
// record changed (archiving an archived project leaves it as it was).
func SetProjectArchived(b Backend, pid string, on bool) (changed bool, err error) {
	if !ProjectArchive {
		return false, ErrArchiveNotInBuild
	}
	s, ok := b.(*BucketBackend)
	if !ok {
		return false, ErrNoArchive
	}
	if !validHex(pid, 32) {
		return false, errors.New("invalid project id")
	}
	ps, err := b.Projects()
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(ps, func(p Project) bool { return p.ID == pid })
	if on && i < 0 {
		return false, ErrNotFound
	}
	name := ""
	if i >= 0 {
		name = ps[i].Name
	}
	for try := 0; try < archiveTries; try++ {
		rec, etag, err := s.readArchived()
		if err != nil {
			return false, err
		}
		if _, there := rec.Projects[pid]; there == on {
			return false, nil
		}
		if on {
			rec.Projects[pid] = ArchivedProject{By: s.actor, At: branchLogNow().UTC().Truncate(time.Second)}
		} else {
			delete(rec.Projects, pid)
		}
		data, _ := json.Marshal(rec)
		cond := etag
		if cond == "" {
			cond = "*"
		}
		err = s.b.Put(archivedKey, bytes.NewReader(data), int64(len(data)), "", cond)
		if errors.Is(err, ErrPrecondition) {
			continue // someone else wrote it: read it again
		}
		if errors.Is(err, ErrForbidden) {
			return false, ErrNotAdmin
		}
		if err != nil {
			return false, err
		}
		kind := EventProjectUnarchived
		if on {
			kind = EventProjectArchived
		}
		s.logTeam(TeamEvent{Kind: kind, Project: pid, Name: name})
		return true, nil
	}
	return false, errors.New("the team's archived projects kept changing: try again")
}
