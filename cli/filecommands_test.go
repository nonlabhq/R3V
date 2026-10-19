package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/handlers"
	"github.com/nonlabhq/r3v/internal/profile"
)

// r3v info: each kind of file the way it tells what it is, and in a
// project whether R3V keeps it.
func TestFileInfo(t *testing.T) {
	wav, _ := filepath.Abs(filepath.Join("..", "testdata", "live", "Samples", "Processed", "Bounce", "Bounce 2-Analog [2026-09-28 195449]-2.wav"))
	set, _ := filepath.Abs(filepath.Join("..", "testdata", "live", "SampleAbletonProject.als"))
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\nrules:\n  - ignore: \"cache/\"\n",
		"notes.md": "# Plan\r\nkick\r\n", "cover.png": pngOf(t, 5, 3), "cache/x.bin": "\x00\x01"})
	t.Chdir(a)
	if code := Run([]string{"init"}); code != 0 { // (init answers in text)
		t.Fatalf("init: exit %d", code)
	}
	info := func(p string) fileInfoJSON {
		t.Helper()
		exit, rep := run(t, a, "info", p)
		if exit != 0 {
			t.Fatalf("info %s: %+v", p, rep.Error)
		}
		var in fileInfoJSON
		json.Unmarshal(rep.Result, &in)
		return in
	}
	if in := info("notes.md"); in.Kind != "text" || in.Text.Lines != 2 || in.Text.LineEnding != "crlf" || in.Tracked == nil || !*in.Tracked {
		t.Errorf("text: %+v", in)
	}
	if in := info("cover.png"); in.Kind != "image" || in.Picture.Width != 5 || in.Picture.Height != 3 {
		t.Errorf("picture: %+v", in)
	}
	if in := info("cache/x.bin"); in.Kind != "binary" || in.Size != 2 || in.Tracked == nil || *in.Tracked || in.By == "" {
		t.Errorf("left out: %+v", in)
	}
	if in := info(wav); in.Kind != "audio" || in.Audio.Rate == 0 || in.Tracked != nil {
		t.Errorf("audio (not in the project): %+v", in)
	}
	if in := info(set); in.Kind != "set" || in.Set.Tracks == 0 {
		t.Errorf("set: %+v", in)
	}
	if code := Run([]string{"info", set}); code != 0 { // (a set in text: its tracks, as before)
		t.Errorf("set in text: exit %d", code)
	}
}

// Outside a project merge-files goes by the presets' handlers for the
// name: these stand for the code preset's text and Unity's scenes.
func init() {
	if err := profile.RegisterPreset([]byte("name: zz-cli-merge-files\npriority: -9\nhandlers:\n" +
		"  - files: [\"*.txt\"]\n    merge: text\n  - files: [\"*.zzscene\"]\n    merge: zz-scene\n")); err != nil {
		panic(err)
	}
	// zz-scene: clean unless a side says "clash"; merged, both sides.
	handlers.RegisterMerge("zz-scene", func(base, ours, theirs []byte) ([]byte, bool, error) {
		if bytes.Contains(ours, []byte("clash")) || bytes.Contains(theirs, []byte("clash")) {
			return nil, false, nil
		}
		return append(append([]byte{}, ours...), theirs...), true, nil
	})
}

// r3v merge-files: text merged place by place, any other file whole; what
// both sides changed differently stops it unless --strategy says.
func TestMergeFiles(t *testing.T) {
	dir := t.TempDir()
	put := func(name, text string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	base := put("base.txt", "a\nb\nc\n")
	ours := put("ours.txt", "A\nb\nc\n")
	theirs := put("theirs.txt", "a\nb\nC\n")
	out := filepath.Join(dir, "out.txt")
	if exit, rep := run(t, dir, "merge-files", base, ours, theirs, "-o", out); exit != 0 {
		t.Fatalf("merge: %+v", rep.Error)
	}
	if data, _ := os.ReadFile(out); string(data) != "A\nb\nC\n" {
		t.Errorf("merged: %q", data)
	}
	clash := put("clash.txt", "a\nB\nc\n")
	other := put("other.txt", "a\nX\nc\n")
	exit, rep := run(t, dir, "merge-files", base, clash, other, "-o", out)
	if exit != exitConflict || rep.Error.Code != "merge_conflict" {
		t.Fatalf("a clash: %d %+v", exit, rep.Error)
	}
	if exit, _ := run(t, dir, "merge-files", base, clash, other, "-o", out, "--strategy", "both"); exit != 0 {
		t.Fatal("--strategy both")
	}
	if data, _ := os.ReadFile(out); string(data) != "a\nB\nX\nc\n" {
		t.Errorf("both: %q", data)
	}
	bb, bo, bt := put("b.bin", "\x00base"), put("o.bin", "\x00base"), put("t.bin", "\x00new")
	if exit, rep := run(t, dir, "merge-files", bb, bo, bt, "-o", out); exit != 0 {
		t.Fatalf("binary, one side changed: %+v", rep.Error)
	}
	if data, _ := os.ReadFile(out); string(data) != "\x00new" {
		t.Errorf("binary: %q", data)
	}
	bo2 := put("o2.bin", "\x00mine")
	if exit, _ := run(t, dir, "merge-files", bb, bo2, bt, "-o", out); exit != exitConflict {
		t.Error("binary changed on both sides: merged")
	}
	if exit, _ := run(t, dir, "merge-files", bb, bo2, bt, "-o", out, "--strategy", "theirs"); exit != 0 {
		t.Error("binary --strategy theirs")
	}
}

// merge-files chooses the merge a branch merge would: a kind merged whole
// (an ASCII .fbx, a scene its handler couldn't merge) gives a whole-file
// result, a conflict unless one side is unchanged; --as text overrides it.
// JSON says which handler it used.
func TestMergeFilesFollowsHandlers(t *testing.T) {
	dir := t.TempDir()
	put := func(name, text string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	merge := func(want int, args ...string) fileMergeJSON {
		t.Helper()
		exit, rep := run(t, dir, append([]string{"merge-files"}, args...)...)
		if exit != want {
			t.Fatalf("merge-files %v: exit %d, want %d: %+v", args, exit, want, rep.Error)
		}
		var res fileMergeJSON
		json.Unmarshal(rep.Result, &res)
		return res
	}
	// An ASCII .fbx: text, but no handler takes it.
	base, ours, theirs := put("b.fbx", "a\nb\nc\n"), put("o.fbx", "A\nb\nc\n"), put("t.fbx", "a\nb\nC\n")
	out := filepath.Join(dir, "Hero.fbx")
	exit, rep := run(t, dir, "merge-files", base, ours, theirs, "-o", out)
	if exit != exitConflict || rep.Error.Code != "merge_conflict" {
		t.Fatalf("fbx changed on both sides: %d %+v", exit, rep.Error)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("a conflict wrote the file")
	}
	if res := merge(0, base, base, theirs, "-o", out); res.Handler != "whole" || res.Kind != "text" || res.Conflicts != 0 {
		t.Errorf("fbx, one side changed: %+v", res)
	}
	if data, _ := os.ReadFile(out); string(data) != "a\nb\nC\n" {
		t.Errorf("fbx, theirs: %q", data)
	}
	if res := merge(0, base, ours, theirs, "-o", out, "--strategy", "theirs"); res.Handler != "whole" || res.Conflicts != 1 || res.Resolved != "theirs" {
		t.Errorf("fbx --strategy theirs: %+v", res)
	}
	merge(exitUsage, base, ours, theirs, "-o", out, "--strategy", "both")
	if res := merge(0, base, ours, theirs, "-o", out, "--as", "text"); res.Handler != "text" || res.Conflicts != 0 {
		t.Errorf("--as text: %+v", res)
	}
	if data, _ := os.ReadFile(out); string(data) != "A\nb\nC\n" {
		t.Errorf("--as text: %q", data)
	}
	merge(exitUsage, base, ours, theirs, "-o", out, "--as", "lines")

	// Text by its handler, and --as whole; --as text of files that aren't.
	tb, to, tt := put("b.txt", "a\nb\nc\n"), put("o.txt", "A\nb\nc\n"), put("t.txt", "a\nb\nC\n")
	if res := merge(0, tb, to, tt, "-o", filepath.Join(dir, "notes.txt")); res.Handler != "text" {
		t.Errorf("txt: %+v", res)
	}
	merge(exitConflict, tb, to, tt, "-o", filepath.Join(dir, "notes.txt"), "--as", "whole")
	bb, bo := put("b.bin", "\x00a"), put("o.bin", "\x00b")
	merge(exitUsage, bb, bo, bb, "-o", filepath.Join(dir, "x.txt"), "--as", "text")
	// A name no preset knows: by its content, as git's mergetool always did.
	if res := merge(0, tb, to, tt, "-o", filepath.Join(dir, "notes.foo")); res.Handler != "text" || res.Conflicts != 0 {
		t.Errorf("unknown text: %+v", res)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "notes.foo")); string(data) != "A\nb\nC\n" {
		t.Errorf("unknown text: %q", data)
	}
	if res := merge(0, bb, bb, bo, "-o", filepath.Join(dir, "x.foo")); res.Handler != "whole" || res.Kind != "binary" {
		t.Errorf("unknown binary: %+v", res)
	}

	// A preset's own handler: merged when clean, else the whole file.
	sb, so, st := put("b.zzscene", "base"), put("o.zzscene", "ours"), put("t.zzscene", "theirs")
	scene := filepath.Join(dir, "Main.zzscene")
	if res := merge(0, sb, so, st, "-o", scene); res.Handler != "zz-scene" || res.Conflicts != 0 {
		t.Errorf("scene: %+v", res)
	}
	if data, _ := os.ReadFile(scene); string(data) != "ourstheirs" {
		t.Errorf("scene: %q", data)
	}
	sc := put("c.zzscene", "clash")
	exit, rep = run(t, dir, "merge-files", sb, sc, st, "-o", scene)
	if exit != exitConflict || rep.Error.Code != "merge_conflict" {
		t.Fatalf("scene clash: %d %+v", exit, rep.Error)
	}
	if res := merge(0, sb, sc, st, "-o", scene, "--strategy", "ours"); res.Handler != "zz-scene" || res.Conflicts != 1 {
		t.Errorf("scene --strategy ours: %+v", res)
	}
	if data, _ := os.ReadFile(scene); string(data) != "clash" {
		t.Errorf("scene, ours: %q", data)
	}
}

// In a project merge-files takes the project's rules for -o's path: one
// whose presets merge no .txt merges it whole, as its branch merges do
// (outside, by the presets, or a name they don't know by its content).
func TestMergeFilesInProject(t *testing.T) {
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n", "notes.txt": "a\nb\nc\n"})
	t.Chdir(a)
	if code := Run([]string{"init"}); code != 0 {
		t.Fatalf("init: exit %d", code)
	}
	tmp := t.TempDir()
	put := func(name, text string) string {
		p := filepath.Join(tmp, name)
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	base, ours, theirs := put("b.txt", "a\nb\nc\n"), put("o.txt", "A\nb\nc\n"), put("t.txt", "a\nb\nC\n")
	exit, rep := run(t, a, "merge-files", base, ours, theirs, "-o", filepath.Join(a, "notes.txt"))
	if exit != exitConflict || rep.Error.Code != "merge_conflict" {
		t.Fatalf("in the project: %d %+v", exit, rep.Error)
	}
	if exit, rep := run(t, a, "merge-files", base, ours, theirs, "-o", filepath.Join(a, "notes.foo")); exit != exitConflict {
		t.Fatalf("a name no preset knows, in the project: %d %+v", exit, rep.Error)
	}
	// The same files outside it: the presets' text handler.
	exit, rep = run(t, tmp, "merge-files", base, ours, theirs, "-o", filepath.Join(tmp, "notes.txt"))
	var res fileMergeJSON
	json.Unmarshal(rep.Result, &res)
	if exit != 0 || res.Handler != "text" {
		t.Fatalf("outside: %d %+v %+v", exit, res, rep.Error)
	}
}
