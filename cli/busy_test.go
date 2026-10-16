package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// Another program holds the project: a command that writes says who,
// after --wait; one that reads reads it as it is and says it is busy.
func TestBusy(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{"notes.txt": "first"})
	r, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	// A teammate's branch, its versions not on this computer yet.
	b := filepath.Join(t.TempDir(), "B", "Song Project")
	rb, _, err := project.Clone(code, r.Config.Name, b, "alex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rb.NewBranchWith("lane-render", project.BranchOptions{Switch: true}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(b, "render.txt"), []byte("render"), 0o644)
	if exit, rep := run(t, b, "commit", "-m", "render pass"); exit != 0 {
		t.Fatalf("commit on lane-render: %+v", rep.Error)
	}

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("second"), 0o644)
	held, _ := project.Open(a)
	project.LockProgram, project.LockCommand = "cli", "commit"
	release, err := held.Lock(0)
	if err != nil {
		t.Fatal(err)
	}

	// A write waits as long as --wait says, then says who has it.
	start := time.Now()
	exit, rep := run(t, a, "commit", "-m", "second", "--wait", "0.3")
	took := time.Since(start)
	if exit != exitLater || rep.Error.Code != "project_busy" || rep.Error.Holder == nil ||
		rep.Error.Holder.PID != os.Getpid() || rep.Error.Holder.Command != "commit" || !rep.Error.Holder.Running ||
		!strings.Contains(rep.Error.Message, "r3v commit (pid ") {
		t.Fatalf("busy commit: %d %+v %+v", exit, rep.Error, rep.Error.Holder)
	}
	if took < 300*time.Millisecond || took > 5*time.Second {
		t.Errorf("--wait 0.3 waited %v", took)
	}
	if exit, rep := run(t, a, "commit", "-m", "second", "--wait", "soon"); exit != exitUsage {
		t.Errorf("--wait soon: %d %+v", exit, rep.Error)
	}

	// Reading: done without the lock, said busy, nothing written.
	ix := filepath.Join(held.Dir, "index.bin")
	before, err := os.Stat(ix)
	if err != nil {
		t.Fatal(err)
	}
	exit, rep = run(t, a, "status")
	var st struct {
		Changes []changeJSON
		Busy    bool
		Holder  *holderJSON
	}
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || !st.Busy || st.Holder == nil || st.Holder.PID != os.Getpid() || len(st.Changes) != 1 {
		t.Fatalf("status while busy: %d %s %+v", exit, rep.Result, rep.Error)
	}
	if after, _ := os.Stat(ix); after == nil || !after.ModTime().Equal(before.ModTime()) {
		t.Error("status while busy wrote the index")
	}
	exit, rep = run(t, a, "log")
	var lg struct {
		Versions []versionJSON
		Busy     bool
	}
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || !lg.Busy || len(lg.Versions) != 1 {
		t.Fatalf("log while busy: %d %s", exit, rep.Result)
	}

	// Another branch's log, its versions fetched; a name not quite right.
	exit, rep = run(t, a, "log", "lane-render")
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 2 || lg.Versions[0].Message != "render pass" || lg.Versions[1].Message != "first" {
		t.Fatalf("log lane-render: %d %s %+v", exit, rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "log", "lane/render", "-n", "1")
	if exit != exitError || rep.Error.Code != "error" || rep.Error.Suggestion != "lane-render" ||
		!strings.Contains(rep.Error.Message, "did you mean lane-render?") {
		t.Fatalf("log lane/render: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "log", "drums"); exit == 0 || rep.Error.Suggestion != "" {
		t.Fatalf("log drums: %d %+v", exit, rep.Error)
	}

	// The holder's process has ended (an older R3V holds it now, say).
	gone := exec.Command(os.Args[0], "-test.run=^$")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(project.Holder{Program: "app", PID: gone.ProcessState.Pid(), Since: time.Now()})
	os.WriteFile(filepath.Join(held.Dir, "lock-holder.json"), data, 0o644)
	exit, rep = run(t, a, "commit", "-m", "second", "--wait", "0")
	if exit != exitLater || rep.Error.Holder == nil || rep.Error.Holder.Running || rep.Error.Holder.Program != "app" ||
		!strings.Contains(rep.Error.Message, "has ended") {
		t.Fatalf("holder gone: %d %+v %+v", exit, rep.Error, rep.Error.Holder)
	}

	// Let go: reading takes the lock again, said not busy; the record goes.
	release()
	if _, err := os.Stat(filepath.Join(held.Dir, "lock-holder.json")); !os.IsNotExist(err) {
		t.Errorf("the holder record stayed: %v", err)
	}
	exit, rep = run(t, a, "status")
	if exit != 0 || strings.Contains(string(rep.Result), `"busy"`) {
		t.Fatalf("status: %d %s", exit, rep.Result)
	}
	if exit, rep := run(t, a, "merge", "lane_render", "--preview"); exit != exitError || rep.Error.Suggestion != "lane-render" {
		t.Fatalf("merge lane_render: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "commit", "-m", "second"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
}
