// Package actor finds out from the environment whether an AI agent runs
// R3V, so that the versions it makes say so (manifest.Actor; see
// docs/design/agent-workspaces.md).
package actor

import (
	"os"
	"strings"

	"github.com/nonlabhq/r3v/internal/manifest"
)

// The variables R3V reads. `r3v agent setup` puts R3V_ACTOR in the agent
// tool's own settings; the others are optional.
const (
	EnvTool    = "R3V_ACTOR"         // the tool's name ("claude-code"); "none": a person
	EnvModel   = "R3V_ACTOR_MODEL"   // the model it runs
	EnvSession = "R3V_ACTOR_SESSION" // the tool's session
)

// None is R3V_ACTOR's value saying that no agent runs R3V, whatever
// the tools announce.
const None = "none"

// Detect is the agent running this process, nil for a person.
func Detect() *manifest.Actor { return FromEnv(os.Getenv) }

// FromEnv is Detect reading the variables through getenv.
func FromEnv(getenv func(string) string) *manifest.Actor {
	tool := clean(getenv(EnvTool))
	if tool == "" {
		tool = announced(getenv)
	}
	if tool == "" || strings.EqualFold(tool, None) {
		return nil
	}
	return &manifest.Actor{Kind: manifest.ActorAgent, Tool: tool,
		Model: clean(getenv(EnvModel)), Session: clean(getenv(EnvSession))}
}

// announced is the tool among those that mark the commands they run.
func announced(getenv func(string) string) string {
	if getenv("CLAUDECODE") == "1" { // Claude Code, in every command it runs
		return "claude-code"
	}
	return ""
}

// clean keeps a value to one short line: it is shown in lists.
func clean(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if r := []rune(s); len(r) > 100 {
		s = string(r[:100])
	}
	return s
}
