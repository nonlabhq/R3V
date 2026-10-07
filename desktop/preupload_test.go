package desktop

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A big file goes up in the background while the project is idle, not
// while an action holds it; it says so as it goes.
func TestPreuploadInTheBackground(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	defer func(m int64, s time.Duration) { project.PreuploadMin, project.PreuploadStable = m, s }(project.PreuploadMin, project.PreuploadStable)
	project.PreuploadMin, project.PreuploadStable = 1000, 0
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	var events []Preupload
	a.emit = func(name string, data any) {
		if p, ok := data.(Preupload); ok && name == "preupload" {
			events = append(events, p)
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
	os.MkdirAll(filepath.Join(root, "Video"), 0o755)
	os.WriteFile(filepath.Join(root, "Video", "take.mov"), []byte(strings.Repeat("frame ", 1000)), 0o644)

	unlock := a.lock(root) // a commit, say
	a.preuploadProject(context.Background(), root)
	unlock()
	if len(events) != 0 {
		t.Fatalf("went up while busy: %+v", events)
	}
	a.preuploadProject(context.Background(), root)
	if len(events) < 2 || events[0].Path != "Video/take.mov" || events[0].Done || !events[len(events)-1].Done {
		t.Fatalf("events: %+v", events)
	}
	if len(a.Preuploads()) != 0 {
		t.Error("still listed as going up")
	}
	r, _ := project.Open(root)
	if c, _ := r.PreuploadCandidates(time.Now()); len(c) != 0 {
		t.Errorf("not recorded: %+v", c)
	}
}

// A project just added is looked at for big files at once, not at the next
// round a minute later; only files still changing wait.
func TestPreuploadStartsWhenAdded(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	defer func(m int64, s time.Duration) { project.PreuploadMin, project.PreuploadStable = m, s }(project.PreuploadMin, project.PreuploadStable)
	project.PreuploadMin, project.PreuploadStable = 1000, 2*time.Minute
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	started := make(chan string, 16)
	a.emit = func(name string, data any) {
		if p, ok := data.(Preupload); ok && name == "preupload" && !p.Done {
			started <- p.Path
		}
	}
	team, err := a.CreateStorageTeam(remote.Storage{Endpoint: fake.URL, Bucket: "band", AccessKey: "k", SecretKey: "s"}, "Band")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.preuploadOnSchedule(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	root := newSong(t)
	old := time.Now().Add(-time.Hour)
	os.MkdirAll(filepath.Join(root, "Video"), 0o755)
	os.WriteFile(filepath.Join(root, "Video", "old.mov"), []byte(strings.Repeat("frame ", 1000)), 0o644)
	os.Chtimes(filepath.Join(root, "Video", "old.mov"), old, old)
	os.WriteFile(filepath.Join(root, "Video", "rendering.mov"), []byte(strings.Repeat("still ", 1000)), 0o644)
	if _, err := a.AddProjectToTeam(team.ID, root); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-started:
		if p != "Video/old.mov" {
			t.Fatalf("went up first: %s", p)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nothing went up after adding the project (waiting for the next round?)")
	}
	// (more news of old.mov as it goes; nothing of the file still changing)
	for wait := time.After(time.Second); ; {
		select {
		case p := <-started:
			if p != "Video/old.mov" {
				t.Fatalf("a file still changing went up: %s", p)
			}
			continue
		case <-wait:
		}
		break
	}
}

// A commit stops the project's background upload and takes over: one queue,
// and the pieces already up aren't sent again.
func TestCommitTakesOverPreupload(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	defer func(m int64, s time.Duration) { project.PreuploadMin, project.PreuploadStable = m, s }(project.PreuploadMin, project.PreuploadStable)
	project.PreuploadMin, project.PreuploadStable = 1000, 0
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	var mu sync.Mutex
	var events []Preupload
	a.emit = func(name string, data any) {
		if p, ok := data.(Preupload); ok && name == "preupload" {
			mu.Lock()
			events = append(events, p)
			mu.Unlock()
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
	big := make([]byte, 24<<20) // in pieces (chunk.MinFile)
	rand.New(rand.NewSource(1)).Read(big)
	os.MkdirAll(filepath.Join(root, "Video"), 0o755)
	os.WriteFile(filepath.Join(root, "Video", "take.mov"), big, 0o644)

	// Hold the background upload at its third write, a few pieces in.
	held, release := make(chan struct{}), make(chan struct{})
	var writes atomic.Int32
	fake.OnWrite = func(_, _ string) {
		if writes.Add(1) == 3 {
			close(held)
			<-release
		}
	}
	before := fake.PutBytes
	done := make(chan struct{})
	go func() { a.preuploadProject(context.Background(), root); close(done) }()
	select {
	case <-held:
	case <-time.After(20 * time.Second):
		t.Fatal("the background upload didn't start")
	}
	// The commit, while the background upload is held; then let it go on.
	if _, err := a.Save(root, "with the take", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the background upload didn't end")
	}
	r, _ := project.Open(root)
	if _, err := os.Stat(filepath.Join(r.Dir, "preuploaded")); err == nil {
		t.Error("the background upload went on beside the commit")
	}
	mu.Lock()
	defer mu.Unlock()
	if last := events[len(events)-1]; !last.Done || len(a.Preuploads()) != 0 {
		t.Errorf("still listed as going up: %+v", last)
	}
	// Each piece once, give or take the one stopped half-way.
	if sent := fake.PutBytes - before; sent > int64(len(big))+8<<20 {
		t.Errorf("sent %d MB for a %d MB file", sent>>20, len(big)>>20)
	}
}
