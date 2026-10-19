package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R3V's part of a project's AGENTS.md is brought up to date in its place:
// one an older R3V wrote (without markers, up to the next heading) and one
// with markers; the project's own text before and after stays.
func TestGuideUpdated(t *testing.T) {
	for name, nl := range map[string]string{"lf": "\n", "crlf": "\r\n"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "AGENTS.md")
			lines := func(ls ...string) string { return strings.Join(ls, nl) + nl }
			os.WriteFile(path, []byte(lines("# Our project", "", "Build with pnpm.", "",
				guideHeading(), "", "- an older line R3V wrote", "", "## Our checks", "", "Run the tests.")), 0o644)
			if !HasGuide(root) || !GuideOld(root) {
				t.Fatal("an older guide not seen")
			}
			if _, err := Guide(root); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(path)
			text := string(data)
			if GuideOld(root) || strings.Contains(text, "an older line") || !strings.Contains(text, guideBegin) ||
				!strings.HasPrefix(text, "# Our project") || !strings.Contains(text, "Run the tests.") ||
				strings.Count(text, guideHeading()) != 1 {
				t.Fatalf("AGENTS.md:\n%s", text)
			}
			if written, _ := Guide(root); len(written) != 0 {
				t.Fatalf("written again: %v", written)
			}

			// Changed between the markers: put back; the rest stays.
			edited := strings.Replace(text, guideEnd, "- a line someone added"+nl+guideEnd, 1)
			os.WriteFile(path, []byte(edited+"After it."+nl), 0o644)
			if !GuideOld(root) {
				t.Fatal("a changed part not seen")
			}
			Guide(root)
			data, _ = os.ReadFile(path)
			if strings.Contains(string(data), "someone added") || !strings.HasSuffix(string(data), "After it."+nl) ||
				!strings.Contains(string(data), "Run the tests.") {
				t.Fatalf("AGENTS.md:\n%s", data)
			}
		})
	}
}

// A project without AGENTS.md gets R3V's part, marked, and CLAUDE.md the
// line that reads it.
func TestGuideNew(t *testing.T) {
	root := t.TempDir()
	written, err := Guide(root)
	if err != nil || len(written) != 2 {
		t.Fatalf("%v %v", written, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(data) != guideBlock() || GuideOld(root) {
		t.Fatalf("AGENTS.md:\n%s", data)
	}
	claude, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(claude) != "@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md: %q", claude)
	}
}

// The guide tells tools without hooks to wrap each task in step start
// and step end, and commit after (a commit ending a task left open);
// Claude Code, set up, is told its hooks
// make the steps, and given no commands for them.
func TestGuideSteps(t *testing.T) {
	block := guideBlock()
	for _, want := range []string{
		"Claude Code set up with\n  `r3v agent setup` makes them with its hooks: nothing to do.",
		"Any other tool: begin each task with\n  `r3v step start -m \"what you're about to do\" --json`",
		"`r3v step end -m \"what you did\" --json`, then commit if the work is finished (a commit also\n  ends the task left open).",
	} {
		if strings.Count(block, want) != 1 {
			t.Fatalf("the guide lacks %q:\n%s", want, block)
		}
	}
	if strings.Count(block, "r3v step start") != 1 || strings.Count(block, "r3v step end") != 1 {
		t.Fatalf("step commands said more than once:\n%s", block)
	}
}

// A CRLF AGENTS.md keeps its line ends.
func TestGuideKeepsLineEnds(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	os.WriteFile(path, []byte("# Ours\r\n\r\nText.\r\n"), 0o644)
	Guide(root)
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "\n") != strings.Count(string(data), "\r\n") {
		t.Fatalf("mixed line ends: %q", data)
	}
}
