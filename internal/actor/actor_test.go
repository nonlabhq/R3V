package actor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonlabhq/r3v/internal/manifest"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestFromEnv(t *testing.T) {
	for _, c := range []struct {
		name string
		vars map[string]string
		want *manifest.Actor
	}{
		{"a person", nil, nil},
		{"claude code announces itself", map[string]string{"CLAUDECODE": "1"},
			&manifest.Actor{Kind: "agent", Tool: "claude-code"}},
		{"only CLAUDECODE=1", map[string]string{"CLAUDECODE": "0"}, nil},
		{"R3V_ACTOR wins", map[string]string{"CLAUDECODE": "1", EnvTool: " codex ", EnvModel: "gpt", EnvSession: "s1"},
			&manifest.Actor{Kind: "agent", Tool: "codex", Model: "gpt", Session: "s1"}},
		{"model and session with a tool that announces itself",
			map[string]string{"CLAUDECODE": "1", EnvModel: "opus", EnvSession: "abc"},
			&manifest.Actor{Kind: "agent", Tool: "claude-code", Model: "opus", Session: "abc"}},
		{"model alone is no actor", map[string]string{EnvModel: "opus"}, nil},
		{"none says a person", map[string]string{"CLAUDECODE": "1", EnvTool: "None"}, nil},
		{"one line", map[string]string{EnvTool: "tool\nmore"}, &manifest.Actor{Kind: "agent", Tool: "tool"}},
	} {
		got := FromEnv(env(c.vars))
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	long := FromEnv(env(map[string]string{EnvTool: strings.Repeat("é", 300)}))
	if n := len([]rune(long.Tool)); n != 100 {
		t.Fatalf("long tool name kept %d runes", n)
	}
}

// R3V_ACTOR, then the copy's identity, then a tool's own marker, else a
// person; a marker of another tool than the copy's wins over it.
func TestResolveOrder(t *testing.T) {
	opencode := &Identity{Tool: "opencode", Model: "glm-4.6"}
	for _, c := range []struct {
		name string
		vars map[string]string
		id   *Identity
		want *manifest.Actor
		from string
	}{
		{"nothing: a person", nil, nil, nil, ""},
		{"the copy's identity", nil, opencode,
			&manifest.Actor{Kind: "agent", Tool: "opencode", Model: "glm-4.6"}, FromCopy},
		{"R3V_ACTOR over the copy's", map[string]string{EnvTool: "codex"}, opencode,
			&manifest.Actor{Kind: "agent", Tool: "codex"}, FromVariable},
		{"R3V_ACTOR naming the copy's tool keeps its model", map[string]string{EnvTool: "opencode"}, opencode,
			&manifest.Actor{Kind: "agent", Tool: "opencode", Model: "glm-4.6"}, FromVariable},
		{"R3V_ACTOR_MODEL over the copy's model", map[string]string{EnvModel: "glm-5"}, opencode,
			&manifest.Actor{Kind: "agent", Tool: "opencode", Model: "glm-5"}, FromCopy},
		{"none over the copy's: a person", map[string]string{EnvTool: "none"}, opencode, nil, ""},
		{"the copy's over its own tool's marker", map[string]string{"CLAUDECODE": "1"}, &Identity{Tool: "claude-code", Model: "opus"},
			&manifest.Actor{Kind: "agent", Tool: "claude-code", Model: "opus"}, FromCopy},
		{"another tool's marker over the copy's", map[string]string{"CLAUDECODE": "1"}, opencode,
			&manifest.Actor{Kind: "agent", Tool: "claude-code"}, FromTool},
		{"a marker, no copy's", map[string]string{"CLAUDECODE": "1"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "claude-code"}, FromTool},
		{"Codex's thread is its session", map[string]string{"CODEX_THREAD_ID": "t-9"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "codex", Session: "t-9"}, FromTool},
		{"a tool's variables for another tool aren't read", map[string]string{"CLAUDE_CODE_SESSION_ID": "s", EnvTool: "codex"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "codex"}, FromVariable},
		{"a tool started from Claude Code is that tool", map[string]string{"CLAUDECODE": "1", "CURSOR_AGENT": "1"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "cursor"}, FromTool},
		{"AI_AGENT names a tool", map[string]string{"AI_AGENT": "opencode"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "opencode"}, FromTool},
		{"AI_AGENT that can't be a name", map[string]string{"AI_AGENT": "Some Agent 2.0"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "agent"}, FromTool},
		{"a tool's own marker over AI_AGENT", map[string]string{"AI_AGENT": "claude-code_2.1_agent", "CLAUDECODE": "1"}, nil,
			&manifest.Actor{Kind: "agent", Tool: "claude-code"}, FromTool},
		{"the copy's over AI_AGENT", map[string]string{"AI_AGENT": "pi"}, opencode,
			&manifest.Actor{Kind: "agent", Tool: "opencode", Model: "glm-4.6"}, FromCopy},
		{"former markers aren't read", map[string]string{"PI_CODING_AGENT": "true", "GEMINI_CLI": "1", "OPENCODE": "1"}, nil, nil, ""},
	} {
		got, from := Resolve(env(c.vars), c.id)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want || from != c.from {
			t.Errorf("%s: got %+v from %q, want %+v from %q", c.name, got, from, c.want, c.from)
		}
	}
}

// Each tool's marker says that tool, and only with its value.
func TestAnnounced(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range markers {
		if seen[m.Var] {
			t.Errorf("%s listed twice", m.Var)
		}
		seen[m.Var] = true
		value := m.Value
		if value == "" {
			value = "anything"
		}
		if got := Announced(env(map[string]string{m.Var: value})); got != m.Tool {
			t.Errorf("%s=%s: %q, want %q", m.Var, value, got, m.Tool)
		}
		if m.Value != "" {
			if got := Announced(env(map[string]string{m.Var: m.Value + "x"})); got != "" {
				t.Errorf("%s=%sx: %q", m.Var, m.Value, got)
			}
		}
		if _, ok := ToolName(m.Tool); !ok {
			t.Errorf("%q can't be a tool's name", m.Tool)
		}
	}
	if got := Announced(env(map[string]string{"TERM_PROGRAM": "vscode"})); got != "" {
		t.Errorf("a terminal isn't an agent: %q", got)
	}
}

func TestIdentityFile(t *testing.T) {
	dir := t.TempDir()
	if ReadIdentity(dir) != nil {
		t.Fatal("an identity from nothing")
	}
	id := Identity{Tool: "pi", Model: "kimi-k2"}
	if changed, err := WriteIdentity(dir, id); err != nil || !changed {
		t.Fatalf("write: %v %v", changed, err)
	}
	if changed, err := WriteIdentity(dir, id); err != nil || changed {
		t.Fatalf("again: %v %v", changed, err)
	}
	if got := ReadIdentity(dir); got == nil || *got != id {
		t.Fatalf("read: %+v", got)
	}
	os.WriteFile(filepath.Join(dir, IdentityFile), []byte("{broken"), 0o644)
	if got := ReadIdentity(dir); got != nil {
		t.Fatalf("a broken file: %+v", got)
	}
	for name, ok := range map[string]bool{"claude-code": true, " Codex ": true, "gemini-cli": true, "none": false,
		"two words": false, "": false, "-x": false, strings.Repeat("a", 41): false} {
		if _, got := ToolName(name); got != ok {
			t.Errorf("ToolName(%q): %v", name, got)
		}
	}
}
