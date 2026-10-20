# Agent identity: who made a version

Status: built (`feat/agent-setup-any`), every build.

Field tests with opencode (GLM) and Pi (Kimi): neither agent ran
`r3v agent setup`, which then knew Claude Code only, and R3V read only
Claude Code's variable. Their versions said nothing of them. Now any tool
can be set up, and R3V tells more tools apart by the variables they set.

The record is the one versions already have (`manifest.Actor`: `kind`,
`tool`, `model`, `session`, all `omitempty`): no new format, no team
feature. What changed is how the CLI fills it in (`internal/actor`).

## Order

For each command (`actor.Resolve`):

1. **`R3V_ACTOR`** in the environment: the tool's name, or `none` for a
   person (then nothing below counts). `R3V_ACTOR_MODEL` and
   `R3V_ACTOR_SESSION` go with it.
2. **The copy's identity**, `.r3v/agent.json` `{"tool", "model"}`,
   written by `r3v agent setup --tool NAME [--model MODEL]` (or
   `workspace add --agent NAME`). It is in the copy's own `.r3v`, as a
   workspace's `HEAD` and index are: this folder's only, never tracked,
   never sent to the team.
3. **A tool's own marker** (table below). One of another tool than the
   copy's wins over 2: that tool runs here now; the copy was set up for
   another (a person who set a folder up for opencode and later opened
   Claude Code in it).
4. Else a person: no actor.

The model: `R3V_ACTOR_MODEL`, else the tool's own variable (only Pi has
one: `PI_MODEL`, what runs now), else the one the tool's hooks recorded
for the session (`actor.ResolveIn`), else the copy's when it is the same
tool.

Claude Code has no variable for its model, but its `SessionStart` hook
is given `model` and `PostModelSwitch` `to_model` (`r3v agent setup`
installs both: `r3v hook session-start`, `r3v hook model-switch`). They
keep it in the copy's `.r3v/agent-sessions/<session id>.json` (`tool`,
`model`, `updated`; this copy's only, never tracked or shared, dropped
after a month), and a command whose session (`CLAUDE_CODE_SESSION_ID`,
or `R3V_ACTOR_SESSION`) has one says that model. The record is the
version's `actor.model`, as before: no format change. The session: `R3V_ACTOR_SESSION`, else the tool's own (Codex's
thread, opencode's, Pi's, Claude Code's session id), read only when that
tool's marker is set.

## The person in the same folder

A folder set up for an agent is often the person's too. Their versions
must stay theirs:

- **The app** never sets an actor (`Repo.Actor` stays nil in
  `desktop`): its versions are the person's, whatever folder, whatever
  environment it was started from. Unchanged, and tested
  (`desktop/agentlocks_test.go`).
- **The CLI in the person's own terminal**: R3V can't tell that terminal
  from an agent tool that sets no variable (the very case agent.json is
  for), so agent.json counts there too. The person says it: `--as
  person` on any command, or `R3V_ACTOR=none` in their shell's profile.
  `r3v agent setup` says so when it sets a folder up.
- Considered and left out: telling by whether stdin is a terminal. Agents
  that run commands in a real terminal (Cursor, VS Code's agent mode,
  Cline) would then be taken for the person; a guess, so not done.

## What tools set

Read in each tool's source or official documentation, 2026-10-19 (the
default branch then). A marker is a hint: a command can unset or
change it, and a tool started from another inherits that one's markers,
so the more specific are checked first (Kilo Code sets opencode's;
Claude Code's comes last, as a tool started from Claude Code inherits
`CLAUDECODE`).

| Tool (R3V's name) | Marker R3V reads | Set in | Model, session | Source |
|---|---|---|---|---|
| Kilo Code (`kilo-code`) | `KILO_PID` | every command (set on its own process at start) | - | Kilo-Org/kilocode `packages/opencode/src/index.ts` |
| Codex CLI (`codex`) | `CODEX_THREAD_ID` | every command while a thread exists, every OS | session: `CODEX_THREAD_ID` | openai/codex `codex-rs/core/src/exec_env.rs`, `codex-rs/protocol/src/shell_environment.rs` |
| opencode (`opencode`) | `OPENCODE=1` | its shell tool | session: `OPENCODE_SESSION_ID` | anomalyco/opencode `packages/core/src/tool/plugin/shell.ts` |
| Gemini CLI (`gemini-cli`) | `GEMINI_CLI=1` | its shell execution service ("so scripts can tell they run inside Gemini CLI") | - | google-gemini/gemini-cli `packages/core/src/services/shellExecutionService.ts` |
| Qwen Code (`qwen-code`) | `QWEN_CODE=1` | its shell execution service | - | QwenLM/qwen-code `packages/core/src/services/shellExecutionService.ts` |
| Pi (`pi`) | `PI_CODING_AGENT=true` | every command (set on its process at start, CLI and RPC) | model: `PI_MODEL`; session: `PI_SESSION_ID` | earendil-works/pi (was badlogic/pi-mono) `packages/coding-agent/src/cli/setup.ts`, `docs/environment-variables.md` |
| Cursor (`cursor`) | `CURSOR_AGENT=1` | agent shells, IDE and `cursor-agent` (once dropped from the CLI by mistake, Sept 2025, and put back) | - | Cursor staff on forum.cursor.com (no official docs): threads 132427, 152451 |
| Cline (`cline`) | `CLINE_ACTIVE=true` | the VS Code terminals Cline opens | - | cline/cline `apps/vscode/src/hosts/vscode/terminal/VscodeTerminalRegistry.ts` |
| Roo Code (`roo-code`) | `ROO_ACTIVE=true` | VS Code terminal mode only (not its inline runner) | - | RooCodeInc/Roo-Code `src/integrations/terminal/Terminal.ts` |
| GitHub Copilot, VS Code agent mode (`copilot`) | `COPILOT_AGENT=1` | the agent's tool terminal | - | microsoft/vscode `src/vs/workbench/contrib/terminalContrib/chatAgentTools/browser/toolTerminalCreator.ts`, `src/vs/platform/chat/common/aiAgentEnv.ts` |
| Goose (`goose`) | `GOOSE_TERMINAL=1` | its shell commands | - | aaif-goose/goose `documentation/docs/guides/environment-variables.md` |
| Claude Code (`claude-code`) | `CLAUDECODE=1` | Bash and PowerShell tools, hooks (also stdio MCP servers, and IDE extensions' terminals) | session: `CLAUDE_CODE_SESSION_ID` | code.claude.com/docs/en/env-vars |

Nothing reliable found, so not read (set them up with `agent setup
--tool`, or `R3V_ACTOR`):

- **Aider**: runs commands with the environment as it is
  (Aider-AI/aider `aider/run_cmd.py`).
- **Windsurf (Cascade)**: nothing in its docs.
- **GitHub Copilot CLI** and the **Copilot coding agent** (Actions): no
  marker documented (`COPILOT_MODEL` and the like are the user's
  settings; `GITHUB_ACTIONS` is any Actions run).
- **Amp**: only third-party detectors' claims (`AMP_CURRENT_THREAD_ID`,
  and that it sets `CLAUDECODE` too); unconfirmed.

Not markers, though set by some agents: `TERM_PROGRAM` (any editor's
terminal), `CI`, `PAGER`, `GIT_PAGER`, the bare `AGENT=1` (opencode and
Kilo, naming neither).

`AI_AGENT` is becoming a shared convention (VS Code's agent mode,
Claude Code, opencode and Pi set it: `github_copilot_vscode_agent`,
`claude-code_<version>_agent`, `opencode`, `pi`), but its values aren't
specified anywhere; R3V reads the tools' own markers instead, which cover
those tools. Worth revisiting once it is written down.

## Next steps R3V suggests

`r3v init` (and `status` on a project on no team) list what to run next
(`cli/next.go`): with no actor, `r3v help agents` (which says to run
`agent setup --tool NAME`); a tool found by its marker while the copy has
no identity, `r3v agent setup --tool <that tool>`; Claude Code while its
hooks aren't there, `r3v agent setup`. A person (`R3V_ACTOR=none`, `--as
person`) gets none of these.
