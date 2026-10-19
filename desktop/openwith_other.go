//go:build !windows

package desktop

import (
	"os"
	"os/exec"
)

// systemLookup has no registry here: only what PATH has (the editors'
// Windows names are rarely on it, so mostly nothing).
func systemLookup() lookup {
	return lookup{
		glob:   func(string) []string { return nil },
		exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
		getenv: os.Getenv,
		lookPath: func(f string) string {
			p, _ := exec.LookPath(f)
			return p
		},
	}
}

func ownConsole(*exec.Cmd, bool) {}
