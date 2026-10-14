//go:build nightly

package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
)

func pngOf(t *testing.T, w, h int) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// r3v diff: each kind of file the way it reads, your changes by default,
// a version's or two versions' on asking; paths narrow it.
func TestFileDiff(t *testing.T) {
	a := newFolder(t, map[string]string{".r3v.yaml": "presets:\n  ./: none\n",
		"notes.md": "# Plan\nkick\nsnare\n", "cover.png": pngOf(t, 4, 4), "data.bin": "\x00\x01\x02"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	if exit, rep := run(t, a, "commit", "--local", "-m", "first"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	os.WriteFile(filepath.Join(a, "notes.md"), []byte("# Plan\nkick\nhats\nsnare\n"), 0o644)
	os.WriteFile(filepath.Join(a, "cover.png"), []byte(pngOf(t, 8, 6)), 0o644)
	os.WriteFile(filepath.Join(a, "data.bin"), []byte("\x00\x01\x02\x03\x04"), 0o644)
	os.WriteFile(filepath.Join(a, "new.txt"), []byte("hello\n"), 0o644)

	diffOf := func(args ...string) map[string]fileDiffJSON {
		t.Helper()
		exit, rep := run(t, a, append([]string{"diff"}, args...)...)
		if exit != 0 {
			t.Fatalf("diff %v: %+v", args, rep.Error)
		}
		var got struct{ Files []fileDiffJSON }
		json.Unmarshal(rep.Result, &got)
		out := map[string]fileDiffJSON{}
		for _, f := range got.Files {
			out[f.Path] = f
		}
		return out
	}
	got := diffOf()
	if n := got["notes.md"]; n.Kind != "text" || n.Added != 1 || n.Removed != 0 || len(n.Hunks) != 1 {
		t.Errorf("text: %+v", n)
	}
	if p := got["cover.png"]; p.Kind != "image" || p.Picture == nil || p.Picture.OldWidth != 4 || p.Picture.NewWidth != 8 || p.Picture.NewHeight != 6 {
		t.Errorf("picture: %+v", p)
	}
	if b := got["data.bin"]; b.Kind != "binary" || b.SizeOld != 3 || b.SizeNew != 5 {
		t.Errorf("binary: %+v", b)
	}
	if n := got["new.txt"]; n.Status != "added" || n.Kind != "text" || n.Added != 1 {
		t.Errorf("added: %+v", n)
	}
	if only := diffOf("notes.md"); len(only) != 1 {
		t.Errorf("one path: %+v", only)
	}

	if exit, rep := run(t, a, "commit", "--local", "-m", "second"); exit != 0 {
		t.Fatalf("commit: %+v", rep.Error)
	}
	if left := diffOf(); len(left) != 0 {
		t.Errorf("nothing changed since: %+v", left)
	}
	if v := diffOf("--version", "HEAD"); len(v) != 4 || v["notes.md"].Added != 1 {
		t.Errorf("the version's changes: %+v", v)
	}
	if v := diffOf("--from", "HEAD~1", "--to", "HEAD", "notes.md"); len(v) != 1 || v["notes.md"].Removed != 0 {
		t.Errorf("between two versions: %+v", v)
	}
	if exit, _ := run(t, a, "diff", "--from", "HEAD~1"); exit == 0 {
		t.Error("--from without --to: taken")
	}
}

// Two Live Sets on disk compare as before.
func TestFileDiffTwoSets(t *testing.T) {
	dir := filepath.Join("..", "testdata", "live")
	a, _ := filepath.Abs(filepath.Join(dir, "SampleAbletonProject.als"))
	b, _ := filepath.Abs(filepath.Join(dir, "SampleAbletonProject_v2.als"))
	if code := Run([]string{"diff", a, b}); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

// Two files on disk, any kind: text line by line, two sets track by track.
func TestFileDiffDiskFiles(t *testing.T) {
	live := filepath.Join("..", "testdata", "live")
	sa, _ := filepath.Abs(filepath.Join(live, "SampleAbletonProject.als"))
	sb, _ := filepath.Abs(filepath.Join(live, "SampleAbletonProject_v2.als"))
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("kick\nsnare\n"), 0o644)
	os.WriteFile(b, []byte("kick\nhats\n"), 0o644)
	exit, rep := run(t, dir, "diff", "--files", a, b)
	if exit != 0 {
		t.Fatalf("diff --files: %+v", rep.Error)
	}
	var got struct{ Files []fileDiffJSON }
	json.Unmarshal(rep.Result, &got)
	if len(got.Files) != 1 || got.Files[0].Kind != "text" || got.Files[0].Added != 1 || got.Files[0].Removed != 1 {
		t.Fatalf("text: %+v", got.Files)
	}
	exit, rep = run(t, dir, "diff", "--files", sa, sb)
	json.Unmarshal(rep.Result, &got)
	if exit != 0 || len(got.Files) != 1 || got.Files[0].Kind != "set" || len(got.Files[0].SetChanges) == 0 {
		t.Fatalf("sets: %+v %+v", got.Files, rep.Error)
	}
	if exit, _ := run(t, dir, "diff", "--files", a); exit == 0 {
		t.Error("one file: taken")
	}
}
