package store

import (
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procCopyFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("CopyFileW")

// copyFile copies src over tmp (an empty file, closed here) with CopyFileW;
// byte by byte where the system can't.
func copyFile(src string, tmp *os.File) error {
	tmp.Close()
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(tmp.Name())
	if err != nil {
		return err
	}
	if procCopyFileW.Find() == nil {
		if ok, _, _ := procCopyFileW.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0); ok != 0 {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(tmp.Name())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
