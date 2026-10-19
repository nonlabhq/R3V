package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/agentsetup"
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

// An agent tool without hooks wraps a task in step start and step end:
// a turn with its note, then a step by that tool, listed as the hooks'
// are. Claude Code whose hooks make the steps changes nothing with them.
func TestStepStartEndWithoutHooks(t *testing.T) {
	asUnknown(t)
	t.Setenv("CODEX_THREAD_ID", "th-1")
	a := newFolder(t, map[string]string{"notes.txt": "one"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	if exit, rep := run(t, a, "step", "end"); exit != 0 || !strings.Contains(string(rep.Result), `"ended": null`) ||
		!strings.Contains(string(rep.Result), "no step was started") {
		t.Fatalf("end, none open: %d %s %+v", exit, rep.Result, rep.Error)
	}
	exit, rep := run(t, a, "step", "start", "-m", "Make the notes longer")
	var started struct {
		Open     project.OpenStep
		Before   *project.Step
		Previous *project.StepEnded
	}
	json.Unmarshal(rep.Result, &started)
	if exit != 0 || started.Open.Session != "th-1" || started.Open.Turn != 1 || started.Before != nil || started.Previous != nil {
		t.Fatalf("start: %d %s %+v", exit, rep.Result, rep.Error)
	}
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("one, two"), 0o644)
	exit, rep = run(t, a, "step", "end", "-m", "Longer notes")
	var ended struct {
		Ended *project.StepEnded
		Step  *project.Step
	}
	json.Unmarshal(rep.Result, &ended)
	if exit != 0 || ended.Step == nil || ended.Step.Tool != project.ToolStepEnd || ended.Step.Actor != "codex" ||
		ended.Ended == nil || ended.Ended.Turn != 1 {
		t.Fatalf("end: %d %s %+v", exit, rep.Result, rep.Error)
	}
	exit, rep = run(t, a, "steps")
	var got struct{ Session project.Session }
	json.Unmarshal(rep.Result, &got)
	if s := got.Session; exit != 0 || s.ID != "th-1" || s.Steps != 1 || s.Turns[0].Note != "Make the notes longer" ||
		s.Turns[0].Steps[0].Detail != "Longer notes" {
		t.Fatalf("steps: %s", rep.Result)
	}
	if exit, rep = run(t, a, "step", "start", "extra"); exit != 2 || rep.Error.Code != "usage" {
		t.Fatalf("usage: %d %+v", exit, rep.Error)
	}

	asAgent(t)
	if _, err := agentsetup.ClaudeCode(a, "r3v hook pre-tool", false, true); err != nil {
		t.Fatal(err)
	}
	exit, rep = run(t, a, "step", "start", "-m", "anything")
	if exit != 0 || !strings.Contains(string(rep.Result), `"hooked": true`) {
		t.Fatalf("Claude Code with hooks: %d %s", exit, rep.Result)
	}
	r, err := project.Open(a)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := r.Sessions(); len(s) != 1 {
		t.Fatalf("sessions: %+v", s)
	}
}

// A commit ends the task an agent began and left open (step_ended, its
// step named for the commit), so the version lists its steps; with none
// open it says nothing of steps.
func TestCommitEndsTheOpenTask(t *testing.T) {
	asUnknown(t)
	t.Setenv("CODEX_THREAD_ID", "th-c")
	a := newFolder(t, map[string]string{"notes.txt": "one"})
	if _, err := project.Init(a, "yi"); err != nil {
		t.Fatal(err)
	}
	run(t, a, "commit", "-m", "first")
	if exit, rep := run(t, a, "step", "start", "-m", "Make the notes longer"); exit != 0 {
		t.Fatalf("start: %+v", rep.Error)
	}
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("one, two"), 0o644)
	exit, rep := run(t, a, "commit", "-m", "Longer notes")
	var out struct {
		Saved     *struct{ ID string }
		StepEnded *project.StepEnded `json:"step_ended"`
	}
	json.Unmarshal(rep.Result, &out)
	if exit != 0 || out.Saved == nil || out.StepEnded == nil || out.StepEnded.Turn != 1 || out.StepEnded.Step == nil ||
		out.StepEnded.Step.Detail != project.CommitEndDetail+"Longer notes" {
		t.Fatalf("commit: %d %s %+v", exit, rep.Result, rep.Error)
	}
	r, err := project.Open(a)
	if err != nil {
		t.Fatal(err)
	}
	r.Actor = &project.Actor{Kind: "agent", Tool: "codex", Session: "th-c"}
	if r.OpenTask() != nil {
		t.Fatal("still open")
	}
	if got, _ := r.VersionSteps(out.Saved.ID); len(got) != 1 || got[0].Steps != 1 {
		t.Fatalf("the version's steps: %+v", got)
	}
	if exit, text := runText(t, a, "commit", "-m", "nothing"); exit != 0 || strings.Contains(text, "ended") {
		t.Fatalf("none open: %d %q", exit, text)
	}
	os.WriteFile(filepath.Join(a, "notes.txt"), []byte("three"), 0o644)
	if exit, rep := run(t, a, "commit", "-m", "Third"); exit != 0 || strings.Contains(string(rep.Result), "step_ended") {
		t.Fatalf("none open: %d %s", exit, rep.Result)
	}
}
