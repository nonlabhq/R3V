package desktop

import (
	"errors"

	"golang.org/x/sys/windows"
)

// renameNoReplace moves src to dst, failing when dst exists (os.Rename
// would replace a file there).
func renameNoReplace(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, 0)
}

// inUseErr: another program holds the file (or something in the folder).
func inUseErr(err error) bool {
	var errno windows.Errno
	return errors.As(err, &errno) && (errno == windows.ERROR_SHARING_VIOLATION || errno == windows.ERROR_LOCK_VIOLATION ||
		errno == windows.ERROR_ACCESS_DENIED)
}
