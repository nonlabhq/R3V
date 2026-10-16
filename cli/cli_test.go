package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/remote/s3test"
)

// Tests must not touch the user's real team store.
func TestMain(m *testing.M) {
	if os.Getenv("R3V_CLI_KILL_ARGS") != "" {
		os.Exit(killedChild())
	}
	dir, err := os.MkdirTemp("", "r3v-cli-")
	if err != nil {
		panic(err)
	}
	os.Setenv("R3V_CONFIG_DIR", dir)
	os.Setenv(actor.EnvTool, actor.None)                              // run by an agent or not, the same results
	openTool = func(*project.Repo) (string, string) { return "", "" } // no tool has a project open
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type reply struct {
	Schema  int             `json:"schema"`
	OK      bool            `json:"ok"`
	Command string          `json:"command"`
	Result  json.RawMessage `json:"result"`
	Error   *errorJSON      `json:"error"`
}

// run runs r3v in dir with --json; returns the exit status and the reply.
func run(t *testing.T, dir string, args ...string) (int, reply) {
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
	out := <-done
	var rep reply
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("r3v %s: not one JSON object: %v\n%s", strings.Join(args, " "), err, out)
	}
	if rep.Schema != 1 || rep.OK != (code == 0) || (rep.Error != nil && rep.Error.Exit != code) {
		t.Fatalf("r3v %s: envelope %+v, exit %d", strings.Join(args, " "), rep, code)
	}
	return code, rep
}

func newFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Song Project")
	os.MkdirAll(filepath.Join(dir, "Ableton Project Info"), 0o755)
	for p, c := range files {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644)
	}
	return dir
}

func TestJSON(t *testing.T) {
	fake := s3test.New("team")
	defer fake.Close()
	code := remote.EncodeConnectionCode(remote.Config{URL: "s3+" + fake.URL + "/team/r3v",
		AccessKey: "key", SecretKey: "secret"})
	a := newFolder(t, map[string]string{"Samples/kick.wav": "kick", "notes.txt": "first"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}

	// Before anything is saved.
	exit, rep := run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || st.Version != "" || st.Team != nil || len(st.Changes) == 0 {
		t.Fatalf("status: %s", rep.Result)
	}
	exit, rep = run(t, a, "save", "-m", "first")
	var sv syncJSON
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || sv.Action != "local" || sv.Saved == nil || sv.Saved.Message != "first" {
		t.Fatalf("save without a team: %s %+v", rep.Result, rep.Error)
	}

	// In a team, with a teammate.
	if exit, rep := run(t, a, "remote", code); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("remote has no --json: %+v", rep)
	}
	ra, _ := project.Open(a)
	if err := ra.SetRemote(code); err != nil {
		t.Fatal(err)
	}
	exit, rep = run(t, a, "commit", "-m", "shared")
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || sv.Action != "published" {
		t.Fatalf("save: %s", rep.Result)
	}
	b := filepath.Join(t.TempDir(), "B", "Song Project")
	if _, _, err := project.Clone(code, ra.Config.Name, b, "alex"); err != nil {
		t.Fatal(err)
	}
	exit, rep = run(t, a, "log", "-n", "1")
	var lg struct{ Versions []versionJSON }
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 1 || lg.Versions[0].Message != "first" || len(lg.Versions[0].Branches) != 1 {
		t.Fatalf("log: %s", rep.Result)
	}

	// Both change notes.txt: B is asked.
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("A's notes"), 0o644)
	run(t, a, "save", "-m", "A's notes")
	os.WriteFile(filepath.Join(b, "notes.txt"), []byte("B's notes"), 0o644)
	exit, rep = run(t, b, "status")
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || st.Team == nil || !st.Team.Reachable || !st.Team.Incoming || len(st.Changes) != 1 ||
		st.Changes[0].Status != "modified" {
		t.Fatalf("status with the team ahead: %s", rep.Result)
	}
	// Uncommitted work is kept through an update, unless both changed a file.
	if exit, rep := run(t, b, "update"); exit != 3 || rep.Error.Code != "merge_conflict" {
		t.Fatalf("update with changes: %d %+v", exit, rep.Error)
	}
	exit, rep = run(t, b, "update", "--preview")
	var pv previewJSON
	json.Unmarshal(rep.Result, &pv)
	if exit != 0 || len(pv.Versions) != 1 || len(pv.Changes) != 1 {
		t.Fatalf("preview: %s", rep.Result)
	}
	exit, rep = run(t, b, "commit", "-m", "B's notes")
	if exit != 3 || rep.Error.Code != "merge_conflict" || len(rep.Error.Conflicts) != 1 ||
		rep.Error.Conflicts[0].File != "notes.txt" || !strings.Contains(rep.Error.Hint, "--strategy") {
		t.Fatalf("conflict: %d %+v", exit, rep.Error)
	}

	// The set is open in Live: B is told, nothing changes.
	openTool = func(*project.Repo) (string, string) { return "ableton-live", "Song.als" }
	exit, rep = run(t, b, "commit", "-m", "B's notes", "--strategy", "theirs")
	if exit != 4 || rep.Error.Code != "set_open_in_live" || rep.Error.Set != "Song.als" {
		t.Fatalf("Live open: %d %+v", exit, rep.Error)
	}
	// Another tool's editor: the same, said for it.
	openTool = func(*project.Repo) (string, string) { return "unity-editor", "?" }
	exit, rep = run(t, b, "commit", "-m", "B's notes", "--strategy", "theirs")
	openTool = func(*project.Repo) (string, string) { return "", "" }
	if exit != 4 || rep.Error.Code != "files_open_in_tool" || !strings.Contains(rep.Error.Message, "Unity Editor") {
		t.Fatalf("Unity open: %d %+v", exit, rep.Error)
	}
	exit, rep = run(t, b, "commit", "-m", "B's notes", "--strategy", "theirs")
	json.Unmarshal(rep.Result, &sv)
	// Theirs taken: nothing of B's left to commit, B is just up to date.
	if exit != 0 || sv.Action != "fast-forward" || sv.Saved != nil || len(sv.Merged) == 0 {
		t.Fatalf("save with theirs: %s", rep.Result)
	}
	if got, _ := os.ReadFile(filepath.Join(b, "notes.txt")); string(got) != "A's notes" {
		t.Errorf("theirs kept %q", got)
	}

	// A commit on this computer only: the next commit shares it.
	os.WriteFile(filepath.Join(b, "local.txt"), []byte("here"), 0o644)
	exit, rep = run(t, b, "commit", "--local", "-m", "here only")
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || rep.Command != "commit" || sv.Action != "local" || sv.Saved == nil || sv.Saved.Message != "here only" {
		t.Fatalf("commit --local: %s %+v", rep.Result, rep.Error)
	}
	exit, rep = run(t, b, "status")
	json.Unmarshal(rep.Result, &st)
	if exit != 0 || len(st.Changes) != 0 {
		t.Fatalf("status after commit --local: %s", rep.Result)
	}
	exit, rep = run(t, b, "commit", "-m", "nothing new")
	json.Unmarshal(rep.Result, &sv)
	if exit != 0 || sv.Action != "published" {
		t.Fatalf("the next commit: %s", rep.Result)
	}
	ra.Update(project.Strategy("fail"))
	if got, _ := os.ReadFile(filepath.Join(a, "local.txt")); string(got) != "here" {
		t.Errorf("A has %q", got)
	}
}

func TestErrors(t *testing.T) {
	empty := t.TempDir()
	if exit, rep := run(t, empty, "status"); exit != 6 || rep.Error.Code != "not_a_project" {
		t.Fatalf("not a project: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, empty, "commit", "--bogus"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("unknown flag: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, empty, "commit"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("no message: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, empty, "frobnicate"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("unknown command: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, empty, "version"); exit != 0 || !strings.Contains(string(rep.Result), `"version"`) {
		t.Fatalf("version: %d %s", exit, rep.Result)
	}
}

func TestEditPath(t *testing.T) {
	id := func(s string) string { return strings.ReplaceAll(s, "%LOCALAPPDATA%", `C:\Users\yi\AppData\Local`) }
	bin := `C:\Users\yi\AppData\Local\Programs\R3V\bin`
	for _, c := range []struct {
		in   string
		add  bool
		want string
		ch   bool
	}{
		{"", true, bin, true},
		{`C:\Go\bin;C:\tools`, true, `C:\Go\bin;C:\tools;` + bin, true},
		{`C:\Go\bin;`, true, `C:\Go\bin;` + bin, true},
		{`C:\Go\bin;%LOCALAPPDATA%\Programs\R3V\bin\`, true, `C:\Go\bin;%LOCALAPPDATA%\Programs\R3V\bin\`, false},
		{`C:\Go\bin;c:\users\YI\appdata\local\programs\r3v\BIN;C:\tools`, false, `C:\Go\bin;C:\tools`, true},
		{`C:\Go\bin;%LOCALAPPDATA%\Programs\R3V\bin`, false, `C:\Go\bin`, true},
		{`C:\Go\bin;C:\Users\yi\AppData\Local\Programs\R3V Pro\bin`, false, `C:\Go\bin;C:\Users\yi\AppData\Local\Programs\R3V Pro\bin`, false},
	} {
		got, ch := editPath(c.in, bin, c.add, id)
		if got != c.want || ch != c.ch {
			t.Errorf("editPath(%q, add=%v) = %q %v, want %q %v", c.in, c.add, got, ch, c.want, c.ch)
		}
	}
}

// goto goes to a version and back (checkout: its name before).
func TestGoto(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "first"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("second"), 0o644)
	run(t, a, "commit", "-m", "second")
	exit, rep := run(t, a, "goto", "HEAD~1")
	var mv moveJSON
	json.Unmarshal(rep.Result, &mv)
	if exit != 0 || rep.Command != "goto" || !mv.Older {
		t.Fatalf("goto: %s %+v", rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "checkout", "latest")
	json.Unmarshal(rep.Result, &mv)
	if exit != 0 || mv.Older {
		t.Fatalf("checkout latest: %s %+v", rep.Result, rep.Error)
	}
}

// discard throws the changes away (the app's "Discard changes"), staying
// on an older version when on one.
func TestDiscard(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "first"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("second"), 0o644)
	run(t, a, "commit", "-m", "second")
	run(t, a, "goto", "HEAD~1")
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("changed"), 0o644)
	os.WriteFile(filepath.Join(a, "new.txt"), []byte("new"), 0o644)
	exit, rep := run(t, a, "discard")
	var mv moveJSON
	json.Unmarshal(rep.Result, &mv)
	if exit != 0 || rep.Command != "discard" || !mv.Older {
		t.Fatalf("discard: %s %+v", rep.Result, rep.Error)
	}
	if data, _ := os.ReadFile(filepath.Join(a, "notes.txt")); string(data) != "first" {
		t.Errorf("notes.txt: %q", data)
	}
	if _, err := os.Stat(filepath.Join(a, "new.txt")); err == nil {
		t.Error("an added file left")
	}
	_, rep = run(t, a, "parked")
	if strings.Contains(string(rep.Result), "notes.txt") {
		t.Errorf("parked instead: %s", rep.Result)
	}
}

// discard with paths throws away only the changes in those files and
// folders (an added one removed, a moved one back where it was); --preview
// lists them and changes nothing; the others stay.
func TestDiscardPaths(t *testing.T) {
	a := newFolder(t, map[string]string{"keep.txt": "one", "scenes/a.txt": "a", "scenes/b.txt": "b"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.WriteFile(filepath.Join(a, "keep.txt"), []byte("mine"), 0o644)
	os.WriteFile(filepath.Join(a, "scenes", "a.txt"), []byte("oops"), 0o644)
	os.Rename(filepath.Join(a, "scenes", "b.txt"), filepath.Join(a, "scenes", "c.txt"))
	os.WriteFile(filepath.Join(a, "scenes", "new.txt"), []byte("new"), 0o644)

	exit, rep := run(t, a, "discard", "--preview", "scenes")
	var out discardJSON
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || !out.Preview || len(out.Discarded) != 3 {
		t.Fatalf("preview: %s %+v", rep.Result, rep.Error)
	}
	if data, _ := os.ReadFile(filepath.Join(a, "scenes", "a.txt")); string(data) != "oops" {
		t.Fatal("the preview changed a file")
	}

	exit, rep = run(t, a, "discard", "scenes")
	out = discardJSON{}
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Preview || len(out.Discarded) != 3 {
		t.Fatalf("discard: %s %+v", rep.Result, rep.Error)
	}
	for p, want := range map[string]string{"keep.txt": "mine", "scenes/a.txt": "a", "scenes/b.txt": "b"} {
		if data, _ := os.ReadFile(filepath.Join(a, p)); string(data) != want {
			t.Errorf("%s: %q, want %q", p, data, want)
		}
	}
	for _, p := range []string{"scenes/c.txt", "scenes/new.txt"} {
		if _, err := os.Stat(filepath.Join(a, p)); err == nil {
			t.Errorf("%s left", p)
		}
	}

	if exit, rep := run(t, a, "discard", "scenes"); exit == 0 || rep.Error == nil || rep.Error.Code != "nothing_to_discard" {
		t.Fatalf("nothing left there: %d %+v", exit, rep.Error)
	}
}

// commit with paths commits only the changes in those files and folders;
// the others stay uncommitted.
func TestCommitPaths(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "one", "src/a.txt": "a", "src/b.txt": "b"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("two"), 0o644)
	os.WriteFile(filepath.Join(a, "src", "a.txt"), []byte("a2"), 0o644)
	os.Rename(filepath.Join(a, "src", "b.txt"), filepath.Join(a, "src", "c.txt"))
	exit, rep := run(t, a, "commit", "-m", "src only", "src")
	if exit != 0 {
		t.Fatalf("commit src: %+v", rep.Error)
	}
	_, rep = run(t, a, "status")
	if s := string(rep.Result); !strings.Contains(s, "notes.txt") || strings.Contains(s, "a.txt") || strings.Contains(s, "c.txt") {
		t.Fatalf("left: %s", s)
	}
	exit, rep = run(t, filepath.Join(a, "src"), "commit", "-m", "nothing here", ".")
	if exit == 0 || rep.Error == nil || rep.Error.Code != "nothing_to_commit" {
		t.Fatalf("nothing in src: %d %+v", exit, rep.Error)
	}
	exit, rep = run(t, a, "commit", "-m", "notes", "notes.txt")
	if _, rep2 := run(t, a, "status"); exit != 0 || strings.Contains(string(rep2.Result), "notes.txt") {
		t.Fatalf("notes: %+v %s", rep.Error, rep2.Result)
	}
}
