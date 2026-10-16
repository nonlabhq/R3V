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
	check("status", st.Warnings)
	if _, text := runText(t, a, "status"); !strings.Contains(text, "note: build-win/ is named like build/, which the rules leave out (2 files,") {
		t.Errorf("status text:\n%s", text)
	}

	exit, rep = run(t, a, "commit", "-m", "a build")
	var cm commitJSON
	json.Unmarshal(rep.Result, &cm)
	if exit != 0 || cm.Saved == nil {
		t.Fatalf("commit refused: %s %+v", rep.Result, rep.Error)
	}
	check("commit", cm.Warnings)

	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("two"), 0o644)
	_, rep = run(t, a, "status")
	st = statusJSON{}
	json.Unmarshal(rep.Result, &st)
	if st.Warnings == nil || len(st.Warnings) != 0 {
		t.Errorf("status after: %s", rep.Result)
	}
	_, rep = run(t, a, "commit", "-m", "notes")
	cm = commitJSON{}
	json.Unmarshal(rep.Result, &cm)
	if cm.Warnings == nil || len(cm.Warnings) != 0 {
		t.Errorf("commit after: %s", rep.Result)
	}
}
