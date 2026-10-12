//go:build nightly

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/project"
)

// The agent tool's hooks make a turn per prompt and a step after a tool
// call that changed files; r3v steps lists them, step back goes back.
func TestStepsFromHooks(t *testing.T) {
	a := newFolder(t, map[string]string{"notes.txt": "one"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	hook := func(kind string, in map[string]string) string {
		in["cwd"] = a
		data, _ := json.Marshal(in)
		var out bytes.Buffer
		hookSteps(kind, bytes.NewReader(data), &out)
		return out.String()
	}
	hook("prompt", map[string]string{"session_id": "s1", "prompt": "Warmer, please", "transcript_path": "C:/t/s1.jsonl"})
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("two"), 0o644)
	hook("post-tool", map[string]string{"session_id": "s1", "tool_name": "Write"})
	hook("post-tool", map[string]string{"session_id": "s1", "tool_name": "Read"}) // reads: no scan
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("three"), 0o644)
	hook("post-tool", map[string]string{"session_id": "s1", "tool_name": "mcp__blender__execute_code"})

	exit, rep := run(t, a, "steps")
	var got struct{ Session project.Session }
	json.Unmarshal(rep.Result, &got)
	s := got.Session
	if exit != 0 || s.ID != "s1" || s.Steps != 2 || len(s.Turns) != 1 || s.Turns[0].Prompt != "Warmer, please" ||
		s.Turns[0].Steps[1].Tool != "mcp__blender__execute_code" {
		t.Fatalf("steps: %s %+v", rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "step", "back", "1")
	if data, _ := os.ReadFile(filepath.Join(a, "notes.txt")); exit != 0 || string(data) != "two" {
		t.Fatalf("back: %q %s %+v", data, rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "step", "diff", "1", "2")
	if exit != 0 || !strings.Contains(string(rep.Result), `"modified"`) {
		t.Fatalf("diff: %s", rep.Result)
	}
	exit, rep = run(t, a, "step", "-m", "before trying the fog")
	if exit != 0 || !strings.Contains(string(rep.Result), `"n": 1`) {
		t.Fatalf("by hand: %s %+v", rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "step")
	if exit != 0 || !strings.Contains(string(rep.Result), `"step": null`) {
		t.Fatalf("by hand, nothing changed: %s", rep.Result)
	}
	exit, rep = run(t, a, "steps", "--list")
	if exit != 0 || !strings.Contains(string(rep.Result), `"by-hand"`) {
		t.Fatalf("sessions: %s", rep.Result)
	}
	// Steps never go to the team, nor into the history.
	exit, rep = run(t, a, "log")
	if n := strings.Count(string(rep.Result), `"id"`); exit != 0 || n != 1 {
		t.Fatalf("log: %d versions: %s", n, rep.Result)
	}
}
