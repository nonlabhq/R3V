package store

import (
	"os"
	"path/filepath"
	"time"
)

// CopyAtomic writes a copy of the file src to path, as WriteAtomic does. On
// Windows the system makes the copy (CopyFile), which block-clones on
// drives that can (ReFS, Dev Drive): no space is taken until either file
// changes. Never a hard link: tools write files in place, and that would
// change both. The copy is a new file: written now, writable.
func CopyAtomic(src, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".r3v-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := copyFile(src, tmp); err != nil {
		return err
	}
	// (a system copy keeps the source's time and read-only mark: a build
	// tool would take a file put in place now for an old one)
	os.Chmod(name, 0o644)
	now := time.Now()
	if err := os.Chtimes(name, now, now); err != nil {
		return err
	}
	return Replace(name, path)
}
