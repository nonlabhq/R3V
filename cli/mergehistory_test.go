package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runText runs r3v in dir without --json; returns the exit status and
// what it printed.
func runText(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(dir)
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() { b, _ := io.ReadAll(r); done <- b }()
	code := Run(args)
	w.Close()
	os.Stdout = stdout
	return code, string(<-done)
}

// laneProject: a team project (on main) with branch vfx three versions
// ahead (one unfinished), main one ahead of where vfx started.
func laneProject(t *testing.T) string {
	t.Helper()
	a := teamProject(t)
	if exit, text := runText(t, a, "branch", "new", "vfx"); exit != 0 {
		t.Fatalf("branch new: %s", text)
	}
	for i, title := range []string{"Add bloom pass", "WIP glow", "Tune shadow cascades"} {
		os.WriteFile(filepath.Join(a, "src", "vfx.ts"), []byte(strings.Repeat("x", i+1)), 0o644)
		if exit, rep := run(t, a, "commit", "-m", title); exit != 0 {
			t.Fatalf("commit: %+v", rep.Error)
		}
	}
	if exit, rep := run(t, a, "switch", "main"); exit != 0 {
		t.Fatalf("switch: %+v", rep.Error)
	}
	os.WriteFile(filepath.Join(a, "src", "main2.ts"), []byte("main"), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "Main moves on"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	return a
}

// A merge's preview has the suggested description and marks unfinished
// versions; merging without -m stores it; log --first-parent lists the
// main line, merges with how many versions they brought in.
func TestMergeMessageAndFirstParentLog(t *testing.T) {
	a := laneProject(t)
	want := "Merge vfx: 3 versions\n- Add bloom pass\n- Tune shadow cascades"
	exit, rep := run(t, a, "merge", "vfx", "--preview")
	var pv previewJSON
	json.Unmarshal(rep.Result, &pv)
	if exit != 0 || pv.Message != want || len(pv.Versions) != 3 {
		t.Fatalf("preview: %d %s", exit, rep.Result)
	}
	for _, v := range pv.Versions {
		if v.WIP != (v.Message == "WIP glow") {
			t.Fatalf("wip: %s", rep.Result)
		}
	}
	if !strings.Contains(string(rep.Result), `"wip": true`) {
		t.Fatalf("no wip field: %s", rep.Result)
	}
	if exit, rep := run(t, a, "merge", "vfx"); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}

	exit, rep = run(t, a, "log", "--first-parent")
	var lg struct{ Versions []versionJSON }
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 3 || lg.Versions[0].Message != want || lg.Versions[0].MergedCount != 3 ||
		lg.Versions[1].Message != "Main moves on" || lg.Versions[1].MergedCount != 0 || lg.Versions[2].Message != "first" {
		t.Fatalf("log --first-parent: %s", rep.Result)
	}
	if strings.Contains(string(rep.Result), `"merged_count": 0`) {
		t.Fatalf("merged_count on a version that isn't a merge: %s", rep.Result)
	}
	if exit, text := runText(t, a, "log", "--first-parent"); exit != 0 || !strings.Contains(text, "Merge vfx: 3 versions  (+3 versions)") {
		t.Fatalf("log text: %s", text)
	}
	// The whole log is as before: every version.
	exit, rep = run(t, a, "log")
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 6 || strings.Contains(string(rep.Result), "merged_count") {
		t.Fatalf("log: %s", rep.Result)
	}
	// Another branch's main line: its own versions.
	exit, rep = run(t, a, "log", "vfx", "--first-parent", "-n", "2")
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || len(lg.Versions) != 2 || lg.Versions[0].Message != "Tune shadow cascades" {
		t.Fatalf("log vfx: %s", rep.Result)
	}

	// -m: exactly that.
	if exit, rep := run(t, a, "switch", "vfx"); exit != 0 {
		t.Fatalf("switch: %+v", rep.Error)
	}
	os.WriteFile(filepath.Join(a, "src", "vfx.ts"), []byte("flare"), 0o644)
	run(t, a, "commit", "-m", "Add lens flare")
	run(t, a, "switch", "main")
	if exit, rep := run(t, a, "merge", "vfx", "-m", "Bring in the flare"); exit != 0 {
		t.Fatalf("merge -m: %+v", rep.Error)
	}
	exit, rep = run(t, a, "log", "-n", "1")
	json.Unmarshal(rep.Result, &lg)
	if exit != 0 || lg.Versions[0].Message != "Bring in the flare" {
		t.Fatalf("log: %s", rep.Result)
	}
}
