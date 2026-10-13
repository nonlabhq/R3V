package cli

import (
	"strings"
)

// wantsHelp: -h or --help anywhere among a command's arguments (before a
// lone "--"), so every command and subcommand answers it the same way.
func wantsHelp(args []string) bool {
	for i, a := range args {
		if i > 0 && (args[i-1] == "-m" || args[i-1] == "--message") {
			continue // a message that reads "-h"
		}
		switch a {
		case "--":
			return false
		case "-h", "--help", "-help":
			return true
		}
	}
	return false
}

// helpAliases are the commands' older names, shown under the current one.
var helpAliases = map[string]string{"save": "commit", "snapshot": "commit", "checkout": "goto", "profile": "rules", "tag": "milestone", "tags": "milestones"}

// helpFor is a command's lines from the usage (with their continuation
// lines); with a subcommand among args (r3v rules preset --help), only
// its lines when there are any. The whole usage for a command it doesn't
// list, and false.
func helpFor(command string, args []string) (string, bool) {
	if c, ok := helpAliases[command]; ok {
		command = c
	}
	sub := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			sub = a
			break
		}
	}
	all := usage + extraUsage
	pick := func(sub string) []string {
		var out []string
		taking := false
		for _, line := range strings.Split(all, "\n") {
			trimmed := strings.TrimLeft(line, " ")
			indent := len(line) - len(trimmed)
			if indent >= 10 { // a continuation of the line above
				if taking {
					out = append(out, line)
				}
				continue
			}
			taking = false
			if indent < 2 || trimmed == "" {
				continue
			}
			words := strings.Fields(trimmed)
			if words[0] != command {
				continue
			}
			if sub != "" && (len(words) < 2 || strings.TrimSuffix(words[1], "|") != sub && !strings.HasPrefix(words[1], sub+"|") && !strings.Contains(words[1], "|"+sub)) {
				continue
			}
			taking = true
			out = append(out, line)
		}
		return out
	}
	lines := pick(sub)
	if len(lines) == 0 {
		lines = pick("")
	}
	if len(lines) == 0 {
		return all, false
	}
	return "usage: r3v " + command + " ...\n" + strings.Join(lines, "\n") + "\n\nall commands: r3v help\n", true
}
