//go:build windows && !server

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procDragQueryFileW = shell32.NewProc("DragQueryFileW")
	procReleaseStgMed  = ole32.NewProc("ReleaseStgMedium")
)

type formatEtc struct {
	format uint16
	ptd    uintptr
	aspect uint32
	index  int32
	tymed  uint32
}

type stgMedium struct {
	tymed   uint32
	hGlobal uintptr
	release uintptr
}

// The data object a drag carries holds the files as CF_HDROP, the format
// Explorer, DAWs and Photoshop take.
func TestDragDataObject(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procOleInitialize.Call(0)
	dir := t.TempDir()
	want := []string{filepath.Join(dir, "kick.wav"), filepath.Join(dir, "Loops")}
	os.WriteFile(want[0], []byte("RIFF"), 0o644)
	os.Mkdir(want[1], 0o755)

	obj, free, err := shellDataObject(want)
	if err != nil {
		t.Fatal(err)
	}
	defer free()
	fe := formatEtc{format: cfHDROP, aspect: dvaspectContent, index: -1, tymed: tymedHGlobal}
	if hr := comCall(obj, idataObjectQueryGetData, uintptr(unsafe.Pointer(&fe))); hr != 0 {
		t.Fatalf("no CF_HDROP: %#x", hr)
	}
	var med stgMedium
	if hr := comCall(obj, idataObjectGetData, uintptr(unsafe.Pointer(&fe)), uintptr(unsafe.Pointer(&med))); hr != 0 {
		t.Fatalf("GetData: %#x", hr)
	}
	defer procReleaseStgMed.Call(uintptr(unsafe.Pointer(&med)))
	n, _, _ := procDragQueryFileW.Call(med.hGlobal, 0xFFFFFFFF, 0, 0)
	var got []string
	for i := uintptr(0); i < n; i++ {
		buf := make([]uint16, windows.MAX_LONG_PATH)
		procDragQueryFileW.Call(med.hGlobal, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		got = append(got, windows.UTF16ToString(buf))
	}
	long := func(p string) string { l, _ := filepath.EvalSymlinks(p); return l }
	for i := range got {
		got[i] = long(got[i])
	}
	if !slices.Equal(got, []string{long(want[0]), long(want[1])}) {
		t.Errorf("CF_HDROP has %v, want %v", got, want)
	}
}

// The drag loop asks the drop source whether to go on: drop when the button
// is let go, call it off with Esc or the right button. (The loop itself
// waits for the mouse, so a test can't run it without moving the user's.)
func TestDropSource(t *testing.T) {
	sourceOnce.Do(initDropSource)
	src := unsafe.Pointer(&theSource)
	const queryContinueDrag, giveFeedback = 3, 4
	for _, c := range []struct {
		escape, keys uintptr
		want         uintptr
	}{
		{0, mkLButton, sOK},
		{0, 0, dragdropSDrop},
		{1, mkLButton, dragdropSCancel},
		{0, mkLButton | mkRButton, dragdropSCancel},
	} {
		if got := comCall(src, queryContinueDrag, c.escape, c.keys); got != c.want {
			t.Errorf("QueryContinueDrag(%d, %#x) = %#x, want %#x", c.escape, c.keys, got, c.want)
		}
	}
	if got := comCall(src, giveFeedback, dropEffectCopy); got != dragdropSUseDefCursors {
		t.Errorf("GiveFeedback = %#x", got)
	}
	var out uintptr
	if hr := comCall(src, 0, uintptr(unsafe.Pointer(&iidIDropSource)), uintptr(unsafe.Pointer(&out))); hr != sOK || out != uintptr(src) {
		t.Errorf("QueryInterface(IDropSource) = %#x, %#x", hr, out)
	}
	if hr := comCall(src, 0, uintptr(unsafe.Pointer(&iidIDataObject)), uintptr(unsafe.Pointer(&out))); hr != eNoInterface || out != 0 {
		t.Errorf("QueryInterface(IDataObject) = %#x", hr)
	}
}

func TestDragPaths(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "A"), 0o755)
	os.WriteFile(filepath.Join(root, "A", "x.wav"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "y.wav"), nil, 0o644)
	if _, err := dragPaths(root, []string{"A/x.wav"}); err != nil {
		t.Error(err)
	}
	for _, files := range [][]string{nil, {"A/x.wav", "y.wav"}, {"../y.wav"}, {".r3v/config.json"}, {"gone.wav"}} {
		if _, err := dragPaths(root, files); err == nil {
			t.Errorf("dragged %v", files)
		}
	}
}
