//go:build !windows

package desktop

import (
	"errors"
	"io/fs"
	"os"
)

// renameNoReplace moves src to dst, failing when dst exists.
func renameNoReplace(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(src, dst)
}

func inUseErr(error) bool { return false }
