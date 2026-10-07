package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A download cut off: the project says what the team has (to get), it can't
// be committed by mistake, and downloading it again finishes it.
func TestDownloadCutOff(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
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
	r, _ := project.Open(root)
	parent := t.TempDir()
	dir := filepath.Join(parent, "Song Project")

	fake.CutReadsAfter = fake.Reads + 2
	if _, err := a.DownloadProject(team.ID, r.Config.ProjectID, parent); err == nil {
		t.Fatal("the download wasn't cut off")
	}
	fake.CutReadsAfter = 0
	if part, err := a.TeamState(dir); err != nil || len(part.Incoming) == 0 {
		t.Errorf("the team's versions aren't listed to get: %v", err)
	}
	if _, err := a.Save(dir, "oops", true, nil, true, nil); !errors.Is(err, project.ErrNotDownloaded) {
		t.Errorf("committed before the download finished: %v", err)
	}
	p, err := a.DownloadProject(team.ID, r.Config.ProjectID, parent)
	if err != nil || p.Root != dir {
		t.Fatalf("downloading again: %+v %v", p, err)
	}
	if st, err := a.State(dir); err != nil || st.Head != r.Head() || len(st.Changes) != 0 {
		t.Errorf("after downloading again: %v", err)
	}
}

// R3V closed while a big file went up: when it starts again, the copy left
// behind is cleaned up and the file goes up again by itself.
func TestPreuploadAfterRestart(t *testing.T) {
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	defer func(m int64, s time.Duration) { project.PreuploadMin, project.PreuploadStable = m, s }(project.PreuploadMin, project.PreuploadStable)
	project.PreuploadMin, project.PreuploadStable = 1000, 0
	fake := s3test.New("band")
	defer fake.Close()
	t.Cleanup(waitTidy)
	a := NewApp()
	done := make(chan Preupload, 16)
	a.emit = func(name string, data any) {
		if p, ok := data.(Preupload); ok && p.Done {
			done <- p
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
	r, _ := project.Open(root)
	left := filepath.Join(r.Dir, "preupload")
	os.MkdirAll(left, 0o755)
	os.WriteFile(filepath.Join(left, "tmp-123"), []byte("half a copy"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { a.preuploadOnSchedule(ctx); close(stopped) }()
	defer func() { cancel(); <-stopped }()
	select {
	case p := <-done:
		if p.Path != "Video/take.mov" {
			t.Fatalf("went up: %s", p.Path)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nothing went up after starting again")
	}
	if _, err := os.Stat(filepath.Join(left, "tmp-123")); !os.IsNotExist(err) {
		t.Error("the copy left behind is still there")
	}
	if c, _ := r.PreuploadCandidates(time.Now()); len(c) != 0 {
		t.Errorf("not recorded: %+v", c)
	}
}
