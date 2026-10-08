package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/handlers"
	"github.com/nonlabhq/r3v/internal/livecheck"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/teams"
)

// localSong is a project the app lists (on this computer only).
func localSong(t *testing.T) string {
	t.Helper()
	t.Setenv("R3V_CONFIG_DIR", t.TempDir())
	root := newSong(t)
	if _, err := project.Init(root, "yi"); err != nil {
		t.Fatal(err)
	}
	store, err := teams.Load()
	if err != nil {
		t.Fatal(err)
	}
	store.AddLocal(root)
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Live has a set of this project open (or not: "").
func liveHas(t *testing.T, set string) {
	handlers.RegisterRunning("ableton-live", func(string) string { return set })
	t.Cleanup(func() { handlers.RegisterRunning("ableton-live", livecheck.OpenSet) })
}

func TestRenameFile(t *testing.T) {
	root := localSong(t)
	liveHas(t, "")
	a := NewApp()
	os.WriteFile(filepath.Join(root, "Samples", "snare.wav"), []byte("RIFF-snare"), 0o644)
	os.MkdirAll(filepath.Join(root, ".r3v-not"), 0o755)

	got, err := a.RenameFile(root, "Samples/kick.wav", "Kick 1.wav")
	if err != nil || got != "Samples/Kick 1.wav" {
		t.Fatalf("rename: %q, %v", got, err)
	}
	if read(t, filepath.Join(root, "Samples", "Kick 1.wav")) != "RIFF-kick" {
		t.Error("the file isn't under its new name")
	}
	// Another case of the same name: the same file.
	if got, err := a.RenameFile(root, "Samples/Kick 1.wav", "KICK 1.wav"); err != nil || got != "Samples/KICK 1.wav" {
		t.Fatalf("case rename: %q, %v", got, err)
	}
	// Folders too.
	if got, err := a.RenameFile(root, "Samples", "Sounds"); err != nil || got != "Sounds" {
		t.Fatalf("folder: %q, %v", got, err)
	}

	for _, c := range []struct{ rel, name, want string }{
		{"Sounds/snare.wav", "KICK 1.wav", "already"},         // never over another file
		{"Sounds/snare.wav", "kick 1.wav", "already"},         // nor another case of it
		{"Sounds/snare.wav", "../snare.wav", "/"},             // stays in its folder
		{"Sounds/snare.wav", `x\snare.wav`, "/"},              // either separator
		{"Sounds/snare.wav", "a:b.wav", "can't have"},         // Windows can't
		{"Sounds/snare.wav", "snare.", "end with"},            // nor this
		{"Sounds/snare.wav", "CON.wav", "Windows keeps"},      // nor this
		{"Sounds/snare.wav", ".r3v", "R3V's own"},             // R3V's folder
		{"Sounds/snare.wav", "  ", "type a name"},             // a name
		{".r3v/config.json", "x.json", "invalid"},             // R3V's own files
		{"../outside.wav", "x.wav", "invalid"},                // only the project's
		{filepath.Join(root, "Song.als"), "x.als", "invalid"}, // relative paths only
		{"Sounds/gone.wav", "x.wav", "isn't there"},
	} {
		_, err := a.RenameFile(root, c.rel, c.name)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("rename %s to %q: %v, want an error with %q", c.rel, c.name, err, c.want)
		}
	}
	if read(t, filepath.Join(root, "Sounds", "snare.wav")) != "RIFF-snare" || read(t, filepath.Join(root, "Sounds", "KICK 1.wav")) != "RIFF-kick" {
		t.Error("a refused rename changed a file")
	}
	if _, err := a.RenameFile(t.TempDir(), "Song.als", "x.als"); err == nil {
		t.Error("renamed in a folder the app doesn't list")
	}
}

func TestRenameWaitsForLive(t *testing.T) {
	root := localSong(t)
	a := NewApp()
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hi"), 0o644)

	liveHas(t, "Song.als")
	for _, rel := range []string{"Song.als", "Samples/kick.wav", "Samples"} {
		if _, err := a.RenameFile(root, rel, "Other"); err == nil || !strings.Contains(err.Error(), "open in Live") {
			t.Errorf("%s renamed under Live: %v", rel, err)
		}
	}
	// Live running, the set it has open not told: still not guessed.
	liveHas(t, "?")
	if _, err := a.RenameFile(root, "Song.als", "Other.als"); err == nil || !strings.Contains(err.Error(), "Live is running") {
		t.Errorf("renamed with Live running: %v", err)
	}
	// A file Live doesn't use is fine.
	if _, err := a.RenameFile(root, "notes.txt", "notes 2.txt"); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Song.als")); err != nil {
		t.Error("the set moved")
	}
}

func TestRenameTakesUnityMeta(t *testing.T) {
	root := localSong(t)
	liveHas(t, "")
	a := NewApp()
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(root, "notes.txt.meta"), []byte("guid: 1"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(root, "c.txt.meta"), []byte("guid: 2"), 0o644)

	if _, err := a.RenameFile(root, "notes.txt", "readme.txt"); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(root, "readme.txt.meta")) != "guid: 1" {
		t.Error("the .meta stayed behind")
	}
	// Its .meta would land on another: nothing is renamed.
	os.WriteFile(filepath.Join(root, "b.txt.meta"), []byte("guid: 3"), 0o644)
	if _, err := a.RenameFile(root, "b.txt", "c.txt"); err == nil {
		t.Error("renamed over another .meta")
	}
	if read(t, filepath.Join(root, "b.txt")) != "b" || read(t, filepath.Join(root, "c.txt.meta")) != "guid: 2" {
		t.Error("a refused rename changed files")
	}
}

func TestCopyIntoProject(t *testing.T) {
	root := localSong(t)
	a := NewApp()
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "pad.wav"), []byte("RIFF-pad"), 0o644)
	os.MkdirAll(filepath.Join(src, "Loops", "More"), 0o755)
	os.WriteFile(filepath.Join(src, "Loops", "a.wav"), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(src, "Loops", "More", "b.wav"), []byte("B"), 0o644)
	os.WriteFile(filepath.Join(src, "kick.wav"), []byte("other kick"), 0o644)

	res, err := a.CopyIntoProject(root, "Samples", []string{filepath.Join(src, "pad.wav"), filepath.Join(src, "Loops")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Copied, ",") != "Samples/pad.wav,Samples/Loops" || len(res.Clashes) != 0 {
		t.Fatalf("copied %v, clashes %v", res.Copied, res.Clashes)
	}
	if read(t, filepath.Join(root, "Samples", "Loops", "More", "b.wav")) != "B" || read(t, filepath.Join(root, "Samples", "pad.wav")) != "RIFF-pad" {
		t.Error("not copied whole")
	}
	if read(t, filepath.Join(src, "pad.wav")) != "RIFF-pad" {
		t.Error("the original changed")
	}

	// A name already there: nothing is copied, the clash is named.
	os.WriteFile(filepath.Join(src, "new.wav"), []byte("N"), 0o644)
	res, err = a.CopyIntoProject(root, "Samples", []string{filepath.Join(src, "new.wav"), filepath.Join(src, "kick.wav")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Clashes, ",") != "kick.wav" || len(res.Copied) != 0 {
		t.Fatalf("clashes %v, copied %v", res.Clashes, res.Copied)
	}
	if read(t, filepath.Join(root, "Samples", "kick.wav")) != "RIFF-kick" {
		t.Error("overwrote a file")
	}
	if _, err := os.Stat(filepath.Join(root, "Samples", "new.wav")); err == nil {
		t.Error("copied some of the files")
	}

	// Into the top of the project; nothing left aside.
	if res, err := a.CopyIntoProject(root, "", []string{filepath.Join(src, "new.wav")}); err != nil || res.Copied[0] != "new.wav" {
		t.Fatalf("%v, %v", res, err)
	}
	if left, _ := filepath.Glob(filepath.Join(root, ".r3v", "objects", "tmp", "drop-*")); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}

	// Not into R3V's folder, out of the project, or a folder into itself.
	for _, c := range []struct {
		dir string
		src []string
	}{
		{".r3v", []string{filepath.Join(src, "pad.wav")}},
		{"../x", []string{filepath.Join(src, "pad.wav")}},
		{"Gone", []string{filepath.Join(src, "pad.wav")}},
		{"Samples/Loops", []string{filepath.Join(root, "Samples")}},
		{"", []string{filepath.Join(root, ".r3v", "config.json")}},
		{"", []string{"relative.wav"}},
	} {
		if _, err := a.CopyIntoProject(root, c.dir, c.src); err == nil {
			t.Errorf("copied %v into %q", c.src, c.dir)
		}
	}
}

// Each file's last change: the newest version that changed it.
func TestLastChanges(t *testing.T) {
	root := localSong(t)
	t.Cleanup(waitTidy)
	a := NewApp()
	if _, err := a.Save(root, "first", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "Samples", "kick.wav"), []byte("RIFF-kick-2"), 0o644)
	if _, err := a.Save(root, "second", true, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	last, err := a.LastChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	if last["Song.als"].Message != "first" || last["Samples/kick.wav"].Message != "second" {
		t.Errorf("last changes: %+v", last)
	}
}
