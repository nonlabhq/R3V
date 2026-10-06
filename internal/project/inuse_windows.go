package project

import (
	"errors"
	"syscall"
)

// inUse: another program holds the file so it can't be read now (Windows:
// a sharing or lock violation, e.g. Live writing a Freeze file).
func inUse(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == 32 || errno == 33) // ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION
}
