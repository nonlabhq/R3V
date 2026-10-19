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

// Sections (docs/design/sections.md): one team-level record, in order, a
// project in at most one; written with a conditional write, the change
// made again over someone else's; unknown fields kept; on R3V Cloud only
// admins write.

func sectionsOn(t *testing.T) {
	t.Helper()
	was := remote.ProjectSections
	remote.ProjectSections = true
	t.Cleanup(func() { remote.ProjectSections = was })
}

func names(ss []remote.Section) string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Name+":"+strings.Join(s.Projects, ","))
	}
	return strings.Join(out, " ")
}

func TestSectionsRecord(t *testing.T) {
	sectionsOn(t)
	mem := membucket.New()
	b := archiveTeam(t, mem)
	if got, err := remote.Sections(b); err != nil || len(got) != 0 {
		t.Fatalf("before: %v %v", got, err)
	}
	engine, err := remote.NewSection(b, "  Engine ")
	if err != nil || engine.Name != "Engine" || len(engine.ID) != 16 {
		t.Fatalf("new: %+v %v", engine, err)
	}
	tools, _ := remote.NewSection(b, "Tools")
	if _, err := remote.NewSection(b, "engine"); !errors.Is(err, remote.ErrSectionExists) {
		t.Errorf("a second Engine: %v", err)
	}
	if _, err := remote.NewSection(b, " "); err == nil {
		t.Error("an empty name")
	}
	if changed, err := remote.SetProjectSection(b, p1, engine.ID); err != nil || !changed {
		t.Fatalf("put in: %v %v", changed, err)
	}
	if changed, _ := remote.SetProjectSection(b, p1, engine.ID); changed {
		t.Error("put in twice: changed")
	}
	// One section at most: moved, not copied.
	remote.SetProjectSection(b, p1, tools.ID)
	remote.SetProjectSection(b, p2, tools.ID)
	got, _ := remote.Sections(b)
	if names(got) != "Engine: Tools:"+p1+","+p2 {
		t.Fatalf("after moving: %s", names(got))
	}
	if remote.SectionOf(got, p2) != tools.ID || remote.FindSection(got, "TOOLS") != 1 || remote.FindSection(got, engine.ID) != 0 {
		t.Error("finding")
	}
	if _, err := remote.SetProjectSection(b, "33333333333333333333333333333333", tools.ID); !errors.Is(err, remote.ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
	if _, err := remote.SetProjectSection(b, p1, "ffffffffffffffff"); !errors.Is(err, remote.ErrNoSection) {
		t.Errorf("no such section: %v", err)
	}
	// Order: team-wide, the record's.
	if changed, err := remote.MoveSection(b, tools.ID, 0); err != nil || !changed {
		t.Fatalf("move: %v %v", changed, err)
	}
	if _, err := remote.RenameSection(b, engine.ID, "Tools"); !errors.Is(err, remote.ErrSectionExists) {
		t.Errorf("renamed onto another: %v", err)
	}
	if _, err := remote.RenameSection(b, engine.ID, "Game"); err != nil {
		t.Fatal(err)
	}
	remote.SetProjectSection(b, p1, "")
	got, _ = remote.Sections(b)
	if names(got) != "Tools:"+p2+" Game:" {
		t.Fatalf("after: %s", names(got))
	}
	// Its shape, as other R3Vs and the service read it.
	data, _, _ := mem.Get("sections.json")
	var shape struct {
		Sections []struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			Projects []string `json:"projects"`
		} `json:"sections"`
	}
	if json.Unmarshal(data, &shape) != nil || len(shape.Sections) != 2 || shape.Sections[1].Projects == nil {
		t.Fatalf("sections.json: %s", data)
	}
	// Deleted: its projects ungrouped, the projects themselves untouched.
	if changed, err := remote.DeleteSection(b, tools.ID); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if got, _ := remote.Sections(b); names(got) != "Game:" || remote.SectionOf(got, p2) != "" {
		t.Errorf("after deleting: %s", names(got))
	}
	if ps, _ := b.Projects(); len(ps) != 2 {
		t.Errorf("projects: %v", ps)
	}
	// The team log has each change.
	evs, _ := b.TeamLog(time.Time{})
	changes := map[string]int{}
	for _, ev := range evs {
		if ev.Kind != remote.EventSectionsChanged || ev.By != me || ev.Section == "" {
			t.Errorf("logged: %+v", ev)
		}
		changes[ev.Change]++
	}
	want := map[string]int{remote.SectionMade: 2, remote.SectionProjectIn: 3, remote.SectionMoved: 1, remote.SectionRenamed: 1,
		remote.SectionProjectOut: 1, remote.SectionDeleted: 1}
	for k, n := range want {
		if changes[k] != n {
			t.Errorf("team log: %v, want %v", changes, want)
			break
		}
	}
	// A record that doesn't read is an error, never written over.
	mem.Put("sections.json", strings.NewReader(`{"sections": [`), 14, "", "")
	if _, err := remote.NewSection(b, "Other"); err == nil {
		t.Error("wrote over an unreadable sections.json")
	}
}

// What a newer R3V wrote is kept, on both levels; a project listed twice
// counts in the first section only.
func TestSectionsKeepUnknownFields(t *testing.T) {
	sectionsOn(t)
	mem := membucket.New()
	b := archiveTeam(t, mem)
	newer := []byte(`{"sections":[{"id":"aaaaaaaaaaaaaaaa","name":"Game","projects":["` + p1 + `"],"look":{"emoji":"🎮"}},` +
		`{"id":"bbbbbbbbbbbbbbbb","name":"Tools","projects":["` + p1 + `"]}],"layout":"grid"}`)
	mem.Put("sections.json", bytes.NewReader(newer), int64(len(newer)), "", "")
	got, _ := remote.Sections(b)
	if names(got) != "Game:"+p1+" Tools:" {
		t.Errorf("read: %s", names(got))
	}
	if _, err := remote.SetProjectSection(b, p2, "bbbbbbbbbbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	data, _, _ := mem.Get("sections.json")
	if !strings.Contains(string(data), `"look":{"emoji":"🎮"}`) || !strings.Contains(string(data), `"layout":"grid"`) {
		t.Errorf("a newer R3V's fields dropped: %s", data)
	}
}

// racingSections writes someone else's section just before this R3V's
// first write lands.
type racingSections struct {
	*membucket.Bucket
	raced bool
	fail  error
}

func (r *racingSections) Put(key string, body io.Reader, size int64, sum, cond string) error {
	if key == "sections.json" && r.fail != nil {
		return r.fail
	}
	if key == "sections.json" && !r.raced {
		r.raced = true
		other := `{"sections":[{"id":"cccccccccccccccc","name":"Engine","projects":[]}]}`
		r.Bucket.Put(key, strings.NewReader(other), int64(len(other)), "", "")
	}
	return r.Bucket.Put(key, body, size, sum, cond)
}

func TestSectionsConflictReapplies(t *testing.T) {
	sectionsOn(t)
	bk := &racingSections{Bucket: membucket.New()}
	b := archiveTeam(t, bk)
	if _, err := remote.NewSection(b, "Game"); err != nil {
		t.Fatal(err)
	}
	if got, _ := remote.Sections(b); names(got) != "Engine: Game:" {
		t.Errorf("after a conflict: %s (both)", names(got))
	}
	// Someone else's of the same name meanwhile: refused, not doubled.
	bk.raced = false
	if _, err := remote.NewSection(b, "Engine"); !errors.Is(err, remote.ErrSectionExists) {
		t.Errorf("the same name meanwhile: %v", err)
	}
}

func TestSectionsFailedWriteChangesNothing(t *testing.T) {
	sectionsOn(t)
	bk := &racingSections{Bucket: membucket.New(), raced: true}
	b := archiveTeam(t, bk)
	bk.fail = errors.New("connection lost")
	if _, err := remote.NewSection(b, "Game"); err == nil {
		t.Fatal("no error")
	}
	if got, _ := remote.Sections(b); len(got) != 0 {
		t.Errorf("sections after a failed write: %s", names(got))
	}
	if evs, _ := b.TeamLog(time.Time{}); len(evs) != 0 {
		t.Errorf("logged a failed write: %+v", evs)
	}
}

func TestSectionsNotInBuild(t *testing.T) {
	was := remote.ProjectSections
	remote.ProjectSections = false
	defer func() { remote.ProjectSections = was }()
	mem := membucket.New()
	b := archiveTeam(t, mem)
	if _, err := remote.NewSection(b, "Game"); !errors.Is(err, remote.ErrSectionsNotInBuild) {
		t.Errorf("a section without them: %v", err)
	}
	if _, err := remote.SetProjectSection(b, p1, ""); !errors.Is(err, remote.ErrSectionsNotInBuild) {
		t.Errorf("ungrouping without them: %v", err)
	}
	// An older R3V doesn't read them at all.
	rec := `{"sections":[{"id":"aaaaaaaaaaaaaaaa","name":"Game","projects":["` + p1 + `"]}]}`
	mem.Put("sections.json", strings.NewReader(rec), int64(len(rec)), "", "")
	if got, err := remote.Sections(b); err != nil || len(got) != 0 {
		t.Errorf("read without them: %v %v", got, err)
	}
}

// On an R3V Cloud team the service lets only admins write the record.
func TestSectionsOnCloudAdminsOnly(t *testing.T) {
	sectionsOn(t)
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
	if _, err := remote.NewSection(member, "Engine"); !errors.Is(err, remote.ErrNotAdmin) {
		t.Fatalf("a member making a section: %v", err)
	}
	owner = open("token")
	owner.SetActor(cloudtest.Owner)
	s, err := remote.NewSection(owner, "Engine")
	if err != nil {
		t.Fatal(err)
	}
	// Changed over the record it wrote (if-match its etag).
	if _, err := remote.SetProjectSection(owner, p1, s.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := remote.Sections(open("kai")); err != nil || names(got) != "Engine:"+p1 {
		t.Errorf("a member reads: %s %v", names(got), err)
	}
	if _, err := remote.SetProjectSection(open("kai"), p1, ""); !errors.Is(err, remote.ErrNotAdmin) {
		t.Errorf("a member ungrouping: %v", err)
	}
}
