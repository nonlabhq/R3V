//go:build windows && !server

package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// Dragging files out of the app (into Explorer, a DAW, Photoshop): a web
// view's own drag can't carry real files, so the page asks for a native
// one while the mouse button is still down. The shell makes the data object
// (CF_HDROP and the shell's formats, which every program takes) and the app
// runs OLE's drag loop on the window's thread. Only copying (or a link) is
// offered: a project file is never moved out by a drag.

var (
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procOleInitialize      = ole32.NewProc("OleInitialize")
	procDoDragDrop         = ole32.NewProc("DoDragDrop")
	procCoTaskMemFree      = ole32.NewProc("CoTaskMemFree")
	procSHParseDisplayName = shell32.NewProc("SHParseDisplayName")
	procSHCreateDataObject = shell32.NewProc("SHCreateDataObject")
	procILFindLastID       = shell32.NewProc("ILFindLastID")
)

const (
	dropEffectCopy = 1
	dropEffectMove = 2
	dropEffectLink = 4

	sOK                     = 0
	eNoInterface            = 0x80004002
	dragdropSDrop           = 0x00040100
	dragdropSCancel         = 0x00040101
	dragdropSUseDefCursors  = 0x00040102
	mkLButton               = 0x0001
	mkRButton               = 0x0002
	cfHDROP                 = 15
	dvaspectContent         = 1
	tymedHGlobal            = 1
	idataObjectGetData      = 3
	idataObjectQueryGetData = 5
	iunknownRelease         = 2
)

var (
	iidIUnknown    = windows.GUID{Data1: 0x00000000, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDropSource = windows.GUID{Data1: 0x00000121, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDataObject = windows.GUID{Data1: 0x0000010e, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// dropSource is the IDropSource the drag loop asks whether to go on. One
// drag at a time: a single object, never freed.
type dropSource struct {
	vtbl *dropSourceVtbl
	refs int32
}

type dropSourceVtbl struct {
	QueryInterface, AddRef, Release, QueryContinueDrag, GiveFeedback uintptr
}

var (
	theSource     dropSource
	theSourceVtbl dropSourceVtbl
	sourceOnce    sync.Once
	dragging      atomic.Bool
)

func initDropSource() {
	theSourceVtbl = dropSourceVtbl{
		QueryInterface: syscall.NewCallback(func(this uintptr, riid *windows.GUID, ppv *uintptr) uintptr {
			if *riid == iidIUnknown || *riid == iidIDropSource {
				*ppv = this
				return sOK
			}
			*ppv = 0
			return eNoInterface
		}),
		AddRef:  syscall.NewCallback(func(uintptr) uintptr { return uintptr(atomic.AddInt32(&theSource.refs, 1)) }),
		Release: syscall.NewCallback(func(uintptr) uintptr { return uintptr(atomic.AddInt32(&theSource.refs, -1)) }),
		QueryContinueDrag: syscall.NewCallback(func(_, escape, keys uintptr) uintptr {
			switch {
			case uint32(escape) != 0 || keys&mkRButton != 0:
				return dragdropSCancel
			case keys&mkLButton == 0:
				return dragdropSDrop
			}
			return sOK
		}),
		GiveFeedback: syscall.NewCallback(func(_, _ uintptr) uintptr { return dragdropSUseDefCursors }),
	}
	theSource.vtbl = &theSourceVtbl
}

// StartDrag drags files of the project (relative paths, all in one folder)
// out of the app, while the mouse button is held; it returns when they are
// dropped (true) or the drag is called off (false).
func (a *App) StartDrag(root string, files []string) (bool, error) {
	if !knownProject(root) {
		return false, errors.New("unknown project")
	}
	paths, err := dragPaths(root, files)
	if err != nil {
		return false, err
	}
	if !dragging.CompareAndSwap(false, true) {
		return false, nil
	}
	defer dragging.Store(false)
	return application.InvokeSyncWithResultAndError(func() (bool, error) { return dragFiles(paths) })
}

// dragPaths: the files' absolute paths, all there and in one folder (the
// shell's data object holds items of one folder).
func dragPaths(root string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, errors.New("nothing to drag")
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if !insideProject(f) {
			return nil, errors.New("invalid path")
		}
		p := filepath.Join(root, filepath.FromSlash(f))
		if _, err := os.Lstat(p); err != nil {
			return nil, fmt.Errorf("%s isn't there", filepath.Base(p))
		}
		if len(out) > 0 && !sameDir(out[0], p) {
			return nil, errors.New("drag files of one folder at a time")
		}
		out = append(out, p)
	}
	return out, nil
}

func sameDir(a, b string) bool {
	return filepath.Clean(filepath.Dir(a)) == filepath.Clean(filepath.Dir(b))
}

// dragFiles runs the drag loop on this (the window's) thread.
func dragFiles(paths []string) (bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procOleInitialize.Call(0) // (once is enough; S_FALSE when it was)
	data, free, err := shellDataObject(paths)
	if err != nil {
		return false, err
	}
	defer free()
	sourceOnce.Do(initDropSource)
	var effect uint32
	hr, _, _ := procDoDragDrop.Call(uintptr(data), uintptr(unsafe.Pointer(&theSource)), dropEffectCopy|dropEffectLink,
		uintptr(unsafe.Pointer(&effect)))
	switch uint32(hr) {
	case dragdropSDrop:
		return effect != 0, nil
	case dragdropSCancel:
		return false, nil
	}
	return false, fmt.Errorf("drag: %w", syscall.Errno(hr))
}

// shellDataObject makes the shell's IDataObject for files of one folder.
func shellDataObject(paths []string) (unsafe.Pointer, func(), error) {
	var pidls []uintptr
	freeAll := func() {
		for _, p := range pidls {
			procCoTaskMemFree.Call(p)
		}
	}
	parse := func(p string) (uintptr, error) {
		s, err := windows.UTF16PtrFromString(p)
		if err != nil {
			return 0, err
		}
		var pidl uintptr
		hr, _, _ := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(s)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
		if hr != 0 {
			return 0, fmt.Errorf("%s: %w", filepath.Base(p), syscall.Errno(hr))
		}
		pidls = append(pidls, pidl)
		return pidl, nil
	}
	folder, err := parse(filepath.Dir(paths[0]))
	if err != nil {
		freeAll()
		return nil, nil, err
	}
	children := make([]uintptr, 0, len(paths))
	for _, p := range paths {
		item, err := parse(p)
		if err != nil {
			freeAll()
			return nil, nil, err
		}
		last, _, _ := procILFindLastID.Call(item)
		children = append(children, last)
	}
	var obj unsafe.Pointer
	hr, _, _ := procSHCreateDataObject.Call(folder, uintptr(len(children)), uintptr(unsafe.Pointer(&children[0])), 0,
		uintptr(unsafe.Pointer(&iidIDataObject)), uintptr(unsafe.Pointer(&obj)))
	runtime.KeepAlive(children)
	if hr != 0 || obj == nil {
		freeAll()
		return nil, nil, fmt.Errorf("drag: %w", syscall.Errno(hr))
	}
	return obj, func() { comCall(obj, iunknownRelease); freeAll() }, nil
}

// comCall calls method n of a COM object's vtable.
func comCall(obj unsafe.Pointer, n int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(n)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return r
}
