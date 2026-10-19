package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/agentsetup"
	"github.com/nonlabhq/r3v/internal/project"
	"github.com/nonlabhq/r3v/internal/remote"
	"github.com/nonlabhq/r3v/internal/teams"
)

// What to run next, after init and while a project isn't connected: AI
// agents in field tests made projects that stayed on one computer, never
// connected to the person's team and never set up as an agent's, because
// nothing in the output led there. `next` in --json says it as commands.

// nextJSON is a command worth running next, and why (in English).
type nextJSON struct {
	Command string `json:"command"`
	Why     string `json:"why"`
}

// usableTeams are the teams this computer has joined that a project can
// be connected to now (not a hosted team the account left, nor one whose
// keys another Windows user sealed).
func usableTeams() []teams.Team {
	store, err := teams.Load()
	if err != nil {
		return nil
	}
	var out []teams.Team
	for _, t := range store.Teams {
		if !t.NoAccess && !t.KeysUnreadable {
			out = append(out, t)
		}
	}
	return out
}

// shellArg quotes a name for a command line when it needs it.
func shellArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'&|<>()^%!;") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// connectSteps are the ways to connect the project here to a team: by
// name to each team this computer has joined (Nightly), else signing in
// to R3V Cloud or a connection code (the person's to enter: it holds the
// storage's keys).
func connectSteps() []nextJSON {
	joined := usableTeams()
	var names []string
	for _, t := range joined {
		names = append(names, t.Name)
	}
	var out []nextJSON
	switch {
	case len(joined) > 0 && teamByName:
		for _, t := range joined {
			out = append(out, nextJSON{"r3v remote --team " + shellArg(t.Name),
				fmt.Sprintf("connect this project to the team %s (this computer has joined it): its versions are shared with the team", t.Name)})
		}
	case len(joined) > 0:
		out = append(out, nextJSON{"r3v remote <connection-code>",
			"connect this project to a team: this computer has joined " + strings.Join(names, ", ") +
				"; add the project to one in the R3V app, or the person enters the team's connection code (it holds the storage keys: never ask for it or print it)"})
	default:
		if remote.HostedTeams {
			why := "sign in to R3V Cloud (the person, in the browser): the teams they are in then appear in `r3v teams`"
			if teamByName {
				why += ", and `r3v remote --team NAME` connects this project to one"
			} else {
				why += "; then add the project to one in the R3V app"
			}
			out = append(out, nextJSON{"r3v login", why})
		}
		out = append(out, nextJSON{"r3v remote <connection-code>",
			"or connect it to a self-hosted team's storage with its connection code: the person enters it (it holds the storage keys: never ask for it or print it)"})
	}
	return out
}

// agentSteps is what an AI agent does once in a project: read the guide
// when R3V can't tell an agent runs it; set Claude Code up where it can
// and the copy isn't yet. A person (R3V_ACTOR=none) gets nothing.
func agentSteps(root string) []nextJSON {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(actor.EnvTool)), actor.None) {
		return nil
	}
	setup := extraCommands["agent"] != nil
	a := actor.Detect()
	if a == nil {
		why := "if you are an AI agent: how to work with R3V; set R3V_ACTOR to your tool's name so the versions you make say an agent made them"
		if setup {
			why = "if you are an AI agent: how to work with R3V, and the one-time `r3v agent setup`"
		}
		return []nextJSON{{"r3v help agents", why}}
	}
	if setup && a.Tool == agentsetup.ClaudeCodeTool && agentsetup.ToolOf(root) == "" {
		return []nextJSON{{"r3v agent setup",
			"set Claude Code up in this folder, once: R3V_ACTOR in its settings (the versions you make say an agent made them), its hooks, the agents' guide in AGENTS.md"}}
	}
	return nil
}

// notConnected is the warning of a project in no team: what is committed
// stays on this computer.
func notConnected() warningJSON {
	return warningJSON{Code: "not_connected", Reason: "this project is not connected to a team: its versions are on this computer only, not shared"}
}

// connected says whether r is in a team.
func connected(r *project.Repo) bool { return r.Config.Remote != nil }

// printNext prints the steps, one command a line.
func printNext(title string, steps []nextJSON) {
	if len(steps) == 0 {
		return
	}
	fmt.Println(title)
	for _, s := range steps {
		fmt.Printf("  %s\n      %s\n", s.Command, s.Why)
	}
}
