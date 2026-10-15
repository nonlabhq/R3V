package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
