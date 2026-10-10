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
		{[]string{"profile", "--help"}, "profile explain", ""},
		{[]string{"profile", "preset", "--help"}, "profile preset <folder>", "profile check"},
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
