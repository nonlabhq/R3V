package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A long history comes in a few rounds, not one version after another: the
// versions are asked for together, and every one of them arrives whole.
func TestLongHistoryComesTogether(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a, _ := Init(newProject(t), "yi")
	a.SetRemote(code)
	const versions = 30
	for i := range versions {
		write(t, a.Root, "Notes/lyrics.txt", fmt.Sprintf("%sline %d\n", lyrics, i))
		if _, _, err := a.Save(fmt.Sprintf("take %d", i), Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	}
	var inFlight, most atomic.Int32
	fake.OnRead = func(path string) {
		if !strings.Contains(path, "/snapshots/") {
			return
		}
		n := inFlight.Add(1)
		for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
	}
	b, _, err := Clone(code, "Song", filepath.Join(t.TempDir(), "B", "Song Project"), "alex")
	if err != nil {
		t.Fatal(err)
	}
	if most.Load() < 2 {
		t.Errorf("versions asked for one at a time (at most %d at once)", most.Load())
	}
	log, err := b.Log()
	if err != nil || len(log) != versions {
		t.Fatalf("history here: %d versions, %v", len(log), err)
	}
	for _, m := range log {
		if _, err := b.Load(m.ID); err != nil {
			t.Errorf("version %s can't be read: %v", short(m.ID), err)
		}
	}
}
