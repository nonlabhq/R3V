package desktop

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Calls that take slowCall or more log how long each step took (in the
// app's log, r3v.log); with R3V_TRACE set, every call does.
var tracing = os.Getenv("R3V_TRACE") != ""

const slowCall = 2 * time.Second

type stopwatch struct {
	name  string
	start time.Time
	last  time.Time
	laps  []string
}

func startWatch(name string) *stopwatch {
	now := time.Now()
	return &stopwatch{name: name, start: now, last: now}
}

// lap records the time since the previous lap under a step name.
func (s *stopwatch) lap(step string) {
	now := time.Now()
	s.laps = append(s.laps, fmt.Sprintf("%s %s", step, now.Sub(s.last).Round(time.Millisecond)))
	s.last = now
}

func (s *stopwatch) done() {
	if tracing || time.Since(s.start) >= slowCall {
		log.Printf("trace %s: %s total — %s", s.name, time.Since(s.start).Round(time.Millisecond), strings.Join(s.laps, ", "))
	}
}
