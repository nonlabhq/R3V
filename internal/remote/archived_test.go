package remote_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
	"github.com/nonlabhq/r3v/internal/remote/membucket"
)

// Archived projects (docs/design/archive.md): one team-level record,
// written with a conditional write, the change made again over someone
// else's; a failed write changes nothing; on R3V Cloud only admins write.

func archiving(t *testing.T) {
	t.Helper()
	was := remote.ProjectArchive
	remote.ProjectArchive = true
	t.Cleanup(func() { remote.ProjectArchive = was })
}

const me = "0000000000000000000000000000000c"

func archiveTeam(t *testing.T, bk remote.Bucket) *remote.BucketBackend {
	t.Helper()
	b := remote.NewBucketBackend(bk)
	b.SetActor(me)
	for _, p := range []remote.Project{{ID: p1, Name: "Game"}, {ID: p2, Name: "Trailer"}} {
		if err := b.PutProject(p); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

func TestArchiveProjectRecord(t *testing.T) {
	archiving(t)
	mem := membucket.New()
	b := archiveTeam(t, mem)
	if got, err := remote.ArchivedProjects(b); err != nil || len(got) != 0 {
		t.Fatalf("before: %v %v", got, err)
	}
	if changed, err := remote.SetProjectArchived(b, p1, true); err != nil || !changed {
		t.Fatalf("archive: %v %v", changed, err)
	}
	if changed, err := remote.SetProjectArchived(b, p1, true); err != nil || changed {
		t.Fatalf("archived again: %v %v (nothing to change)", changed, err)
	}
	got, err := remote.ArchivedProjects(b)
	if err != nil || len(got) != 1 || got[p1].By != me || got[p1].At.IsZero() {
		t.Fatalf("after: %+v %v", got, err)
	}
	// Its shape, as other R3Vs and the service read it.
	data, _, _ := mem.Get("archived.json")
	var shape struct {
		Projects map[string]struct {
			By string `json:"by"`
			At string `json:"at"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &shape) != nil || shape.Projects[p1].By != me {
		t.Fatalf("archived.json: %s", data)
	}
	if _, err := time.Parse(time.RFC3339, shape.Projects[p1].At); err != nil {
		t.Errorf("at: %v", err)
	}
	// Nothing of the project changed: its record, its id still listed.
	if ps, _ := b.Projects(); len(ps) != 2 {
		t.Errorf("projects: %v", ps)
	}
	if _, err := remote.SetProjectArchived(b, "33333333333333333333333333333333", true); !errors.Is(err, remote.ErrNotFound) {
		t.Errorf("archiving no project: %v", err)
	}
	// What a newer R3V wrote is kept.
	newer := []byte(`{"projects":{"` + p1 + `":{"by":"` + me + `","at":"2026-10-01T00:00:00Z","why":"done"}},"note":1}`)
	mem.Put("archived.json", bytes.NewReader(newer), int64(len(newer)), "", "")
	if _, err := remote.SetProjectArchived(b, p2, true); err != nil {
		t.Fatal(err)
	}
	data, _, _ = mem.Get("archived.json")
	if !strings.Contains(string(data), `"why":"done"`) || !strings.Contains(string(data), `"note":1`) {
		t.Errorf("a newer R3V's fields dropped: %s", data)
	}
	if changed, err := remote.SetProjectArchived(b, p1, false); err != nil || !changed {
		t.Fatalf("unarchive: %v %v", changed, err)
	}
	if got, _ := remote.ArchivedProjects(b); len(got) != 1 || got[p2].By != me {
		t.Errorf("after unarchiving one: %+v", got)
	}
	// The team log has each.
	evs, _ := b.TeamLog(time.Time{})
	kinds := map[string]int{}
	for _, ev := range evs {
		kinds[ev.Kind]++
		if ev.By != me || ev.Project == "" || ev.Name == "" {
			t.Errorf("logged: %+v", ev)
		}
	}
	if kinds[remote.EventProjectArchived] != 2 || kinds[remote.EventProjectUnarchived] != 1 || len(evs) != 3 {
		t.Errorf("team log: %v", kinds)
	}
	// A record that doesn't read is an error, never written over.
	mem.Put("archived.json", strings.NewReader(`{"projects": broken`), 19, "", "")
	if _, err := remote.SetProjectArchived(b, p1, true); err == nil {
		t.Error("wrote over an unreadable archived.json")
	}
}

// racing writes someone else's change to archived.json just before this
// R3V's first write lands.
type racing struct {
	*membucket.Bucket
	raced bool
	fail  error
}

func (r *racing) Put(key string, body io.Reader, size int64, sum, cond string) error {
	if key == "archived.json" && r.fail != nil {
		return r.fail
	}
	if key == "archived.json" && !r.raced {
		r.raced = true
		other := `{"projects":{"` + p2 + `":{"by":"0000000000000000000000000000000d","at":"2026-10-01T00:00:00Z"}}}`
		r.Bucket.Put(key, strings.NewReader(other), int64(len(other)), "", "")
	}
	return r.Bucket.Put(key, body, size, sum, cond)
}

func TestArchiveConflictReapplies(t *testing.T) {
	archiving(t)
	bk := &racing{Bucket: membucket.New()}
	b := archiveTeam(t, bk)
	if _, err := remote.SetProjectArchived(b, p1, true); err != nil {
		t.Fatal(err)
	}
	got, _ := remote.ArchivedProjects(b)
	if len(got) != 2 || got[p1].By != me || got[p2].By == "" {
		t.Errorf("after a conflict: %+v (both)", got)
	}
}

func TestArchiveFailedWriteChangesNothing(t *testing.T) {
	archiving(t)
	bk := &racing{Bucket: membucket.New(), raced: true}
	b := archiveTeam(t, bk)
	bk.fail = errors.New("connection lost")
	if _, err := remote.SetProjectArchived(b, p1, true); err == nil {
		t.Fatal("no error")
	}
	if got, _ := remote.ArchivedProjects(b); len(got) != 0 {
		t.Errorf("archived after a failed write: %+v", got)
	}
	if evs, _ := b.TeamLog(time.Time{}); len(evs) != 0 {
		t.Errorf("logged a failed write: %+v", evs)
	}
}

func TestArchiveNotInBuild(t *testing.T) {
	was := remote.ProjectArchive
	remote.ProjectArchive = false
	defer func() { remote.ProjectArchive = was }()
	b := archiveTeam(t, membucket.New())
	if _, err := remote.SetProjectArchived(b, p1, true); !errors.Is(err, remote.ErrArchiveNotInBuild) {
		t.Errorf("archiving without it: %v", err)
	}
}

// On an R3V Cloud team the service lets only admins write the record.
func TestArchiveOnCloudAdminsOnly(t *testing.T) {
	archiving(t)
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	f := cloudtest.NewFake(t)
	const kai = "0000000000000000000000000000000b"
	f.AddMember("kai", kai, false)
	open := func(token string) *remote.BucketBackend {
		t.Setenv("R3V_CLOUD_TOKEN", token)
		c, err := remote.Open(remote.Config{URL: f.Address})
		if err != nil {
			t.Fatal(err)
		}
		return c.(*remote.BucketBackend)
	}
	owner := open("token")
	owner.SetActor(cloudtest.Owner)
	if err := owner.PutProject(remote.Project{ID: p1, Name: "Game"}); err != nil {
		t.Fatal(err)
	}
	member := open("kai")
	if _, err := remote.SetProjectArchived(member, p1, true); !errors.Is(err, remote.ErrNotAdmin) {
		t.Fatalf("a member archiving: %v", err)
	}
	owner = open("token")
	owner.SetActor(cloudtest.Owner)
	if _, err := remote.SetProjectArchived(owner, p1, true); err != nil {
		t.Fatal(err)
	}
	if got, err := remote.ArchivedProjects(open("kai")); err != nil || got[p1].By != cloudtest.Owner {
		t.Errorf("a member reads: %+v %v", got, err)
	}
	// Brought back over the record it wrote (if-match its etag).
	owner = open("token")
	if _, err := remote.SetProjectArchived(owner, p1, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := remote.ArchivedProjects(owner); len(got) != 0 {
		t.Errorf("after unarchiving: %+v", got)
	}
}
