//go:build nightly

package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A team's move to R3V Cloud, the service's copy played from the plan:
// with only what the plan lists copied and the last step done, a teammate
// gets the whole project from the hosted team.
func TestTeamMovePlanIsWhole(t *testing.T) {
	fake := s3test.New("one")
	defer fake.Close()
	own := storageCode(fake, "one")
	a, big := movingProject(t, own)
	from, _ := a.Team()
	hosted := newHostedFake(t)
	to := connected(t, hosted)

	est, err := EstimateMove(from)
	if err != nil || len(est.Plans) != 1 {
		t.Fatalf("estimate: %+v %v", est, err)
	}
	plan := est.Plans[0]
	src, _ := from.Open()
	dst, _ := to.Open()
	sb := src.(*remote.BucketBackend).Bucket()
	db := dst.(*remote.BucketBackend).Bucket()
	if err := FinishMove(from, to, plan.Project); err != ErrNotCopied {
		t.Fatalf("finishing before the copy: %v", err)
	}
	// What the service does: each key as it is, in order.
	for _, it := range plan.Items {
		data, _, err := sb.Get(it.Src)
		if err != nil {
			t.Fatalf("%s: %v", it.Src, err)
		}
		if err := db.Put("projects/"+plan.Project.ID+"/"+it.Dst, bytes.NewReader(data), int64(len(data)), "", ""); err != nil {
			t.Fatalf("%s: %v", it.Dst, err)
		}
	}
	if err := FinishMove(from, to, plan.Project); err != nil {
		t.Fatal(err)
	}
	if err := FinishMove(from, to, plan.Project); err != nil {
		t.Fatalf("finishing again: %v", err)
	}
	b, _, err := Clone(hosted, plan.Project.Name, filepath.Join(t.TempDir(), "B", "Song"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.Root, "Samples", "stem.wav")); !bytes.Equal(got, big) {
		t.Error("the big file isn't whole")
	}
	if ms, _ := b.Milestones(); len(ms) != 1 {
		t.Errorf("milestones: %+v", ms)
	}
	if gone, _ := b.ArchivedBranches(); len(gone) != 1 {
		t.Errorf("deleted branches: %+v", gone)
	} else if err := b.UnarchiveBranch(gone[0].Key); err != nil {
		t.Errorf("restoring the deleted branch on the hosted team: %v", err)
	}
	log, _ := b.Log()
	for _, m := range log {
		if _, _, err := b.GoTo(m.ID, true); err != nil {
			t.Errorf("version %s: %v", short(m.ID), err)
		}
	}
}
