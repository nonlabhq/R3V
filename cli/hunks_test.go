package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// A preset merging .ts as text, as the code preset (Nightly) does.
func init() {
	if err := profile.RegisterPreset([]byte("name: zz-cli-hunks\npriority: -9\nhandlers:\n  - files: [\"*.ts\"]\n    merge: text\n")); err != nil {
		panic(err)
	}
}

// lines10 is a file of ten lines, those named replaced.
func lines10(change map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		if s, ok := change[i]; ok {
			b.WriteString(s + "\n")
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	return b.String()
}

func TestHunkConflictsJSON(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := filepath.Join(t.TempDir(), "app")
	os.MkdirAll(filepath.Join(a, "src"), 0o755)
	os.WriteFile(filepath.Join(a, ".r3v.yaml"), []byte("presets:\n  ./: zz-cli-hunks\n"), 0o644)
	app := filepath.Join("src", "app.ts")
	os.WriteFile(filepath.Join(a, app), []byte(lines10(nil)), 0o644)
	ra, err := project.Init(a, "yi")
	if err != nil {
		t.Fatal(err)
	}
	if err := ra.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "-m", "first"); exit != 0 {
		t.Fatalf("first commit: %+v", rep.Error)
	}
	b := filepath.Join(t.TempDir(), "b", "app")
	if _, _, err := project.Clone(code, ra.Config.Name, b, "alex"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(a, app), []byte(lines10(map[int]string{2: "a2", 8: "a8"})), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "a"); exit != 0 {
		t.Fatalf("a's commit: %+v", rep.Error)
	}
	mine := lines10(map[int]string{2: "b2", 5: "b5", 8: "b8"})
	os.WriteFile(filepath.Join(b, app), []byte(mine), 0o644)

	// Each hunk is a conflict of its own, with its lines.
	exit, rep := run(t, b, "commit", "-m", "b")
	if exit != 3 || rep.Error.Code != "merge_conflict" || len(rep.Error.Conflicts) != 2 {
		t.Fatalf("hunk conflicts: %d %+v", exit, rep.Error)
	}
	c := rep.Error.Conflicts[0]
	if c.Key != "text:src/app.ts#1" || c.File != "src/app.ts" || c.Unit != "line 2" || c.Hunk != 1 || !c.CanKeepBoth ||
		c.Base == nil || c.Base.Text != "line 2\n" || c.Ours.Text != "b2\n" || c.Theirs.Text != "a2\n" || c.Ours.Start != 2 {
		t.Fatalf("first hunk: %+v", c)
	}
	var raw struct {
		Error struct{ Conflicts []map[string]any } `json:"error"`
	}
	exit, out := runRaw(t, b, "commit", "-m", "b")
	json.Unmarshal(out, &raw)
	if exit != 3 || raw.Error.Conflicts[1]["key"] != "text:src/app.ts#2" || raw.Error.Conflicts[1]["hunk"] != float64(2) ||
		raw.Error.Conflicts[1]["ours"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("hunk JSON: %s", out)
	}

	// Bad keys and choices are usage errors.
	for _, args := range [][]string{
		{"--resolve", "nope=ours"}, {"--resolve", "text:src/app.ts#1=mine"}, {"--resolve", "text:src/app.ts#x=ours"},
		{"--resolved", "../x.ts=" + filepath.Join(b, app)}, {"--decisions", filepath.Join(b, "missing.json")},
	} {
		if exit, rep := run(t, b, append([]string{"commit", "-m", "b"}, args...)...); exit != 2 || rep.Error.Code != "usage" {
			t.Errorf("%v: %d %+v", args, exit, rep.Error)
		}
	}

	// One decided: the other is left.
	exit, rep = run(t, b, "commit", "-m", "b", "--resolve", "text:src/app.ts#1=theirs")
	if exit != 3 || len(rep.Error.Conflicts) != 1 || rep.Error.Conflicts[0].Key != "text:src/app.ts#2" {
		t.Fatalf("one left: %d %+v", exit, rep.Error)
	}
	if got, _ := os.ReadFile(filepath.Join(b, app)); string(got) != mine {
		t.Fatalf("a conflict changed the file: %q", got)
	}

	// Both decided (one from a decisions file on stdin): committed.
	stdin := filepath.Join(t.TempDir(), "decisions.json")
	os.WriteFile(stdin, []byte(`{"text:src/app.ts#2": "both"}`), 0o644)
	f, _ := os.Open(stdin)
	defer f.Close()
	saved := os.Stdin
	os.Stdin = f
	exit, rep = run(t, b, "commit", "-m", "b", "--resolve", "text:src/app.ts#1=theirs", "--decisions", "-")
	os.Stdin = saved
	var sv syncJSON
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || sv.Action != "published" {
		t.Fatalf("decided: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if got, want := readFile(t, filepath.Join(b, app)), lines10(map[int]string{2: "a2", 5: "b5", 8: "b8\na8"}); got != want {
		t.Errorf("merged:\n%s\nwant:\n%s", got, want)
	}

	// The file given as it should be, on an update keeping the work.
	if exit, rep := run(t, a, "update"); exit != 0 {
		t.Fatalf("a's update: %+v", rep.Error)
	}
	os.WriteFile(filepath.Join(a, app), []byte("from a\n"), 0o644)
	run(t, a, "commit", "-m", "a again")
	os.WriteFile(filepath.Join(b, app), []byte("from b\n"), 0o644)
	given := filepath.Join(t.TempDir(), "given.ts")
	os.WriteFile(given, []byte("from a and b\n"), 0o644)
	exit, rep = run(t, b, "update", "--resolved", "src/app.ts="+given)
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || !sv.KeptWork {
		t.Fatalf("update --resolved: %d %+v %s", exit, rep.Error, rep.Result)
	}
	if got := readFile(t, filepath.Join(b, app)); got != "from a and b\n" {
		t.Errorf("given: %q", got)
	}
}

// runRaw runs r3v in dir with --json; returns the exit status and stdout.
func runRaw(t *testing.T, dir string, args ...string) (int, []byte) {
	t.Helper()
	t.Chdir(dir)
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	code := Run(append(args, "--json"))
	w.Close()
	os.Stdout = stdout
	return code, <-done
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
