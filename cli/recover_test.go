package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// killedChild runs r3v in the child (this test binary): R3V_CLI_KILL_ARGS
// the command ("|" between its words), in R3V_CLI_KILL_DIR, killed (exit 3:
// nothing deferred runs) once R3V_CLI_KILL_WRITES files of a switch are in
// place.
func killedChild() int {
	os.Chdir(os.Getenv("R3V_CLI_KILL_DIR"))
	os.Setenv(actor.EnvTool, actor.None)
	openTool = func(*project.Repo) (string, string) { return "", "" }
	n, _ := strconv.Atoi(os.Getenv("R3V_CLI_KILL_WRITES"))
	written := 0
	project.OnFileWritten(func(string) {
		if written++; written >= n {
			os.Exit(3)
		}
	})
	return Run(strings.Split(os.Getenv("R3V_CLI_KILL_ARGS"), "|"))
}

// runKilledCLI runs r3v args in dir in a child killed after writes files:
// whether it was (it didn't finish).
func runKilledCLI(t *testing.T, dir string, writes int, args ...string) bool {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "R3V_CLI_KILL_DIR="+dir, fmt.Sprintf("R3V_CLI_KILL_WRITES=%d", writes),
		"R3V_CLI_KILL_ARGS="+strings.Join(append(args, "--json"), "|"))
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 3) {
		t.Fatalf("r3v %s: %v", strings.Join(args, " "), err)
	}
	return err != nil
}

// folderFiles: the project's files (not R3V's .r3v) and their contents.
func folderFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".r3v" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return out
}

// switchedProject: a team project on version one, two (a file changed,
// added, removed) the latest; the files of each.
func switchedProject(t *testing.T) (dir string, fake *s3test.Server, one, two string, before, after map[string]string) {
	t.Helper()
	fake = s3test.New("team")
	t.Cleanup(fake.Close)
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	big := func(c string) string { return strings.Repeat(c, 64<<10) }
	dir = newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "one",
		"art/a.bin": big("a"), "art/b.bin": big("b"), "docs/old.txt": "old"})
	r, err := project.Init(dir, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	commit := func(msg string) string {
		t.Helper()
		exit, rep := run(t, dir, "commit", "-m", msg)
		var out syncJSON
		json.Unmarshal(rep.Result, &out)
		if exit != 0 || out.Saved == nil {
			t.Fatalf("commit %s: %d %+v", msg, exit, rep.Error)
		}
		return out.Saved.ID
	}
	one = commit("one")
	before = folderFiles(t, dir)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("two"), 0o644)
	os.WriteFile(filepath.Join(dir, "art", "a.bin"), []byte(big("A")), 0o644)
	os.WriteFile(filepath.Join(dir, "art", "c.bin"), []byte(big("c")), 0o644)
	os.MkdirAll(filepath.Join(dir, "mix"), 0o755)
	os.WriteFile(filepath.Join(dir, "mix", "bounce.bin"), []byte(big("m")), 0o644)
	os.RemoveAll(filepath.Join(dir, "docs"))
	two = commit("two")
	after = folderFiles(t, dir)
	if exit, rep := run(t, dir, "goto", one); exit != 0 {
		t.Fatalf("goto one: %+v", rep.Error)
	}
	return
}

// statusAfter runs r3v status, counting what it sends to the team.
func statusAfter(t *testing.T, dir string, fake *s3test.Server) (statusResult, int, reply) {
	t.Helper()
	var writes atomic.Int32
	fake.OnWrite = func(_, _ string) { writes.Add(1) }
	defer func() { fake.OnWrite = nil }()
	exit, rep := run(t, dir, "status")
	if exit != 0 {
		t.Fatalf("status: %d %+v", exit, rep.Error)
	}
	var st statusResult
	json.Unmarshal(rep.Result, &st)
	return st, int(writes.Load()), rep
}

type statusResult struct {
	Version    string
	Changes    []changeJSON
	Busy       bool
	Recovered  *recoveredJSON
	Unfinished *unfinishedJSON
}

// A CLI goto killed while it rewrites the files, at every file: the next
// command (status, here) puts them back as the version the project is on,
// says so (recovered), sends nothing, and leaves no record; going there
// again works.
func TestKilledGotoRecovered(t *testing.T) {
	if testing.Short() {
		t.Skip("kills switches")
	}
	dir, _, _, two, _, _ := switchedProject(t)
	steps := 0
	project.OnFileWritten(func(string) { steps++ })
	if exit, rep := run(t, dir, "goto", two); exit != 0 {
		t.Fatalf("goto two: %+v", rep.Error)
	}
	project.OnFileWritten(nil)
	if steps < 3 {
		t.Fatalf("the switch wrote %d files", steps)
	}
	for n := 1; n <= steps; n++ {
		what := fmt.Sprintf("killed after %d of %d files", n, steps)
		dir, fake, one, two, before, after := switchedProject(t)
		if !runKilledCLI(t, dir, n, "goto", two) {
			t.Fatalf("%s: it finished", what)
		}
		if got := folderFiles(t, dir); maps.Equal(got, before) {
			t.Fatalf("%s: nothing changed: not half-way", what)
		}
		st, writes, rep := statusAfter(t, dir, fake)
		if st.Recovered == nil || st.Recovered.Action != "restored" || st.Recovered.Version != one || st.Unfinished != nil {
			t.Fatalf("%s: status: %s", what, rep.Result)
		}
		if got := folderFiles(t, dir); !maps.Equal(got, before) || st.Version != one || len(st.Changes) != 0 {
			t.Fatalf("%s: after status: on %s, %d changes, files %v", what, short(st.Version), len(st.Changes), slices.Sorted(maps.Keys(got)))
		}
		if _, err := os.Stat(filepath.Join(dir, ".r3v", "switching")); !os.IsNotExist(err) {
			t.Fatalf("%s: the record stays: %v", what, err)
		}
		if writes != 0 {
			t.Errorf("%s: putting the files back sent %d writes to the team", what, writes)
		}
		if st, _, rep := statusAfter(t, dir, fake); st.Recovered != nil {
			t.Fatalf("%s: recovered again: %s", what, rep.Result)
		}
		if exit, rep := run(t, dir, "goto", two); exit != 0 || !maps.Equal(folderFiles(t, dir), after) {
			t.Fatalf("%s: goto again: %d %+v", what, exit, rep.Error)
		}
	}
}

// A commit after a goto killed half-way, nothing changed since: the files
// are put back first (recovered), then it commits as ever; nothing of the
// half-written files is committed.
func TestKilledGotoThenCommit(t *testing.T) {
	dir, _, one, two, _, before := switchedProject(t)
	if exit, rep := run(t, dir, "goto", "latest"); exit != 0 { // (commits go on the latest)
		t.Fatalf("goto latest: %+v", rep.Error)
	}
	if !runKilledCLI(t, dir, 2, "goto", one) {
		t.Fatal("it finished")
	}
	exit, rep := run(t, dir, "commit", "-m", "x")
	var out struct {
		Action    string
		Saved     *versionJSON
		Recovered *recoveredJSON
	}
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Recovered == nil || out.Recovered.Version != two || out.Saved != nil || out.Action != "nothing-changed" {
		t.Fatalf("commit: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if !maps.Equal(folderFiles(t, dir), before) {
		t.Fatal("the files aren't back as they were")
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("three"), 0o644)
	exit, rep = run(t, dir, "commit", "-m", "three")
	out.Saved, out.Recovered = nil, nil
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Saved == nil || out.Recovered != nil {
		t.Fatalf("commit after: %d %+v %s", exit, rep.Error, rep.Result)
	}
}

// What stops the next command putting the files back: the app holding the
// project (a reading command reads it as it is, a writing one waits), a
// tool with it open, a file changed since. Each leaves the files and the
// record as they are, and says why (unfinished); once it's gone, they are
// put back.
func TestKilledGotoNotRecovered(t *testing.T) {
	dir, fake, one, two, before, _ := switchedProject(t)
	if !runKilledCLI(t, dir, 2, "goto", two) {
		t.Fatal("it finished")
	}
	half := folderFiles(t, dir)
	record := filepath.Join(dir, ".r3v", "switching")
	untouched := func(what string) {
		t.Helper()
		if got := folderFiles(t, dir); !maps.Equal(got, half) {
			t.Fatalf("%s: the files changed: %v", what, slices.Sorted(maps.Keys(got)))
		}
		if _, err := os.Stat(record); err != nil {
			t.Fatalf("%s: the record went: %v", what, err)
		}
	}

	// The app has the project.
	app, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	project.LockProgram, project.LockCommand = "app", ""
	release, err := app.Lock(0)
	if err != nil {
		t.Fatal(err)
	}
	if st, _, rep := statusAfter(t, dir, fake); !st.Busy || st.Recovered != nil || st.Unfinished != nil {
		t.Fatalf("status while the app has it: %s", rep.Result)
	}
	if exit, rep := run(t, dir, "commit", "-m", "x", "--wait", "0"); exit != exitLater || rep.Error.Recovered != nil {
		t.Fatalf("commit while the app has it: %d %+v", exit, rep.Error)
	}
	untouched("the app has it")
	release()

	// Live has a set open.
	openTool = func(*project.Repo) (string, string) { return "ableton-live", "Song.als" }
	st, _, rep := statusAfter(t, dir, fake)
	if exit, _ := runText(t, dir, "status"); exit != 0 { // (its note on stderr)
		t.Fatalf("status in text: %d", exit)
	}
	openTool = func(*project.Repo) (string, string) { return "", "" }
	if st.Recovered != nil || st.Unfinished == nil || st.Unfinished.Code != "set_open_in_live" || st.Unfinished.Version != one ||
		!strings.Contains(st.Unfinished.Message, `"Song.als"`) {
		t.Fatalf("status with Live open: %s", rep.Result)
	}
	untouched("Live has it open")

	// A file new since, and one changed: theirs to decide.
	os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("kept"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("edited after"), 0o644)
	half = folderFiles(t, dir)
	exit, rep := run(t, dir, "commit", "-m", "x")
	if exit == 0 || rep.Error.Code != "switch_unfinished" || rep.Error.Unfinished == nil || rep.Error.Unfinished.Code != "changed_since_switch" ||
		!slices.Equal(slices.Sorted(slices.Values(rep.Error.Unfinished.Paths)), []string{"mine.txt", "notes.txt"}) {
		t.Fatalf("commit with files changed since: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, dir, "commit", "-m", "x", "--local"); exit == 0 || rep.Error.Code != "switch_unfinished" {
		t.Fatalf("commit --local with files changed since: %d %+v", exit, rep.Error)
	}
	untouched("files changed since")
	if r, err := project.Open(dir); err != nil || r.Head() != one {
		t.Fatalf("a refused commit moved the project: %v", err)
	}

	// Taken away: put back.
	os.Remove(filepath.Join(dir, "mine.txt"))
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(before["notes.txt"]), 0o644)
	if st, _, rep := statusAfter(t, dir, fake); st.Recovered == nil || st.Unfinished != nil || !maps.Equal(folderFiles(t, dir), before) {
		t.Fatalf("status at last: %s", rep.Result)
	}
}
