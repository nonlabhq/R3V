package flock

import (
	"errors"
	"syscall"
)

const (
	errorSharingViolation = syscall.Errno(32)  // ERROR_SHARING_VIOLATION
	errorWriteProtect     = syscall.Errno(19)  // ERROR_WRITE_PROTECT
	errorDirectory        = syscall.Errno(267) // ERROR_DIRECTORY
)

// deniedErrnos say R3V may not write a path (fs.ErrPermission aside).
var deniedErrnos = []error{errorWriteProtect, errorDirectory, syscall.ENOTDIR}

// lockFile opens path shared with no one: another program can't open it
// until it is closed, which Windows does when this program ends. Someone
// holding it is a sharing violation; access denied is not someone (a
// folder R3V may not write, a read-only file, a directory there), except
// for a moment while the file is being deleted.
func lockFile(path string) (func(), error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, errorSharingViolation) {
			return nil, errLocked
		}
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return nil, &deniedError{err}
		}
		return nil, err
	}
	return func() { syscall.CloseHandle(h) }, nil
}
