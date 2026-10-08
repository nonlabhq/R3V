//go:build nightly

package cloud

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// fakeMoves answers the moves and claims calls as R3V-Cloud does, keeping
// what it was sent.
type fakeMoves struct {
	*httptest.Server
	mu    sync.Mutex
	calls []string
	plans [][]PlanItem
	sent  []string // bodies, as sent
}

const testTeam = "11111111111111111111111111111111"

func newFakeMoves(t *testing.T) *fakeMoves {
	f := &fakeMoves{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		f.sent = append(f.sent, string(body))
		f.mu.Unlock()
		team := "/v1/teams/" + testTeam
		switch {
		case r.URL.Path == "/v1/me":
			w.Write([]byte(`{"user":{"id":"u1","email":"yi@example.test"},"teams":[{"id":"` + testTeam + `","name":"Band","role":"member","memberId":"` + strings.Repeat("2", 32) + `"}]}`))
		case r.Method == "POST" && r.URL.Path == team+"/moves":
			var in struct{ Source MoveSource }
			json.Unmarshal(body, &in)
			if in.Source.AccessKey != "ro" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error":{"code":"cant_read","message":"R3V Cloud can't read that storage's team.json with this key (403 InvalidAccessKeyId)."}}`))
				return
			}
			w.WriteHeader(201)
			w.Write([]byte(`{"move":"m1"}`))
		case r.URL.Path == team+"/moves/m1/plan":
			var in struct {
				Project string
				Items   []PlanItem
			}
			json.Unmarshal(body, &in)
			f.mu.Lock()
			f.plans = append(f.plans, in.Items)
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]int{"added": len(in.Items) - 1, "skipped": 1})
		case r.Method == "GET" && r.URL.Path == team+"/moves/m1":
			w.Write([]byte(`{"state":"paused","reason":"over_limit","items":3,"itemsDone":2,"bytes":30,"bytesDone":20,"failed":[]}`))
		case r.URL.Path == "/v1/invitations/tok3n":
			w.Write([]byte(`{"teamId":"` + testTeam + `","team":"Band","role":"member","claimables":[{"id":"` + strings.Repeat("3", 32) + `","name":"Mia","color":"b3","picture":null,"claimed":false}]}`))
		case r.URL.Path == "/v1/invitations/tok3n/accept":
			w.Write([]byte(`{"teamId":"` + testTeam + `","role":"member"}`))
		case r.Method == "GET" && r.URL.Path == team+"/members/import":
			w.Write([]byte(`{"items":[{"id":"` + strings.Repeat("3", 32) + `","name":"Mia","color":null,"picture":null,"claimed":true}],"more":false}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(f.Close)
	t.Setenv("R3V_CLOUD_TOKEN", "tok")
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	return f
}

func TestMoveCalls(t *testing.T) {
	f := newFakeMoves(t)
	src := MoveSource{Endpoint: "https://old.example", Region: "auto", Bucket: "band", Prefix: "r3v", AccessKey: "ro", SecretKey: "s"}

	bad := src
	bad.AccessKey = "rw"
	if _, err := StartMove(f.URL, testTeam, bad); !errors.Is(err, ErrMoveCantRead) || !strings.Contains(err.Error(), "InvalidAccessKeyId") {
		t.Errorf("a key it can't read with: %v", err)
	}
	move, err := StartMove(f.URL, testTeam, src)
	if err != nil || move != "m1" {
		t.Fatal(move, err)
	}

	// 2,500 items: three calls of up to 1,000, in order.
	items := make([]PlanItem, 2500)
	for i := range items {
		items[i] = PlanItem{Src: "r3v/objects/" + string(rune('a'+i%26)), Dst: "objects/x", Size: int64(i)}
	}
	added, skipped, err := AddPlan(f.URL, testTeam, move, strings.Repeat("4", 32), items)
	if err != nil || added != 2497 || skipped != 3 {
		t.Errorf("AddPlan: %d added, %d skipped, %v", added, skipped, err)
	}
	if len(f.plans) != 3 || len(f.plans[0]) != 1000 || len(f.plans[2]) != 500 || !slices.Equal(slices.Concat(f.plans...), items) {
		t.Errorf("plans sent: %d calls", len(f.plans))
	}

	if err := StartCopy(f.URL, testTeam, move); err != nil {
		t.Fatal(err)
	}
	st, err := Move(f.URL, testTeam, move)
	if err != nil || st.State != "paused" || st.Reason != "over_limit" || st.ItemsDone != 2 || st.BytesDone != 20 || st.Failed == nil {
		t.Errorf("Move: %+v %v", st, err)
	}
	if err := CancelMove(f.URL, testTeam, move); err != nil {
		t.Fatal(err)
	}
	if err := ForgetMove(f.URL, testTeam, move); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /v1/teams/" + testTeam + "/moves/m1/start",
		"GET /v1/teams/" + testTeam + "/moves/m1",
		"POST /v1/teams/" + testTeam + "/moves/m1/cancel",
		"DELETE /v1/teams/" + testTeam + "/moves/m1",
	}
	if got := f.calls[len(f.calls)-4:]; !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

func TestClaimCalls(t *testing.T) {
	f := newFakeMoves(t)
	mia := strings.Repeat("3", 32)
	if err := ImportMembers(f.URL, testTeam, []ImportedMember{{ID: mia, Name: "Mia", Color: "b3"}}); err != nil {
		t.Fatal(err)
	}
	if got := f.sent[len(f.sent)-1]; got != `[{"id":"`+mia+`","name":"Mia","color":"b3"}]` {
		t.Errorf("imported %s", got)
	}

	team, ms, err := InvitationClaimables(f.URL, "tok3n")
	if err != nil || team != "Band" || len(ms) != 1 || ms[0].ID != mia || ms[0].Color != "b3" || ms[0].Picture != "" || ms[0].Claimed {
		t.Errorf("claimables: %q %+v %v", team, ms, err)
	}
	addr, err := AcceptAs(f.URL, "tok3n", mia)
	if err != nil || addr != TeamAddress(f.URL, testTeam) {
		t.Fatal(addr, err)
	}
	if i := slices.Index(f.calls, "POST /v1/invitations/tok3n/accept"); i < 0 || f.sent[i] != `{"claim":"`+mia+`"}` {
		t.Errorf("accepted with %q", f.sent[max(i, 0)])
	}
	if _, err := AcceptAs(f.URL, "tok3n", ""); err != nil {
		t.Fatal(err)
	}
	if f.sent[len(f.sent)-2] != "" { // (then /v1/me)
		t.Errorf("accepted as a new member with %q", f.sent[len(f.sent)-2])
	}

	if err := Claim(f.URL, testTeam, mia); err != nil {
		t.Fatal(err)
	}
	cs, err := TeamClaimables(f.URL, testTeam)
	if err != nil || len(cs) != 1 || !cs[0].Claimed || cs[0].Color != "" {
		t.Errorf("team claimables: %+v %v", cs, err)
	}
}
