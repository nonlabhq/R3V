package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/transcript"
)

// Steps (docs/design/steps.md): the agent tool's hooks make them (`r3v hook
// prompt`, `r3v hook post-tool`); `r3v steps` lists them, `r3v step` makes
// one by hand, goes back to one, compares two.

const stepsUsage = `
steps (an agent's moves, on this computer; Nightly):
  steps [--session ID] [--list] [--json] the latest session's prompts and steps (--list: the sessions)
  step [-m NOTE]                         a step of the files as they are now
  step back <n> [--session ID]           the files as at step n (the files now are a step first)
  step diff <a> [<b>] [--session ID]     what changed from step a to step b (or to the files now)
`

// hookIn is what Claude Code tells a prompt or post-tool hook.
type hookIn struct {
	SessionID      string          `json:"session_id"`
	Cwd            string          `json:"cwd"`
	ToolName       string          `json:"tool_name"`
	Prompt         string          `json:"prompt"`
	TranscriptPath string          `json:"transcript_path"`
	ToolInput      json.RawMessage `json:"tool_input"`
}

// readOnlyTools change no files: no scan after them.
var readOnlyTools = map[string]bool{"Read": true, "Grep": true, "Glob": true, "LS": true, "WebFetch": true,
	"WebSearch": true, "TodoWrite": true, "Task": true, "ExitPlanMode": true, "BashOutput": true}

// stepSlow: a step taking longer is worth telling once (big projects).
const stepSlow = 500 * time.Millisecond

// hookSteps runs the prompt or post-tool hook: never in the agent's way
// (any error says nothing), never waiting long for the project.
func hookSteps(kind string, in io.Reader, out io.Writer) {
	if !project.Steps {
		return
	}
	var h hookIn
	if json.NewDecoder(io.LimitReader(in, 16<<20)).Decode(&h) != nil || h.SessionID == "" {
		return
	}
	if kind == "post-tool" && readOnlyTools[h.ToolName] {
		return
	}
	dir := h.Cwd
	if dir == "" {
		dir = "."
	}
	r, err := openAgent(dir)
	if err != nil || !r.StepsOn() {
		return
	}
	release, err := r.Lock(3 * time.Second)
	if err != nil {
		return // the project is busy (a commit, the app tidying): no step this time
	}
	defer release()
	if kind == "prompt" {
		r.StepTurn(h.SessionID, h.Prompt, h.TranscriptPath)
		return
	}
	start := time.Now()
	r.TakeStepWith(h.SessionID, h.ToolName, transcript.Detail(h.ToolInput), "")
	if took := time.Since(start); took > stepSlow {
		noted := filepath.Join(r.Dir, "steps", ".slow")
		if _, err := os.Stat(noted); err == nil {
			return
		}
		os.WriteFile(noted, nil, 0o644)
		json.NewEncoder(out).Encode(map[string]string{"systemMessage": fmt.Sprintf(
			"R3V: a step of this project took %.1f s after a tool call. If that slows the agent down, steps can be turned off in R3V's project settings.",
			took.Seconds())})
	}
}

func cmdSteps(args []string) error {
	fs := flag.NewFlagSet("steps", flag.ContinueOnError)
	session := fs.String("session", "", "a session's id (default: the latest)")
	list := fs.Bool("list", false, "list the sessions")
	if err := fs.Parse(args); err != nil {
		return usageError("usage: r3v steps [--session ID] [--list]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if *list {
		all, err := r.Sessions()
		if err != nil {
			return err
		}
		type row struct {
			ID    string `json:"id"`
			Last  string `json:"last"`
			Steps int    `json:"steps"`
			Turns int    `json:"turns"`
		}
		rows := []row{}
		for _, s := range all {
			rows = append(rows, row{s.ID, s.Last, s.Steps, len(s.Turns)})
		}
		result("steps", map[string]any{"sessions": rows}, func() {
			if len(rows) == 0 {
				fmt.Println("no steps yet")
			}
			for _, s := range rows {
				fmt.Printf("%s  %d step(s), %d prompt(s), latest %s\n", s.ID, s.Steps, s.Turns, localTime(s.Last))
			}
		})
		return nil
	}
	s, err := r.SessionSteps(*session)
	if errors.Is(err, project.ErrNoStep) && *session == "" {
		result("steps", map[string]any{"session": nil}, func() { fmt.Println("no steps yet") })
		return nil
	}
	if err != nil {
		return err
	}
	result("steps", map[string]any{"session": s}, func() {
		fmt.Printf("session %s: %d step(s)\n", s.ID, s.Steps)
		for _, t := range s.Turns {
			switch {
			case t.Prompt != "":
				from, words := promptWords(t.Prompt)
				if from != "" {
					words = from + ": " + words
				}
				fmt.Printf("\n> %s\n", oneLine(words, 100))
			case t.Note != "":
				fmt.Printf("\n# %s\n", oneLine(t.Note, 100))
			}
			for _, st := range t.Steps {
				tool := st.Tool
				if st.Detail != "" && !slices.Contains(st.Files, st.Detail) {
					tool += " · " + oneLine(st.Detail, 50)
				}
				fmt.Printf("  %3d  %s  %-14s %s\n", st.N, localTime(st.Time), tool, filesLine(st.Files))
			}
		}
	})
	return nil
}

func cmdStep(args []string) error {
	if len(args) > 0 && (args[0] == "back" || args[0] == "diff") {
		return cmdStepGo(args[0], args[1:])
	}
	fs := flag.NewFlagSet("step", flag.ContinueOnError)
	note := fs.String("m", "", "a note for the step")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return usageError("usage: r3v step [-m NOTE] | r3v step back <n> | r3v step diff <a> [<b>]")
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if !r.StepsOn() {
		return errors.New("steps are off for this project (R3V's project settings)")
	}
	st, err := r.TakeStep(project.ByHand, "r3v step", *note)
	if err != nil {
		return err
	}
	result("step", map[string]any{"step": st}, func() {
		if st == nil {
			fmt.Println("nothing changed since the step before")
			return
		}
		fmt.Printf("step %d: %s\n", st.N, filesLine(st.Files))
	})
	return nil
}

func cmdStepGo(what string, args []string) error {
	fs := flag.NewFlagSet("step "+what, flag.ContinueOnError)
	session := fs.String("session", "", "a session's id (default: the latest)")
	var nums []int
	var rest []string
	for len(args) > 0 { // numbers and flags in any order
		if n, err := strconv.Atoi(args[0]); err == nil {
			nums, args = append(nums, n), args[1:]
			continue
		}
		rest, args = append(rest, args[0]), args[1:]
	}
	usage := usageError("usage: r3v step back <n> [--session ID] | r3v step diff <a> [<b>] [--session ID]")
	if err := fs.Parse(rest); err != nil || fs.NArg() > 0 || len(nums) == 0 || (what == "back" && len(nums) != 1) || len(nums) > 2 {
		return usage
	}
	r, err := openRepo()
	if err != nil {
		return err
	}
	if what == "diff" {
		b := 0
		if len(nums) == 2 {
			b = nums[1]
		}
		changes, err := r.StepDiff(*session, nums[0], b)
		if err != nil {
			return err
		}
		result("step", map[string]any{"changes": changes}, func() {
			if len(changes) == 0 {
				fmt.Println("no files changed")
			}
			for _, c := range changes {
				fmt.Printf("  %-8s %s\n", c.Status, c.Path)
			}
		})
		return nil
	}
	if err := guardLiveAlways(r, false); err != nil {
		return err
	}
	back, notes, err := r.StepBack(*session, nums[0])
	if err != nil {
		return err
	}
	result("step", map[string]any{"step": back, "relink": nonNil(notes)}, func() {
		fmt.Printf("the files are as at step %d (step %d; the files before are step %d)\n", nums[0], back.N, back.N-1)
		for _, n := range notes {
			fmt.Println("  relinked " + n)
		}
	})
	return nil
}

// relayed is a message another session sent this one (Claude Code wraps
// it so): who from, and its words.
var relayed = regexp.MustCompile(`(?s)^\s*<cross-session-message\b([^>]*)>(.*?)(?:</cross-session-message>\s*)?$`)

var fromName = regexp.MustCompile(`from-name="([^"]*)"`)

// promptWords is a prompt's words, and who relayed it ("": the person's).
func promptWords(p string) (from, words string) {
	m := relayed.FindStringSubmatch(p)
	if m == nil {
		return "", p
	}
	if n := fromName.FindStringSubmatch(m[1]); n != nil {
		from = n[1]
	}
	return from, strings.TrimSpace(m[2])
}

func localTime(t string) string {
	if tm, err := time.Parse(time.RFC3339, t); err == nil {
		return tm.Local().Format("01-02 15:04")
	}
	return t
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n]) + "…"
	}
	return s
}

func filesLine(files []string) string {
	if len(files) <= 3 {
		return strings.Join(files, ", ")
	}
	return fmt.Sprintf("%s (+%d)", strings.Join(files[:3], ", "), len(files)-3)
}
