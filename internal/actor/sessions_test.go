package actor

import (
	"os"
	"path/filepath"
	"testing"
)

// The model Claude Code's hooks recorded for the session counts when no
// variable says one: before the copy's identity's (what runs now), after
// R3V_ACTOR_MODEL; another session's or another tool's record doesn't.
func TestSessionModel(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".r3v")
	os.MkdirAll(dir, 0o755)
	WriteIdentity(dir, Identity{Tool: "claude-code", Model: "set-up-model"})
	if err := RecordModel(dir, "claude-code", "sess-1", "claude-opus-5-5"); err != nil {
		t.Fatal(err)
	}
	RecordModel(dir, "codex", "sess-2", "gpt-6")
	RecordModel(dir, "claude-code", "../escape", "nope")
	for _, c := range []struct {
		name string
		vars map[string]string
		want string
	}{
		{"recorded", map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": "sess-1"}, "claude-opus-5-5"},
		{"R3V_ACTOR_SESSION", map[string]string{EnvTool: "claude-code", EnvSession: "sess-1"}, "claude-opus-5-5"},
		{"the variable wins", map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": "sess-1", EnvModel: "mine"}, "mine"},
		{"another session", map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": "sess-9"}, "set-up-model"},
		{"another tool's record", map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": "sess-2"}, "set-up-model"},
		{"no session", map[string]string{"CLAUDECODE": "1"}, "set-up-model"},
	} {
		a, _ := ResolveIn(env(c.vars), dir)
		if a == nil || a.Model != c.want {
			t.Errorf("%s: %+v, want model %q", c.name, a, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.json")); !os.IsNotExist(err) {
		t.Fatal("a session id named a file outside")
	}
	// Resolve (no copy) knows no records.
	if a, _ := Resolve(env(map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_SESSION_ID": "sess-1"}), nil); a.Model != "" {
		t.Fatalf("Resolve: %+v", a)
	}
	// A model switch replaces it.
	RecordModel(dir, "claude-code", "sess-1", "claude-sonnet-5")
	if m := SessionModel(dir, "claude-code", "sess-1"); m != "claude-sonnet-5" {
		t.Fatalf("after a switch: %q", m)
	}
}
