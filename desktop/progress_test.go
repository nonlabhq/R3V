package desktop

import (
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
)

// The last progress of a stage, held back by the 150 ms pace, still goes
// out (the bar reaches its 100%); none goes after "done".
func TestProgressKeepsTheLast(t *testing.T) {
	a := NewApp()
	var mu sync.Mutex
	var got []ProgressEvent
	a.emit = func(name string, data any) {
		if name == "progress" {
			mu.Lock()
			got = append(got, data.(ProgressEvent))
			mu.Unlock()
		}
	}
	report, done := a.progressFor("C:/Song")
	report(project.Progress{Stage: project.StageUploading, Done: 1, Total: 3})
	report(project.Progress{Stage: project.StageUploading, Done: 2, Total: 3})
	report(project.Progress{Stage: project.StageUploading, Done: 3, Total: 3})
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	if len(got) != 2 || got[1].Done != 3 {
		t.Fatalf("events: %+v", got)
	}
	mu.Unlock()
	report(project.Progress{Stage: project.StageUploading, Done: 1, Total: 3}) // held back…
	done()                                                                     // …and dropped
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if last := got[len(got)-1]; last.Stage != "done" {
		t.Errorf("after done: %+v", got)
	}
}
