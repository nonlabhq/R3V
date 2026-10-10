package profile

import (
	"strings"
	"testing"
)

// A project picks the merge of its own files: later handlers win, then the
// presets'; "whole" turns a preset's merge off.
func TestProjectHandlers(t *testing.T) {
	was := ProjectHandlers
	defer func() { ProjectHandlers = was }()
	ProjectHandlers = true
	const yaml = `presets:
  ./: none
handlers:
  - files: ["script/**/*.md", "*.scene"]
    merge: text
  - files: ["script/generated/"]
    merge: whole
`
	p, err := Parse([]byte(yaml), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"script/a/notes.md": "text", "levels/one.scene": "text", "readme.md": "",
		"script/generated/api.md": "",
	} {
		if got := p.Handler(rel).Merge; got != want {
			t.Errorf("%s: merge %q, want %q", rel, got, want)
		}
	}
	for _, bad := range []string{
		"handlers:\n  - files: [\"*.x\"]\n    merge: ableton-set\n",
		"handlers:\n  - merge: text\n",
		"handlers:\n  - files: [\"*.x\"]\n    merge: text\n    samples: ableton\n",
	} {
		if _, err := Parse([]byte(bad), t.TempDir()); err == nil {
			t.Errorf("taken: %s", bad)
		}
	}
	// Stable: refused, saying where they are.
	ProjectHandlers = false
	if _, err := Parse([]byte(yaml), t.TempDir()); err == nil || !strings.Contains(err.Error(), "Nightly") {
		t.Errorf("Stable: %v", err)
	}
}
