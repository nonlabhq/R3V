package project

import (
	"errors"
	"syscall"
)

// processRunning: a process with this id is there (one denying us a look
// is, too).
func processRunning(pid int) bool {
	const queryLimited = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	const stillActive = 259     // STILL_ACTIVE
	h, err := syscall.OpenProcess(queryLimited, false, uint32(pid))
	if err != nil {
		return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if syscall.GetExitCodeProcess(h, &code) != nil {
		return true
	}
	return code == stillActive
}
