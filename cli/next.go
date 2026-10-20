package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
// name to each team this computer has joined, else signing in
// to R3V Cloud or a connection code (the person's to enter: it holds the
// storage's keys).
func connectSteps() []nextJSON {
	joined := usableTeams()
	var out []nextJSON
	switch {
	case len(joined) > 0:
		for _, t := range joined {
			out = append(out, nextJSON{"r3v remote --team " + shellArg(t.Name),
				fmt.Sprintf("connect this project to the team %s (this computer has joined it): its versions are shared with the team", t.Name)})
		}
	default:
		if remote.HostedTeams {
			out = append(out, nextJSON{"r3v login",
				"sign in to R3V Cloud (the person, in the browser): the teams they are in then appear in `r3v teams`, and `r3v remote --team NAME` connects this project to one"})
		}
		out = append(out, nextJSON{"r3v remote <connection-code>",
			"or connect it to a self-hosted team's storage with its connection code: the person enters it (it holds the storage keys: never ask for it or print it)"})
	}
	return out
}

// agentSteps is what an AI agent does once in a project: read the guide
// when R3V can't tell an agent runs it; set the copy up for the tool R3V
// tells from its variables while it isn't (versions then say so from
// agent.json, even where the variables don't reach), and Claude Code's
// hooks while they aren't there. A person (R3V_ACTOR=none) gets nothing.
func agentSteps(root string) []nextJSON {
	if byPerson || strings.EqualFold(strings.TrimSpace(os.Getenv(actor.EnvTool)), actor.None) {
		return nil
	}
	a, from := actor.Resolve(os.Getenv, actor.ReadIdentity(filepath.Join(root, ".r3v")))
	if a == nil {
		return []nextJSON{{"r3v help agents",
			"if you are an AI agent: how to work with R3V, and the one-time `r3v agent setup --tool NAME` (NAME: your tool, such as codex, opencode, gemini-cli, cursor, pi) so the versions you make say an agent made them"}}
	}
	switch {
	case a.Tool == agentsetup.ClaudeCodeTool && agentsetup.ToolOf(root) == "":
		return []nextJSON{{"r3v agent setup",
			"set Claude Code up in this folder, once: R3V_ACTOR in its settings (the versions you make say an agent made them), its hooks, the agents' guide in AGENTS.md"}}
	case from == actor.FromTool && a.Tool == actor.GenericTool: // (AI_AGENT, naming no tool R3V can write)
		return []nextJSON{{"r3v agent setup --tool NAME",
			"set this folder up for your tool, once (NAME: its name, such as codex or cursor): the versions you make here say so (add --model with the model you run), the agents' guide in AGENTS.md"}}
	case from == actor.FromTool:
		return []nextJSON{{"r3v agent setup --tool " + a.Tool,
			"set this folder up for " + a.Tool + ", once: the versions you make here say so (add --model with the model you run), the agents' guide in AGENTS.md"}}
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
