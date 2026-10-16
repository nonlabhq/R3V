//go:build !windows

package project

import "syscall"

// processRunning: a process with this id is there (one we may not signal
// is, too).
func processRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
