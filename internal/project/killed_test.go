package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// R3V killed mid-way (closed, crashed, the computer off): the step runs in
// a child process (this test binary) that is killed, so nothing after runs
// (no deferred cleanup), unlike a step that fails. Then: the team's data is
// whole, the files are as they were or complete, doing it again finishes
// it without sending again what went up, and what was left is cleaned up.
// See crash_test.go and resume_test.go for steps that fail.

// killedChild runs a step in the child: R3V_KILL_DIR the project (or the
// folder to clone into), R3V_KILL_CODE and R3V_KILL_NAME the team and
// project to clone, R3V_KILL_STAGE a stage to die at (os.Exit, as a crash)
// once R3V_KILL_DONE items of it are done.
func killedChild(op string) int {
	dir, stage := os.Getenv("R3V_KILL_DIR"), os.Getenv("R3V_KILL_STAGE")
	done, _ := strconv.Atoi(os.Getenv("R3V_KILL_DONE"))
	die := func(p Progress) {
		if stage != "" && p.Stage == stage && p.Done >= done {
			os.Exit(3)
		}
	}
	var err error
	switch op {
	case "clone":
		t, cerr := Connect(os.Getenv("R3V_KILL_CODE"))
		if err = cerr; err == nil {
			_, _, err = CloneFromTeam(t, os.Getenv("R3V_KILL_NAME"), dir, "alex", die)
		}
	default:
		r, oerr := Open(dir)
		if err = oerr; err != nil {
			break
		}
		r.OnProgress = die
		switch op {
		case "save":
			_, _, err = r.Save("change", Strategy("fail"))
		case "update":
			_, err = r.Update(Strategy("fail"))
		case "preupload":
			PreuploadMin = 1000 // (as the test that started it)
			cands, cerr := r.PreuploadCandidates(time.Now().Add(time.Hour))
			if err = cerr; err == nil && len(cands) == 1 {
				c, _ := r.Client()
				if err = r.Preupload(c, cands[0], nil, nil); err == nil {
					err = r.NotePreuploaded(cands[0].Hash)
				}
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runKilled runs op in a child and kills it at the n-th request of kind
// ("write" or "read") to the team's storage (0: not by request), or lets it
// die itself (R3V_KILL_STAGE in env). It says whether the child ended
// before finishing.
func runKilled(t *testing.T, fake *s3test.Server, op, kind string, n int, env ...string) bool {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), append([]string{"R3V_KILL_OP=" + op}, env...)...)
	var mu sync.Mutex
	var seen atomic.Int32
	kill := func() {
		if n > 0 && int(seen.Add(1)) == n {
			mu.Lock()
			cmd.Process.Kill()
			mu.Unlock()
		}
	}
	switch kind {
	case "write":
		fake.OnWrite = func(_, _ string) { kill() }
	case "read":
		fake.OnRead = func(string) { kill() }
	}
	defer func() { fake.OnWrite, fake.OnRead = nil, nil }()
	mu.Lock()
	err := cmd.Start()
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return cmd.Wait() != nil
}

// requests counts the requests of kind a step makes, run here.
func requests(fake *s3test.Server, kind string, step func()) int {
	var n atomic.Int32
	if kind == "write" {
		fake.OnWrite = func(_, _ string) { n.Add(1) }
	} else {
		fake.OnRead = func(string) { n.Add(1) }
	}
	step()
	fake.OnWrite, fake.OnRead = nil, nil
	return int(n.Load())
}

// killPoints: where to kill a step of steps requests (the first, the last
// and some between).
func killPoints(steps int) []int {
	out := []int{1}
	for _, f := range []int{4, 2} {
		if p := steps / f; p > out[len(out)-1] {
			out = append(out, p)
		}
	}
	if steps > out[len(out)-1] {
		out = append(out, steps)
	}
	return out
}

func leftIn(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// A commit killed while it copies the files into the history, or at any
// point of its upload: the team has a whole version, the files aren't
// touched, committing again finishes it without sending again what went
// up, and the copies it left go at the next cleanup.
func TestSaveKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills commits")
	}
	fake, code := cutTeam(t)
	a, _, _ := sharedProject(t, code, "Count")
	put := fake.PutBytes
	steps := requests(fake, "write", func() {
		if _, _, err := a.Save("change", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	})
	sent := fake.PutBytes - put
	type kill struct {
		stage   string
		done, n int
	}
	// (while storing: before the first file, and with one stored and others on the way)
	kills := []kill{{stage: StageStoring}, {stage: StageStoring, done: 1}}
	for _, n := range killPoints(steps) {
		kills = append(kills, kill{n: n})
	}
	for _, k := range kills {
		what := fmt.Sprintf("killed at write %d of %d", k.n, steps)
		if k.stage != "" {
			what = fmt.Sprintf("killed while %s (%d done)", k.stage, k.done)
		}
		fake, code := cutTeam(t)
		a, before, after := sharedProject(t, code, "Song")
		put := fake.PutBytes
		if !runKilled(t, fake, "save", "write", k.n, "R3V_KILL_DIR="+a.Root, "R3V_KILL_STAGE="+k.stage,
			"R3V_KILL_DONE="+strconv.Itoa(k.done)) {
			t.Fatalf("%s: it finished", what)
		}
		if got := cloneFiles(t, code, a.Config.Name); !maps.Equal(got, before) && !maps.Equal(got, after) {
			t.Fatalf("%s: a clone gets neither version:\n%v", what, fileNames(got))
		}
		if !maps.Equal(files(t, a.Root), after) {
			t.Fatalf("%s: the files changed", what)
		}
		r, err := Open(a.Root)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.Save("again", Strategy("fail")); err != nil {
			t.Fatalf("%s: committing again: %v", what, err)
		}
		if got := cloneFiles(t, code, a.Config.Name); !maps.Equal(got, after) {
			t.Fatalf("%s: after committing again, a clone gets:\n%v", what, fileNames(got))
		}
		if both := fake.PutBytes - put; both > sent+64<<10 {
			t.Errorf("%s: sent %d KB in all, %d KB without being killed", what, both>>10, sent>>10)
		}
		tmp := filepath.Join(r.Dir, "objects", "tmp")
		cleanLeftovers(tmp, time.Now().Add(time.Hour)) // (as a day later)
		if n := leftIn(tmp); n != 0 {
			t.Errorf("%s: %d files left after cleanup", what, n)
		}
	}
}

// An update killed at any download: the files and the version are as they
// were, and updating again finishes it.
func TestUpdateKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills updates")
	}
	fake, code := cutTeam(t)
	b, _, _ := behindTeammate(t, code)
	steps := requests(fake, "read", func() {
		if _, err := b.Update(Strategy("fail")); err != nil {
			t.Fatal(err)
		}
	})
	for _, n := range killPoints(steps) {
		what := fmt.Sprintf("killed at read %d of %d", n, steps)
		fake, code := cutTeam(t)
		b, before, after := behindTeammate(t, code)
		head := b.Head()
		if !runKilled(t, fake, "update", "read", n, "R3V_KILL_DIR="+b.Root) {
			t.Fatalf("%s: it finished", what)
		}
		r, err := Open(b.Root)
		if err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(files(t, r.Root), before) || r.Head() != head || r.UnfinishedSwitch() != "" {
			t.Fatalf("%s: the files or the version changed", what)
		}
		if _, err := r.Update(Strategy("fail")); err != nil {
			t.Fatalf("%s: updating again: %v", what, err)
		}
		if got := files(t, r.Root); !maps.Equal(got, after) {
			t.Fatalf("%s: after updating again:\n%v", what, fileNames(got))
		}
	}
}

// A clone killed at any download: what it left can't be committed, and
// cloning again into the folder finishes it.
func TestCloneKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills clones")
	}
	fake, code := cutTeam(t)
	a, _, _ := sharedProject(t, code, "Song")
	var want map[string]string
	steps := requests(fake, "read", func() { want = cloneFiles(t, code, a.Config.Name) })
	for _, n := range killPoints(steps) {
		what := fmt.Sprintf("killed at read %d of %d", n, steps)
		dir := filepath.Join(t.TempDir(), "B", "Song Project")
		if !runKilled(t, fake, "clone", "read", n, "R3V_KILL_DIR="+dir, "R3V_KILL_CODE="+code,
			"R3V_KILL_NAME="+a.Config.Name) {
			t.Fatalf("%s: it finished", what)
		}
		if r, err := Open(dir); err == nil {
			if _, _, err := r.Save("oops", Strategy("fail")); !errors.Is(err, ErrNotDownloaded) {
				t.Fatalf("%s: a commit before the download finished: %v", what, err)
			}
		}
		c, _, err := Clone(code, a.Config.Name, dir, "alex")
		if err != nil {
			t.Fatalf("%s: cloning again: %v", what, err)
		}
		if got := files(t, c.Root); !maps.Equal(got, want) {
			t.Fatalf("%s: after cloning again:\n%v", what, fileNames(got))
		}
	}
}

// A big file going up early, R3V killed at any point: its copy is left
// behind, cleaned up when R3V starts again, and the file goes up again
// sending only the pieces that didn't.
func TestPreuploadKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("kills uploads")
	}
	defer func(m int64, s time.Duration) { PreuploadMin, PreuploadStable = m, s }(PreuploadMin, PreuploadStable)
	PreuploadMin, PreuploadStable = 1000, 0
	setup := func() (*s3test.Server, *Repo, int64) {
		fake, code := cutTeam(t)
		r, _ := Init(newProject(t), "yi")
		if err := r.SetRemote(code); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.Save("v1", Strategy("fail")); err != nil {
			t.Fatal(err)
		}
		big := randomBytes(6, 20<<20) // in pieces
		write(t, r.Root, "Video/take.mov", string(big))
		return fake, r, int64(len(big))
	}
	fake, r, _ := setup()
	cands, _ := r.PreuploadCandidates(time.Now())
	c, _ := r.Client()
	steps := requests(fake, "write", func() {
		if err := r.Preupload(c, cands[0], nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	for _, n := range killPoints(steps) {
		what := fmt.Sprintf("killed at write %d of %d", n, steps)
		fake, r, size := setup()
		put := fake.PutBytes
		if !runKilled(t, fake, "preupload", "write", n, "R3V_KILL_DIR="+r.Root) {
			t.Fatalf("%s: it finished", what)
		}
		first := fake.PutBytes - put
		r.CleanPreuploads() // R3V starting again
		if n := leftIn(filepath.Join(r.Dir, preuploadDir)); n != 0 {
			t.Fatalf("%s: %d files left", what, n)
		}
		cands, _ := r.PreuploadCandidates(time.Now())
		if len(cands) != 1 {
			t.Fatalf("%s: not to go up again: %+v", what, cands)
		}
		c, _ := r.Client()
		put = fake.PutBytes
		if err := r.Preupload(c, cands[0], nil, nil); err != nil {
			t.Fatalf("%s: going up again: %v", what, err)
		}
		if again := fake.PutBytes - put; first+again > size+64<<10 {
			t.Errorf("%s: sent %d KB, then %d KB, for %d KB", what, first>>10, again>>10, size>>10)
		}
	}
}
