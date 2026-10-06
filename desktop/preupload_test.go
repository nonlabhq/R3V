package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
