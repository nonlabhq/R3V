package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/profile"
	"github.com/nonlabhq/r3v/internal/project"
)

// A new folder named like one the rules leave out: status warns before,
// the commit goes ahead and says it too; then nothing more to say.
func TestLooksBuiltWarnings(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "one",
		profile.FileName: "rules:\n  - ignore: \"**/build/\"\n"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.MkdirAll(filepath.Join(a, "build-win", "x64"), 0o755)
	os.WriteFile(filepath.Join(a, "build-win", "x64", "app.exe"), []byte("built"), 0o644)
	os.WriteFile(filepath.Join(a, "build-win", "log.txt"), []byte("log"), 0o644)

	want := warningJSON{Code: "looks_built", Folder: "build-win", Kind: "name", Like: "build/", Files: 2, Bytes: 8,
		Rule: `ignore: "build-*/"`, Pattern: "build-*/"}
	check := func(what string, ws []warningJSON) {
		t.Helper()
		if len(ws) != 1 || ws[0].Reason == "" {
			t.Fatalf("%s: warnings %+v", what, ws)
		}
		got := ws[0]
		got.Reason = ""
		if got != want {
			t.Errorf("%s: %+v, want %+v", what, got, want)
		}
	}
	exit, rep := run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if exit != 0 {
		t.Fatalf("status: %+v", rep.Error)
	}
	check("status", builtOnly(st.Warnings))
	if _, text := runText(t, a, "status"); !strings.Contains(text, "note: build-win/ is named like build/, which the rules leave out (2 files,") {
		t.Errorf("status text:\n%s", text)
	}

	exit, rep = run(t, a, "commit", "-m", "a build")
	var cm commitJSON
	json.Unmarshal(rep.Result, &cm)
	if exit != 0 || cm.Saved == nil {
		t.Fatalf("commit refused: %s %+v", rep.Result, rep.Error)
	}
	check("commit", builtOnly(cm.Warnings))

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("two"), 0o644)
	_, rep = run(t, a, "status")
	st = statusJSON{}
	json.Unmarshal(rep.Result, &st)
	if st.Warnings == nil || len(builtOnly(st.Warnings)) != 0 {
		t.Errorf("status after: %s", rep.Result)
	}
	_, rep = run(t, a, "commit", "-m", "notes")
	cm = commitJSON{}
	json.Unmarshal(rep.Result, &cm)
	if cm.Warnings == nil || len(builtOnly(cm.Warnings)) != 0 {
		t.Errorf("commit after: %s", rep.Result)
	}
}

// builtOnly are the looks_built warnings among ws (these projects are on
// no team: not_connected comes too).
func builtOnly(ws []warningJSON) []warningJSON {
	out := []warningJSON{}
	for _, w := range ws {
		if w.Code == "looks_built" {
			out = append(out, w)
		}
	}
	return out
}

// What a looks_built warning's pattern is for: `r3v rules ignore` adds it
// to .r3v.yaml (once), the folder is out of the changes, and it waits for
// the project as other writing commands do.
func TestRulesIgnore(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "one",
		profile.FileName: "rules:\n  - ignore: \"**/Backup/\"\n"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	os.MkdirAll(filepath.Join(a, "Backup-2024"), 0o755)
	os.WriteFile(filepath.Join(a, "Backup-2024", "Song.als"), []byte("set"), 0o644)
	_, rep := run(t, a, "status")
	var st statusJSON
	json.Unmarshal(rep.Result, &st)
	if st.Warnings = builtOnly(st.Warnings); len(st.Warnings) != 1 || st.Warnings[0].Pattern != "Backup-*/" {
		t.Fatalf("warnings %+v", st.Warnings)
	}
	if _, text := runText(t, a, "status"); !strings.Contains(text, `r3v rules ignore "Backup-*/"`) {
		t.Errorf("the note doesn't say the command:\n%s", text)
	}

	ignore := func() rulesIgnoreJSON {
		t.Helper()
		exit, rep := run(t, a, "rules", "ignore", st.Warnings[0].Pattern)
		var got rulesIgnoreJSON
		json.Unmarshal(rep.Result, &got)
		if exit != 0 || got.Rule != `ignore: "Backup-*/"` {
			t.Fatalf("rules ignore: %d %s %+v", exit, rep.Result, rep.Error)
		}
		return got
	}
	if !ignore().Added {
		t.Error("not added")
	}
	before, _ := os.ReadFile(filepath.Join(a, profile.FileName))
	if ignore().Added {
		t.Error("added twice")
	}
	if after, _ := os.ReadFile(filepath.Join(a, profile.FileName)); string(after) != string(before) ||
		strings.Count(string(after), `"Backup-*/"`) != 1 {
		t.Errorf(".r3v.yaml:\n%s", after)
	}
	_, rep = run(t, a, "status")
	st = statusJSON{}
	json.Unmarshal(rep.Result, &st)
	if len(builtOnly(st.Warnings)) != 0 {
		t.Errorf("warnings after the rule: %+v", st.Warnings)
	}
	for _, c := range st.Changes {
		if strings.HasPrefix(c.Path, "Backup-2024/") {
			t.Errorf("still a change: %s", c.Path)
		}
	}

	if exit, rep := run(t, a, "rules", "check"); exit != exitUsage {
		t.Errorf("rules check --json: %d %+v", exit, rep.Error)
	}
	if exit, _ := run(t, a, "rules", "ignore"); exit != exitUsage {
		t.Errorf("rules ignore without a pattern: %d", exit)
	}
	held, _ := project.Open(a)
	release, err := held.Lock(0)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if exit, rep := run(t, a, "rules", "ignore", "*.tmp", "--wait", "0.2"); exit != exitLater || rep.Error.Code != "project_busy" {
		t.Errorf("rules ignore while busy: %d %+v", exit, rep.Error)
	}
}
