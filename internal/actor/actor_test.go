package actor

import (
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
