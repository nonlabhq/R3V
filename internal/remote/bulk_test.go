package remote

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote/cloudtest"
)

func bulkTeam(t *testing.T) (*cloudtest.Fake, *BucketBackend) {
	t.Helper()
	f := cloudtest.NewFake(t)
	b, err := NewBroker(strings.TrimPrefix(f.Address, "r3v-cloud+"), "token")
	if err != nil {
		t.Fatal(err)
	}
	b.(*brokerBucket).backoff = time.Millisecond
	return f, NewBucketBackend(b)
}

func putKey(t *testing.T, s *BucketBackend, key, value string) {
	t.Helper()
	if err := s.b.Put(key, strings.NewReader(value), int64(len(value)), "", ""); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

// teamHomeRead is what a team's home and its projects' pages read of a
// team: its projects, members and log, each project's branches, their
// records and its milestones.
type teamHomeRead struct {
	Projects   []Project
	Members    []Member
	Log        []TeamEvent
	Branches   map[string]map[string]string
	Records    map[string]map[string]BranchRecord
	Milestones map[string]map[string]Milestone
}

func readTeamHome(t *testing.T, s *BucketBackend, since time.Time) teamHomeRead {
	t.Helper()
	var out teamHomeRead
	var err error
	if out.Projects, err = s.Projects(); err != nil {
		t.Fatal(err)
	}
	if out.Members, err = s.Members(); err != nil {
		t.Fatal(err)
	}
	if out.Log, err = s.TeamLog(since); err != nil {
		t.Fatal(err)
	}
	out.Branches, out.Records, out.Milestones = map[string]map[string]string{}, map[string]map[string]BranchRecord{}, map[string]map[string]Milestone{}
	for _, p := range out.Projects {
		if out.Branches[p.ID], err = s.Branches(p.ID); err != nil {
			t.Fatal(err)
		}
		if out.Records[p.ID], err = s.BranchRecords(p.ID); err != nil {
			t.Fatal(err)
		}
		if out.Milestones[p.ID], err = s.Milestones(p.ID); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// Reading a team's many small records on R3V Cloud: a request or a few per
// folder (keys/read, listings with values) instead of a listing and a GET
// apiece; the same answers from an older service, read one by one.
func TestBulkReadsOfATeam(t *testing.T) {
	f, s := bulkTeam(t)
	hex := func(c byte, i int) string { return strings.Repeat(string(c), 31) + fmt.Sprint(i) }
	for i := range 5 {
		pid := hex('a', i)
		putKey(t, s, "projects/"+pid+"/project.json", fmt.Sprintf(`{"id":%q,"name":"Project %d"}`, pid, i))
		for _, br := range []string{"main", "verse", "chorus"} {
			putKey(t, s, "projects/"+pid+"/branches/"+br, strings.Repeat("c", 64)+"\n")
		}
		for _, br := range []string{"verse", "chorus"} {
			putKey(t, s, "projects/"+pid+"/branchinfo/"+br+".json", `{"name":"The `+br+`"}`)
		}
		for j := range 2 {
			putKey(t, s, "projects/"+pid+"/milestones/"+hex('d', j)+".json", `{"version":"`+strings.Repeat("e", 64)+`","name":"v`+fmt.Sprint(j)+`"}`)
		}
	}
	for i := range 4 {
		id := hex('f', i)
		putKey(t, s, "members/"+id+".json", fmt.Sprintf(`{"id":%q,"name":"Member %d"}`, id, i))
	}
	now := time.Now().UTC()
	for i := range 3 {
		at := now.Add(-time.Duration(i) * time.Hour)
		putKey(t, s, teamLogDir+at.Format(logKeyTime)+fmt.Sprintf("-0000000%d.json", i),
			fmt.Sprintf(`{"time":%q,"kind":"project-added","project":%q,"by":%q}`, at.Format(time.RFC3339Nano), hex('a', i), hex('f', 0)))
	}
	putKey(t, s, teamLogDir+now.Add(-90*24*time.Hour).Format(logKeyTime)+"-00000009.json", `{"kind":"project-added"}`)
	since := now.Add(-7 * 24 * time.Hour)

	f.ResetRequests()
	bulk := readTeamHome(t, s, since)
	after := f.Total()
	afterBy := f.Requests()

	f.Old.Store(true)
	f.ResetRequests()
	old := readTeamHome(t, s, since)
	before := f.Total()
	beforeBy := f.Requests()
	t.Logf("requests: %d one by one (%v), %d with bulk reads (%v)", before, beforeBy, after, afterBy)

	if len(bulk.Projects) != 5 || len(bulk.Members) != 4 || len(bulk.Log) != 3 || len(bulk.Branches[hex('a', 0)]) != 3 ||
		bulk.Records[hex('a', 4)]["verse"].Name != "The verse" || len(bulk.Milestones[hex('a', 2)]) != 2 {
		t.Fatalf("read: %+v", bulk)
	}
	if !reflect.DeepEqual(bulk, old) {
		t.Errorf("bulk reads differ from reading one by one:\n%+v\n%+v", bulk, old)
	}
	// 1 folders + 1 keys/read + members, the log and 3 folders of each of
	// 5 projects: a listing with values each.
	if after != 19 || afterBy["GET key"] != 0 {
		t.Errorf("%d requests with bulk reads: %v", after, afterBy)
	}
	// The older service: keys/read asked once (404), then as before.
	if before < 3*after || beforeBy["POST keys/read"] != 1 {
		t.Errorf("%d requests one by one: %v", before, beforeBy)
	}
	// Found old, it isn't asked keys/read again for a while.
	f.ResetRequests()
	if _, err := s.Projects(); err != nil {
		t.Fatal(err)
	}
	if n := f.Requests()["POST keys/read"]; n != 0 {
		t.Errorf("asked an older service keys/read again: %v", f.Requests())
	}
}

// keys/read in pages of 1000, again for those "later", without those
// missing; listings with values a page at a time, pictures read alone.
func TestBulkReadsPagesLaterAndMissing(t *testing.T) {
	f, s := bulkTeam(t)
	br := s.b.(*brokerBucket)
	var keys []string
	want := map[string][]byte{}
	for i := range 1203 {
		k := fmt.Sprintf("members/%032x.json", i)
		keys = append(keys, k)
		if i%100 != 7 { // (some absent)
			v := fmt.Sprintf(`{"id":"%032x"}`, i)
			putKey(t, s, k, v)
			want[k] = []byte(v)
		}
	}
	f.ReadBytes = 4000 // about 180 values a request, then "later"
	f.ResetRequests()
	got, err := br.ReadKeys(keys)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read %d keys, want %d", len(got), len(want))
	}
	if n := f.Requests()["POST keys/read"]; n < 7 || f.Requests()["GET key"] != 0 {
		t.Errorf("requests: %v", f.Requests())
	}

	f.PageKeys = 500
	f.ResetRequests()
	listed, err := s.readDir("members/", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed, want) || f.Requests()["GET keys values"] != 3 {
		t.Errorf("listed %d keys in %v", len(listed), f.Requests())
	}

	pic := "pictures/team/" + strings.Repeat("ab", 32)
	png := []byte("\x89PNG\r\n\x1a\nnot really")
	if err := s.b.Put(pic, bytes.NewReader(png), int64(len(png)), "", ""); err != nil {
		t.Fatal(err)
	}
	f.ResetRequests()
	pics, err := s.readDir("pictures/team/", "")
	if err != nil || !bytes.Equal(pics[pic], png) || f.Requests()["GET key"] != 1 {
		t.Errorf("pictures %v %v, requests %v", pics, err, f.Requests())
	}
}

// An older service lists without values: each key is read alone, the
// answer the same.
func TestBulkReadsFromAnOlderService(t *testing.T) {
	f, s := bulkTeam(t)
	f.Old.Store(true)
	for i := range 3 {
		putKey(t, s, fmt.Sprintf("members/%032x.json", i), fmt.Sprintf(`{"id":"%032x","name":"M%d"}`, i, i))
	}
	f.ResetRequests()
	ms, err := s.Members()
	if err != nil || len(ms) != 3 || ms[2].Name != "M2" {
		t.Fatalf("members %+v %v", ms, err)
	}
	if r := f.Requests(); r["GET keys values"] != 1 || r["GET key"] != 3 {
		t.Errorf("requests %v", r)
	}
	got, err := s.b.(*brokerBucket).ReadKeys([]string{"members/" + strings.Repeat("0", 32) + ".json", "members/none.json"})
	if err != nil || len(got) != 1 {
		t.Errorf("read %v %v", got, err)
	}
}
