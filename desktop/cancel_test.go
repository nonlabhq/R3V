package desktop

import (
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A commit cancelled while it uploads: its progress says it can be, it
// ends as "cancelled" and the changes are uncommitted again.
func TestCancelSave(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	var cancellable atomic.Bool
	a.emit = func(name string, data any) {
		if p, ok := data.(ProgressEvent); ok && p.Cancellable {
			cancellable.Store(true)
		}
	}
	team, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "band", AccessKey: "k", SecretKey: "s"}, "Band")
	if err != nil {
		t.Fatal(err)
	}
	root := newSong(t)
	if _, err := a.AddProjectToTeam(team.ID, root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	waitTidy()
	if a.CancelSave(root) {
		t.Error("cancelled with nothing under way")
	}
	head := func() string { r, _ := project.Open(root); return r.Head() }
	was := head()
	big := make([]byte, 20<<20)
	rand.New(rand.NewSource(3)).Read(big)
	os.MkdirAll(filepath.Join(root, "Video"), 0o755)
	os.WriteFile(filepath.Join(root, "Video", "take.mov"), big, 0o644)

	// Cancelled at its third write to the team's storage.
	var writes atomic.Int32
	fake.OnWrite = func(_, _ string) {
		if writes.Add(1) == 3 && !a.CancelSave(root) {
			t.Error("couldn't cancel")
		}
	}
	res, err := a.Save(root, "oops", true, nil, true, nil)
	if err != nil || res.Action != "cancelled" {
		t.Fatalf("%+v %v", res, err)
	}
	if !cancellable.Load() {
		t.Error("progress didn't say it could be cancelled")
	}
	if head() != was {
		t.Error("the version stayed")
	}
	if st, err := a.State(root); err != nil || len(st.Changes) != 1 {
		t.Errorf("changes: %v", err)
	}
	if a.CancelSave(root) {
		t.Error("still cancellable after it ended")
	}

	fake.OnWrite = nil
	if res, err := a.Save(root, "the take", true, nil, true, nil); err != nil || res.Action != "published" {
		t.Fatalf("again: %+v %v", res, err)
	}
}
