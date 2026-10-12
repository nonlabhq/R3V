package transcript

import (
	"strings"
	"testing"
)

// A conversation as Claude Code keeps it: prompts, replies and tool calls
// come out in order; thinking, tool output and the tool's own lines don't;
// keys are taken out.
func TestParse(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"type":"user","timestamp":"2026-10-12T04:49:05Z","message":{"content":"Make it warmer; the key is AKIAIOSFODNN7EXAMPLE"}}`,
		`{"type":"assistant","timestamp":"2026-10-12T04:49:07Z","message":{"content":[{"type":"thinking","thinking":"secret thoughts"}]}}`,
		`{"type":"assistant","timestamp":"2026-10-12T04:49:09Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cargo test\nmore","description":"Run the scene tests"}}]}}`,
		`{"type":"user","timestamp":"2026-10-12T04:49:20Z","message":{"content":[{"type":"tool_result","content":"214 passed"}]}}`,
		`{"type":"assistant","timestamp":"2026-10-12T04:49:30Z","message":{"content":[{"type":"text","text":"The tests pass; warmer light is in."}]}}`,
		`{"type":"assistant","timestamp":"2026-10-12T04:49:31Z","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"r3v commit -m done"}}]}}`,
		`{"type":"user","timestamp":"2026-10-12T04:50:00Z","isMeta":true,"message":{"content":"(a reminder the tool added)"}}`,
		`{"type":"user","timestamp":"2026-10-12T04:50:01Z","message":{"content":"<command-name>/clear</command-name>"}}`,
		`not json`,
		`{"type":"attachment","timestamp":"2026-10-12T04:50:02Z"}`,
	}, "\n")
	got, err := parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range got {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "prompt,tool,reply,tool" {
		t.Fatalf("entries: %+v", got)
	}
	if strings.Contains(got[0].Text, "AKIA") || got[1].Detail != "Run the scene tests" || got[3].Detail != "r3v commit -m done" ||
		got[2].Text != "The tests pass; warmer light is in." {
		t.Fatalf("entries: %+v", got)
	}
	if _, err := Read("C:/not/a/transcript.txt"); err != ErrNotTranscript {
		t.Fatalf("a file that isn't .jsonl: %v", err)
	}
}
