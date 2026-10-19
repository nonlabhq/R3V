package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/nonlabhq/r3v/internal/actor"
	"github.com/nonlabhq/r3v/internal/agentsetup"
)

// `r3v agent setup`: a copy set up for an AI agent tool, any tool (field
// tests: agents in tools other than Claude Code made versions that said
// nothing of them). In every build: what it writes, every R3V reads.
func init() {
	extraCommands["agent"] = cmdAgent
	jsonCommands["agent"] = true
}

const agentSetupUsage = "usage: r3v agent setup [--tool NAME] [--model MODEL]"

// agentIdentityJSON is the identity a copy keeps (.r3v/agent.json).
type agentIdentityJSON struct {
	Tool  string `json:"tool"`
	Model string `json:"model,omitempty"`
	File  string `json:"file"` // where it is kept, relative to the folder
}

func cmdAgent(args []string) error {
	if len(args) == 0 || args[0] != "setup" {
		return usageError(agentSetupUsage)
	}
	var tool, model string
	rest := args[1:]
	for len(rest) > 0 {
		name, value, inline := strings.Cut(rest[0], "=")
		name = "--" + strings.TrimLeft(name, "-")
		if (name != "--tool" && name != "--model") || (!inline && len(rest) < 2) {
			return usageError(agentSetupUsage)
		}
		if inline {
			rest = rest[1:]
		} else {
			value, rest = rest[1], rest[2:]
		}
		if name == "--tool" {
			tool = value
		} else {
			model = strings.TrimSpace(value)
		}
	}
	if tool == "" { // (the one R3V_ACTOR, or the tool's own variables, say)
		if t := strings.TrimSpace(os.Getenv(actor.EnvTool)); t != "" && !strings.EqualFold(t, actor.None) {
			tool = t
		} else {
			tool = actor.Announced(os.Getenv)
		}
		if tool == "" {
			return usageError("r3v agent setup: name your agent tool (R3V can't tell it): --tool NAME, such as claude-code, codex, opencode, gemini-cli, cursor, pi")
		}
	}
	tool, ok := actor.ToolName(tool)
	if !ok {
		return usageError("--tool %q: an agent tool's name (lower case, no spaces), such as claude-code, codex or opencode", tool)
	}
	if strings.ContainsAny(model, "\r\n") || len(model) > 100 {
		return usageError("--model: one short line, the model's name")
	}
	r, err := openAgent(".")
	if err != nil {
		return err
	}
	steps := false
	if tool == agentsetup.ClaudeCodeTool {
		r.RefreshAgentTeamLocks() // (for the hook, which never asks the team)
		steps = r.StepsOn()
	}
	// The guide: in the main copy, committed once (a workspace has it in its files).
	changed, id, err := agentsetup.Setup(r.Root, r.Dir, agentsetup.Options{Tool: tool, Model: model,
		Hook: hookCommand(), Guide: !r.IsWorkspace(), Steps: steps})
	if err != nil {
		return err
	}
	changed = nonNil(changed)
	identity := agentIdentityJSON{Tool: id.Tool, Model: id.Model, File: ".r3v/" + actor.IdentityFile}
	result("agent", map[string]any{"tool": tool, "model": id.Model, "folder": r.Root, "changed": changed,
		"files_written": changed, "identity": identity}, func() {
		for _, c := range changed {
			fmt.Println("wrote " + c)
		}
		who := tool
		if id.Model != "" {
			who += " (" + id.Model + ")"
		}
		if len(changed) == 0 {
			fmt.Printf("already set up for %s\n", who)
		} else {
			fmt.Printf("set up for %s: versions committed in this folder say this agent made them (the person's own: r3v commit --as person, or the R3V app)\n", who)
		}
		if t := strings.TrimSpace(os.Getenv(actor.EnvTool)); t != "" && !strings.EqualFold(t, tool) {
			fmt.Printf("note: R3V_ACTOR=%s here wins over it\n", t)
		}
	})
	return nil
}
