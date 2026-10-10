package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A copy is a new file: the content, written now, writable, whatever the
// source's time and read-only mark; nothing left beside it.
func TestCopyAtomic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	os.WriteFile(src, []byte("content"), 0o644)
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(src, old, old)
	os.Chmod(src, 0o444)
	defer os.Chmod(src, 0o644)
	dst := filepath.Join(dir, "out", "dst.bin")
	os.MkdirAll(filepath.Dir(dst), 0o755)
	os.WriteFile(dst, []byte("old"), 0o644)
	if err := CopyAtomic(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "content" {
		t.Fatalf("copied %q", got)
	}
	fi, err := os.Stat(dst)
	if err != nil || time.Since(fi.ModTime()) > time.Hour || fi.Mode().Perm()&0o200 == 0 {
		t.Fatalf("copy: %v %v %v", err, fi.ModTime(), fi.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(dst)); len(entries) != 1 {
		t.Errorf("left beside it: %v", entries)
	}
}
