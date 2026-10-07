package teamwatch

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
)

// fakeLive is a notice source: the test nudges and says whether it's
// connected.
type fakeLive struct {
	nudge     chan struct{}
	mu        sync.Mutex
	connected bool
	watched   []string
	stopped   bool
}

func (f *fakeLive) Watch(team, pid string) (<-chan struct{}, func() bool, func()) {
	f.mu.Lock()
	f.watched = append(f.watched, team+" "+pid)
	f.mu.Unlock()
	return f.nudge, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.connected },
		func() { f.mu.Lock(); f.stopped = true; f.mu.Unlock() }
}

// A nudge makes the watch look at once, long before its next poll.
func TestRunLiveLooksWhenNudged(t *testing.T) {
	_, code := storage(t)
	rootB := watchTeam(t, code)
	live := &fakeLive{nudge: make(chan struct{}, 1), connected: true}
	events := make(chan Event, 8)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunLive(ctx, rootB, func() time.Duration { return time.Hour }, live, func(e Event) { events <- e })
		close(done)
	}()

	// Yi commits; the notice comes.
	yi, _, err := project.Clone(code, "Song", filepath.Join(t.TempDir(), "Yi"), "yi")
	if err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(fixtures, "SampleAbletonProject_v3.als"), filepath.Join(yi.Root, "Song.als"))
	time.Sleep(100 * time.Millisecond) // the first round is done
	if _, _, err := yi.Save("a new take", project.Strategy("fail")); err != nil {
		t.Fatal(err)
	}
	live.nudge <- struct{}{}
	for found := false; !found; {
		select {
		case e := <-events:
			// (The first round tells about what was waiting already.)
			found = e.Kind == NewVersions && !e.Waiting && e.Versions[len(e.Versions)-1].Message == "a new take"
		case <-time.After(10 * time.Second):
			t.Fatal("the nudge didn't make the watch look")
		}
	}

	cancel()
	<-done
	r, _ := project.Open(rootB)
	live.mu.Lock()
	defer live.mu.Unlock()
	if len(live.watched) != 1 || live.watched[0] != r.Config.Remote.URL+" "+r.Config.ProjectID {
		t.Errorf("watched %v", live.watched)
	}
	if !live.stopped {
		t.Error("the subscription wasn't stopped")
	}
}
