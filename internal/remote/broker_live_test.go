package remote_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/backendtest"
)

// Against a running R3V-Cloud service with its development calls on (they
// make people and put them in teams without email); skipped otherwise:
//
//	R3V_TEST_CLOUD=<address> [R3V_TEST_CLOUD_DEV_KEY=<key>] go test ./internal/remote -run Broker -v
//
// Each run makes a new team with an owner, a member and a collaborator.

type cloudPerson struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}

func cloudCall(t *testing.T, base, token, method, path string, in, out any) {
	t.Helper()
	data, _ := json.Marshal(in)
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(data))
	req.Header.Set("content-type", "application/json")
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	if k := os.Getenv("R3V_TEST_CLOUD_DEV_KEY"); k != "" { // a deployed service's development calls
		req.Header.Set("x-dev-key", k)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, body)
	}
	if out != nil {
		json.Unmarshal(body, out)
	}
}

type cloudTeam struct {
	base, id                    string
	owner, member, collab       cloudPerson
	asOwner, asMember, asCollab *remote.BucketBackend
}

func newCloudTeam(t *testing.T) *cloudTeam {
	base := os.Getenv("R3V_TEST_CLOUD")
	if base == "" {
		t.Skip("R3V_TEST_CLOUD not set")
	}
	run := make([]byte, 4)
	rand.Read(run)
	who := func(name string) cloudPerson {
		var p cloudPerson
		cloudCall(t, base, "", "POST", "/dev/session", map[string]string{"email": name + "-" + hex.EncodeToString(run) + "@example.test"}, &p)
		return p
	}
	c := &cloudTeam{base: base, owner: who("owner"), member: who("member"), collab: who("collab")}
	var team struct{ ID string }
	cloudCall(t, base, c.owner.Token, "POST", "/v1/teams", map[string]string{"name": "Band"}, &team)
	c.id = team.ID
	cloudCall(t, base, "", "POST", "/dev/join", map[string]string{"team": c.id, "userId": c.member.UserID, "role": "member"}, nil)
	cloudCall(t, base, "", "POST", "/dev/join", map[string]string{"team": c.id, "userId": c.collab.UserID, "role": "collaborator"}, nil)
	open := func(p cloudPerson) *remote.BucketBackend {
		b, err := remote.NewBroker(base+"/v1/teams/"+c.id, p.Token)
		if err != nil {
			t.Fatal(err)
		}
		return remote.NewBucketBackend(b)
	}
	c.asOwner, c.asMember, c.asCollab = open(c.owner), open(c.member), open(c.collab)
	return c
}

func TestBrokerContract(t *testing.T) {
	c := newCloudTeam(t)
	// The contract's contents go in one project, which must exist.
	const pid = "cccccccccccccccccccccccccccccccc"
	if err := c.asOwner.PutProject(remote.Project{ID: pid, Name: "Contract"}); err != nil {
		t.Fatal(err)
	}
	backendtest.Run(t, remote.ForProject(c.asOwner, pid))
}

func blob(s string) (string, []byte) {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:]), []byte(s)
}

const (
	projA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	projB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestBrokerCollaborator(t *testing.T) {
	c := newCloudTeam(t)
	owner, member, collab := c.asOwner, c.asMember, c.asCollab
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Refused: not allowed, or (for what they can't see) not there.
	refused := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, remote.ErrForbidden) && !errors.Is(err, remote.ErrNotFound) {
			t.Errorf("%s: want refused, got %v", what, err)
		}
	}

	// The owner sets up the team: two projects with contents and a branch.
	must(owner.SetInfo(remote.TeamInfo{Name: "Band"}))
	hA, a := blob("in project A")
	hB, b := blob("in project B")
	for _, p := range []struct {
		id, hash string
		data     []byte
	}{{projA, hA, a}, {projB, hB, b}} {
		must(owner.PutProject(remote.Project{ID: p.id, Name: p.id[:1]}))
		must(remote.ForProject(owner, p.id).PutObject(p.hash, bytes.NewReader(p.data)))
		must(owner.UpdateBranch(p.id, "main", "", strings.Repeat("1", 64)))
	}
	// The collaborator gets project A, through the service's own call.
	cloudCall(t, c.base, c.owner.Token, "PUT", "/v1/teams/"+c.id+"/projects/"+projA+"/collaborators/"+c.collab.UserID,
		map[string]string{"access": "write"}, nil)

	if ps, err := member.Projects(); err != nil || len(ps) != 2 {
		t.Errorf("member sees %d projects (%v), want 2", len(ps), err)
	}
	ps, err := collab.Projects()
	if err != nil || len(ps) != 1 || ps[0].ID != projA {
		t.Errorf("collaborator sees %v (%v), want only A", ps, err)
	}

	// In project A the collaborator reads and writes.
	inA := remote.ForProject(collab, projA)
	r, err := inA.GetObject(hA)
	must(err)
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, a) {
		t.Error("collaborator read the wrong contents in A")
	}
	h2, d2 := blob("the collaborator's work")
	must(inA.PutObject(h2, bytes.NewReader(d2)))
	if miss, err := inA.MissingObjects([]string{hA, h2}); err != nil || len(miss) != 0 {
		t.Errorf("missing in A: %v %v", miss, err)
	}
	must(collab.UpdateBranch(projA, "main", strings.Repeat("1", 64), strings.Repeat("2", 64)))

	// Project B: nothing.
	inB := remote.ForProject(collab, projB)
	_, err = inB.GetObject(hB)
	refused("read B's contents", err)
	refused("write B's contents", inB.PutObject(h2, bytes.NewReader(d2)))
	_, err = collab.Branches(projB)
	refused("read B's branches", err)
	// B is hidden: its branch looks absent (so moving it is a conflict, and
	// nothing is written), and all its contents look missing.
	if err := collab.UpdateBranch(projB, "main", strings.Repeat("1", 64), strings.Repeat("2", 64)); err == nil {
		t.Error("collaborator moved B's branch")
	}
	if heads, err := owner.Branches(projB); err != nil || heads["main"] != strings.Repeat("1", 64) {
		t.Errorf("B's branch after the collaborator's try: %v %v", heads, err)
	}
	if miss, err := inB.MissingObjects([]string{hB}); err == nil && (len(miss) != 1 || miss[0] != hB) {
		t.Errorf("B's contents revealed to the collaborator: missing %v", miss)
	}

	// The team's own records and projects: not theirs to change.
	refused("rename the team", collab.SetInfo(remote.TeamInfo{Name: "Mine"}))
	refused("create a project", collab.PutProject(remote.Project{ID: "dddddddddddddddddddddddddddddddd", Name: "d"}))
	refused("delete project A", collab.DeleteProject(projA))
	if info, err := collab.Info(); err != nil || info.Name != "Band" {
		t.Errorf("collaborator reads the team's name: %q %v", info.Name, err)
	}

	// Nobody writes contents outside a project; members don't delete projects.
	refused("contents outside a project", member.PutObject(h2, bytes.NewReader(d2)))
	refused("member deletes a project", member.DeleteProject(projA))
}
