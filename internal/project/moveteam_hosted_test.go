//go:build nightly

package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// Between a team on its own storage (one pool of contents) and a hosted
// team (each project's apart), both ways.
func TestMoveProjectToAndFromHostedTeam(t *testing.T) {
	fake := s3test.New("one")
	defer fake.Close()
	own := storageCode(fake, "one")
	a, big := movingProject(t, own)
	hosted := newHostedFake(t)

	if err := a.MoveToTeam(connected(t, hosted), MoveOptions{}); err != nil {
		t.Fatalf("to the hosted team: %v", err)
	}
	b, _, err := Clone(hosted, a.Config.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("on the hosted team, the big file isn't whole")
	}
	if ms, _ := b.Milestones(); len(ms) != 1 {
		t.Errorf("milestones on the hosted team: %+v", ms)
	}

	// And back.
	if err := a.MoveToTeam(connected(t, own), MoveOptions{}); err != nil {
		t.Fatalf("back to its own storage: %v", err)
	}
	c, _, err := Clone(own, a.Config.Name, filepath.Join(t.TempDir(), "C", "Song"), "sam")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(c.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("back on its own storage, the big file isn't whole")
	}
}
