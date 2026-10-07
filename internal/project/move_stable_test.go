//go:build !nightly

package project

import (
	"errors"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// Stable doesn't move projects between teams: the other team gets nothing.
func TestStableMovesNoProject(t *testing.T) {
	if MoveProjects {
		t.Fatal("moving projects is on in Stable")
	}
	fake := s3test.New("one", "two")
	defer fake.Close()
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(storageCode(fake, "one"))
	write(t, a.Root, "Notes/lyrics.txt", lyrics)
	a.Save("first", Strategy("fail"))
	if err := a.MoveToTeam(connected(t, storageCode(fake, "two")), MoveOptions{}); !errors.Is(err, ErrMoveNotInBuild) {
		t.Errorf("MoveToTeam: %v", err)
	}
	two, _ := remote.Open(mustConfig(t, storageCode(fake, "two")))
	if ps, _ := two.Projects(); len(ps) != 0 {
		t.Errorf("the other team got it: %+v", ps)
	}
}
