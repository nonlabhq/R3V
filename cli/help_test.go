package cli

import (
	"strings"
	"testing"
)

// Every command and subcommand answers --help with its lines from the
// usage, not an error.
func TestHelpFor(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
		not  string
	}{
		{[]string{"profile", "--help"}, "rules explain", ""},
		{[]string{"rules", "preset", "--help"}, "rules preset <folder>", "rules [check]"},
		{[]string{"save", "-h"}, "commit --local", ""},
		{[]string{"branch", "archive", "-h"}, "branch archive NAME", "branch new"},
	} {
		if !wantsHelp(c.args[1:]) {
			t.Fatalf("%v: not a help request", c.args)
		}
		text, ok := helpFor(c.args[0], c.args[1:])
		if !ok || !strings.Contains(text, c.want) || (c.not != "" && strings.Contains(text, c.not)) {
			t.Errorf("%v: %v\n%s", c.args, ok, text)
		}
	}
	if wantsHelp([]string{"-m", "-h"}) || wantsHelp([]string{"--", "--help"}) {
		t.Error("a message or a file named -h taken for help")
	}
	if _, ok := helpFor("nosuch", nil); ok {
		t.Error("help for a command R3V doesn't have")
	}
	if !strings.Contains(usage, "run inside a project folder") {
		t.Error("the usage still says Live projects only")
	}
}

// rules is profile's new name; both answer.
func TestRulesName(t *testing.T) {
	text, ok := helpFor("profile", []string{"--help"})
	if !ok || !strings.Contains(text, "rules explain") || strings.Contains(usage, "  profile ") {
		t.Errorf("profile help: %v\n%s", ok, text)
	}
}

// AI agents told only "see r3v -h" read its first lines: they lead to the
// guide and agent setup (every build's) before anything else,
// and the setup section lists the steps in the order a newcomer takes
// them: sign in, the teams, init or connect, then the agent's setup.
func TestHelpLeadsAgents(t *testing.T) {
	check := func(t *testing.T) {
		code, text := runText(t, t.TempDir(), "-h")
		if code != 0 {
			t.Fatalf("-h: %d", code)
		}
		lines := strings.Split(text, "\n")
		if len(lines) < 4 || !strings.Contains(lines[2], `run "r3v help agents" first`) {
			t.Fatalf("the first lines don't send agents to the guide:\n%s", strings.Join(lines[:min(5, len(lines))], "\n"))
		}
		top := strings.Join(lines[:4], "\n")
		if !strings.Contains(top, `"connected"`) || strings.Index(text, "r3v help agents") > strings.Index(text, "everyday") {
			t.Errorf("top:\n%s", top)
		}
		if !strings.Contains(top, "agent setup") {
			t.Errorf("no agent setup in the top lines:\n%s", top)
		}
		at := func(s string) int {
			i := strings.Index(text, s)
			if i < 0 {
				t.Fatalf("no %q in the usage", s)
			}
			return i
		}
		order := []string{"setup (the first time", "\n  login ", "\n  teams ", "\n  init [", "\n  init --team NAME",
			"\n  remote --team NAME", "\n  remote <connection-code>", "\n  clone --team NAME", "\n  agent setup [--tool NAME]"}
		for i := 1; i < len(order); i++ {
			if at(order[i-1]) > at(order[i]) {
				t.Errorf("%q comes before %q", order[i], order[i-1])
			}
		}
		if strings.Contains(text, "{{") {
			t.Error("a placeholder left in the usage")
		}
		if !strings.Contains(text, "\n  project archive") || !strings.Contains(text, "\n  move-team") {
			t.Error("a section of every build's lost from the usage")
		}
		if help, ok := helpFor("login", nil); !ok || strings.Count(help, "login [") != 1 {
			t.Errorf("login --help:\n%s", help)
		}
	}
	t.Run("this build", check)
	t.Run("agent locks on", func(t *testing.T) {
		withAgentLocks(t)
		check(t)
	})
}
