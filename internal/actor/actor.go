// Package actor finds out whether an AI agent runs R3V, so that the
// versions it makes say so (manifest.Actor; see docs/design/agent-identity.md).
// In order: R3V_ACTOR, the copy's own identity (.r3v/agent.json, written by
// `r3v agent setup`), the variables agent tools set themselves; else a
// person.
package actor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nonlabhq/r3v/internal/manifest"
	"github.com/nonlabhq/r3v/internal/store"
)

// The variables R3V reads. `r3v agent setup` puts R3V_ACTOR in Claude
// Code's settings; the others are optional.
const (
	EnvTool    = "R3V_ACTOR"         // the tool's name ("claude-code"); "none": a person
	EnvModel   = "R3V_ACTOR_MODEL"   // the model it runs
	EnvSession = "R3V_ACTOR_SESSION" // the tool's session
)

// None is R3V_ACTOR's value saying that no agent runs R3V, whatever
// the copy or the tools say.
const None = "none"

// Where an actor was found (Resolve).
const (
	FromVariable = "R3V_ACTOR"  // the environment's R3V_ACTOR
	FromCopy     = "agent.json" // the copy's identity
	FromTool     = "detected"   // a variable the tool sets itself
)

// IdentityFile is a copy's agent identity, in its own .r3v: this copy's
// only (a workspace has its own), never tracked and never shared.
const IdentityFile = "agent.json"

// Identity is the agent tool a copy is set up for (r3v agent setup):
// what versions committed in it say when R3V_ACTOR doesn't.
type Identity struct {
	Tool  string `json:"tool"`
	Model string `json:"model,omitempty"`
}

// ReadIdentity is the identity in the .r3v folder dir, nil when there is
// none (or none R3V can read: the copy is then a person's).
func ReadIdentity(dir string) *Identity {
	if dir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, IdentityFile))
	if err != nil {
		return nil
	}
	var id Identity
	if json.Unmarshal(data, &id) != nil {
		return nil
	}
	id.Tool, id.Model = clean(id.Tool), clean(id.Model)
	if id.Tool == "" || strings.EqualFold(id.Tool, None) {
		return nil
	}
	return &id
}

// WriteIdentity keeps id in the .r3v folder dir; changed: it wasn't
// there as it is.
func WriteIdentity(dir string, id Identity) (changed bool, err error) {
	if old := ReadIdentity(dir); old != nil && *old == id {
		return false, nil
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return false, err
	}
	return true, store.WriteAtomic(filepath.Join(dir, IdentityFile), strings.NewReader(string(data)+"\n"))
}

// toolName is what a tool's name may be: short, lower case, no spaces
// (it is shown in lists and typed in R3V_ACTOR).
var toolName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)

// ToolName is name as a tool's name (lower case), ok false when it can't
// be one.
func ToolName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	return name, toolName.MatchString(name) && name != None
}

// Detect is the agent running this process in the copy whose .r3v is dir
// ("": no copy), nil for a person.
func Detect(dir string) *manifest.Actor {
	a, _ := Resolve(os.Getenv, ReadIdentity(dir))
	return a
}

// FromEnv is the agent the variables read through getenv say, with no
// copy's identity.
func FromEnv(getenv func(string) string) *manifest.Actor {
	a, _ := Resolve(getenv, nil)
	return a
}

// Resolve is the agent running R3V and where it was found: R3V_ACTOR
// ("none": a person, whatever else says), else the copy's identity id,
// else a tool's own marker (Announced). A marker of another tool than
// id's wins over id: that tool runs here now, the copy was set up for
// another. The model is R3V_ACTOR_MODEL's, else the tool's own variable's
// (Pi's PI_MODEL: what runs now), else id's when it is id's tool; the
// session R3V_ACTOR_SESSION's, else the tool's own.
func Resolve(getenv func(string) string, id *Identity) (a *manifest.Actor, from string) {
	tool := clean(getenv(EnvTool))
	from = FromVariable
	if strings.EqualFold(tool, None) {
		return nil, ""
	}
	if tool == "" {
		announced := Announced(getenv)
		switch {
		case id != nil && (announced == "" || announced == id.Tool):
			tool, from = id.Tool, FromCopy
		case announced != "":
			tool, from = announced, FromTool
		default:
			return nil, ""
		}
	}
	a = &manifest.Actor{Kind: manifest.ActorAgent, Tool: tool,
		Model: clean(getenv(EnvModel)), Session: clean(getenv(EnvSession))}
	if m := markerOf(getenv, tool); m != nil { // (the tool's own, when it runs here)
		if a.Model == "" && m.ModelVar != "" {
			a.Model = clean(getenv(m.ModelVar))
		}
		if a.Session == "" && m.SessionVar != "" {
			a.Session = clean(getenv(m.SessionVar))
		}
	}
	if a.Model == "" && id != nil && id.Tool == tool {
		a.Model = id.Model
	}
	return a, from
}

// marker is a variable an agent tool sets in the commands it runs, the
// value saying it ("": any but empty), and the tool's own variables for
// its model and session, if it has them. Only ones documented by the tool
// or read in its source: docs/design/agent-identity.md has where. A tool
// started from another inherits that one's marker: the more specific come
// first (Kilo Code sets opencode's too), Claude Code's last.
type marker struct {
	Var, Value, Tool     string
	ModelVar, SessionVar string
}

var markers = []marker{
	{Var: "KILO_PID", Tool: "kilo-code"},
	{Var: "CODEX_THREAD_ID", Tool: "codex", SessionVar: "CODEX_THREAD_ID"},
	{Var: "OPENCODE", Value: "1", Tool: "opencode", SessionVar: "OPENCODE_SESSION_ID"},
	{Var: "GEMINI_CLI", Value: "1", Tool: "gemini-cli"},
	{Var: "QWEN_CODE", Value: "1", Tool: "qwen-code"},
	{Var: "PI_CODING_AGENT", Value: "true", Tool: "pi", ModelVar: "PI_MODEL", SessionVar: "PI_SESSION_ID"},
	{Var: "CURSOR_AGENT", Value: "1", Tool: "cursor"},
	{Var: "CLINE_ACTIVE", Value: "true", Tool: "cline"},  // (in the terminals it opens)
	{Var: "ROO_ACTIVE", Value: "true", Tool: "roo-code"}, // (in VS Code terminals, not its inline runner)
	{Var: "COPILOT_AGENT", Value: "1", Tool: "copilot"},  // (VS Code's agent mode)
	{Var: "GOOSE_TERMINAL", Value: "1", Tool: "goose"},
	{Var: "CLAUDECODE", Value: "1", Tool: "claude-code", SessionVar: "CLAUDE_CODE_SESSION_ID"},
}

// Markers are the tools' variables R3V reads (tests clear them).
func Markers() []string {
	var out []string
	for _, m := range markers {
		out = append(out, m.Var)
		for _, v := range []string{m.ModelVar, m.SessionVar} {
			if v != "" && v != m.Var {
				out = append(out, v)
			}
		}
	}
	return out
}

// Announced is the tool among those that mark the commands they run, ""
// when none does.
func Announced(getenv func(string) string) string {
	for _, m := range markers {
		if m.set(getenv) {
			return m.Tool
		}
	}
	return ""
}

func (m marker) set(getenv func(string) string) bool {
	v := strings.TrimSpace(getenv(m.Var))
	return v != "" && (m.Value == "" || v == m.Value)
}

// markerOf is tool's marker when it is set, nil otherwise.
func markerOf(getenv func(string) string, tool string) *marker {
	for i, m := range markers {
		if m.Tool == tool && m.set(getenv) {
			return &markers[i]
		}
	}
	return nil
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
