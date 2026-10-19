package agentsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/actor"
)

// Set up for a tool other than Claude Code: its identity and the guide in
// AGENTS.md, nothing of Claude Code's; for Claude Code, its settings and
// CLAUDE.md too. Again, nothing changes; a workspace (no guide) gets the
// identity only. CopyTool reads either.
func TestSetup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".r3v")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Ours\n\nKeep it short.\n"), 0o644)

	written, id, err := Setup(root, dir, Options{Tool: "pi", Model: "kimi-k2", Guide: true})
	if err != nil || strings.Join(written, ",") != "AGENTS.md,.r3v/agent.json" || id != (actor.Identity{Tool: "pi", Model: "kimi-k2"}) {
		t.Fatalf("pi: %v %+v %v", written, id, err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); !strings.HasPrefix(string(data), "# Ours\n\nKeep it short.\n\n"+guideBegin) {
		t.Fatalf("AGENTS.md: %q", data)
	}
	for _, f := range []string{"CLAUDE.md", ".claude"} {
		if _, err := os.Stat(filepath.Join(root, f)); !os.IsNotExist(err) {
			t.Errorf("%s written for pi", f)
		}
	}
	if written, id, _ := Setup(root, dir, Options{Tool: "pi", Guide: true}); len(written) != 0 || id.Model != "kimi-k2" {
		t.Fatalf("again: %v %+v", written, id)
	}
	if CopyTool(root) != "pi" || ToolOf(root) != "" {
		t.Fatalf("CopyTool %q, ToolOf %q", CopyTool(root), ToolOf(root))
	}

	// Now Claude Code: a model of pi's isn't carried over.
	written, id, err = Setup(root, dir, Options{Tool: ClaudeCodeTool, Hook: "r3v hook pre-tool", Guide: true})
	if err != nil || strings.Join(written, ",") != ".claude/settings.local.json,CLAUDE.md,.r3v/agent.json" || id.Model != "" {
		t.Fatalf("claude-code: %v %+v %v", written, id, err)
	}
	if CopyTool(root) != ClaudeCodeTool {
		t.Fatalf("CopyTool %q", CopyTool(root))
	}

	ws := t.TempDir()
	os.MkdirAll(filepath.Join(ws, ".r3v"), 0o755)
	if written, _, err := Setup(ws, filepath.Join(ws, ".r3v"), Options{Tool: "codex"}); err != nil || strings.Join(written, ",") != ".r3v/agent.json" {
		t.Fatalf("a workspace: %v %v", written, err)
	}
}
