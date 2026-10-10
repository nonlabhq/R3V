//go:build !windows

package store

import (
	"io"
	"os"
)

// copyFile copies src into tmp (an empty file, closed here).
func copyFile(src string, tmp *os.File) error {
	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return err
	}
	defer in.Close()
	_, err = io.Copy(tmp, in)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	return err
}
