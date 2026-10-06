package project

import (
	"syscall"
	"testing"
)

// hold opens path with no sharing, as Live does with a Freeze file it is
// writing: no other program can read it until it's closed.
func hold(t *testing.T, path string) func() {
	t.Helper()
	p, _ := syscall.UTF16PtrFromString(path)
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return func() { syscall.CloseHandle(h) }
}

// A file another program holds doesn't stop R3V from reading the project:
// it is skipped (as in the version you are on, or not there yet), listed
// as in use, and read once it is free.
func TestFilesInUse(t *testing.T) {
	dir := newProject(t)
	r, _ := Init(dir, "yi")
	write(t, dir, "notes.txt", "one")
	if _, err := r.Snapshot("v1"); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "notes.txt", "two")
	write(t, dir, "Samples/Processed/Freeze/Freeze 1.wav", "frozen")
	release := hold(t, r.Abs("Samples/Processed/Freeze/Freeze 1.wav"))
	releaseNotes := hold(t, r.Abs("notes.txt"))
	changes, err := r.Status()
	if err != nil {
		t.Fatalf("status with files in use: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("changes while in use: %+v", changes)
	}
	if got := r.InUse(); len(got) != 2 || got[0] != "Samples/Processed/Freeze/Freeze 1.wav" || got[1] != "notes.txt" {
		t.Errorf("in use: %v", got)
	}
	release()
	releaseNotes()
	changes, err = r.Status()
	if err != nil || len(changes) != 2 || len(r.InUse()) != 0 {
		t.Fatalf("once free: %+v %v %v", changes, r.InUse(), err)
	}
}
