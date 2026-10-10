package project

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
)

// An agent's branch says so, and whom it works for; the project stays
// where it is unless told.
func TestAgentBranch(t *testing.T) {
	withBranchRecords(t)
	a, _ := team(t)
	head, on := a.Head(), a.BranchName()
	key, err := a.NewBranchWith("Renderer", BranchOptions{Agent: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.BranchName() != on || a.Head() != head {
		t.Fatalf("moved to %q", a.BranchName())
	}
	recs, err := a.BranchRecords()
	if err != nil {
		t.Fatal(err)
	}
	me, _ := a.Identity()
	if rec := recs[key]; rec.Kind != remote.BranchAgent || rec.Owner != me || rec.Name != "Renderer" || rec.From != head {
		t.Fatalf("record: %+v (me %q)", rec, me)
	}
	// A person's branch has no kind, as before.
	other, err := a.NewBranch("Mix", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if recs, _ := a.BranchRecords(); recs[other].Kind != "" || recs[other].Owner != "" {
		t.Fatalf("person's branch: %+v", recs[other])
	}
}

// Without branch records (Stable) an agent's branch isn't made: it
// couldn't say whose it is.
func TestAgentBranchNeedsRecords(t *testing.T) {
	was := remote.BranchRecords
	remote.BranchRecords = false
	t.Cleanup(func() { remote.BranchRecords = was })
	a, _ := team(t)
	if _, err := a.NewBranchWith("renderer", BranchOptions{Agent: true}); !errors.Is(err, remote.ErrNoBranchRecords) {
		t.Fatalf("got %v", err)
	}
	branches, err := a.Branches()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		if b.Name == "renderer" {
			t.Fatal("made anyway")
		}
	}
}
