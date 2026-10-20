# Agent identity: who made a version

Status: built (`feat/agent-setup-any`, narrowed on `feat/agent-worktrees`),
every build.

## The principle

**Generic mechanisms first; tool-specific code only for the three
mainstream tools, and only passive.** Agent tools change fast (new ones
every month, variables renamed, features dropped and put back), so every
line that knows one tool is code to keep up with it. Any tool works with
what is generic:

- the guide in `AGENTS.md` (most tools read it) and `r3v help agents`;
- `--json` results, stable error codes and `next`;
- `r3v agent setup --tool ANY [--model MODEL]` (the copy's identity) and
  `R3V_ACTOR`, `R3V_ACTOR_MODEL`, `R3V_ACTOR_SESSION`;
- `r3v step start` / `step end` for steps.

Claude Code, Codex and Cursor also get code of their own, which only
reads (who it is, its model and session) or records (steps from Claude
Code's hooks). R3V never replaces or blocks a tool's own behaviour (its
worktrees, its shell commands): where a tool does something R3V can't
see, R3V says so in its own commands (`git_worktree`) and the guide says
what to do instead.

The record is the one versions already have (`manifest.Actor`: `kind`,
`tool`, `model`, `session`, all `omitempty`): no new format, no team
feature. How the CLI fills it in is `internal/actor`.

## Order

For each command (`actor.Resolve`, `actor.ResolveIn` for a copy):

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
   another (a person who set a folder up for Codex and later opened
   Claude Code in it).
4. **`AI_AGENT`**, when none of the three's markers is set: its value as
   the tool's name when it can be one (`actor.ToolName`), else `agent`.
   It doesn't win over 2: its values follow no agreed form.
5. Else a person: no actor.

The model: `R3V_ACTOR_MODEL`, else the one the tool's hooks recorded for
the session, else the copy's when it is the same tool. The session:
`R3V_ACTOR_SESSION`, else the tool's own (Codex's thread, Claude Code's
session id), read only when that tool's marker is set.

Claude Code has no variable for its model, but its `SessionStart` hook
is given `model` and `PostModelSwitch` `to_model` (`r3v agent setup`
installs both: `r3v hook session-start`, `r3v hook model-switch`; they
only read and exit 0). They keep it in the copy's
`.r3v/agent-sessions/<session id>.json` (`tool`, `model`, `updated`; this
copy's only, never tracked or shared, dropped after a month), and a
command whose session (`CLAUDE_CODE_SESSION_ID`, or `R3V_ACTOR_SESSION`)
has one says that model.

## The person in the same folder

A folder set up for an agent is often the person's too. Their versions
must stay theirs:

- **The app** never sets an actor (`Repo.Actor` stays nil in
  `desktop`): its versions are the person's, whatever folder, whatever
  environment it was started from. Tested (`desktop/agentlocks_test.go`).
- **The CLI in the person's own terminal**: R3V can't tell that terminal
  from an agent tool that sets no variable (the very case agent.json is
  for), so agent.json counts there too. The person says it: `--as
  person` on any command, or `R3V_ACTOR=none` in their shell's profile.
  `r3v agent setup` says so when it sets a folder up.
- Considered and left out: telling by whether stdin is a terminal. Agents
  that run commands in a real terminal (Cursor, VS Code's agent mode)
  would then be taken for the person; a guess, so not done.

## What the three set

Read in each tool's source or official documentation, 2026-10-19. A
marker is a hint: a command can unset or change it, and a tool started
from another inherits that one's markers, so Claude Code's is checked
last.

| Tool (R3V's name) | Marker | Session | Source |
|---|---|---|---|
| Codex CLI (`codex`) | `CODEX_THREAD_ID`, every command while a thread exists | `CODEX_THREAD_ID` | openai/codex `codex-rs/core/src/exec_env.rs` |
| Cursor (`cursor`) | `CURSOR_AGENT=1`, agent shells, IDE and `cursor-agent` | - | Cursor staff on forum.cursor.com (no official docs) |
| Claude Code (`claude-code`) | `CLAUDECODE=1`, Bash and PowerShell tools, hooks | `CLAUDE_CODE_SESSION_ID` | code.claude.com/docs/en/env-vars |

Others (opencode, Pi, Gemini CLI, Qwen Code, Kilo Code, Cline, Roo Code,
VS Code's agent mode, Goose) set markers of their own too, and some
their model; R3V before 0.3 read them, from 0.3 it doesn't: they are set
up with `agent setup --tool`, `R3V_ACTOR`, or found by `AI_AGENT`, which
VS Code's agent mode, Claude Code, opencode and Pi set
(`github_copilot_vscode_agent`, `claude-code_<version>_agent`,
`opencode`, `pi`). Not markers: `TERM_PROGRAM`, `CI`, the bare `AGENT=1`.

## Next steps R3V suggests

`r3v init` (and `status` on a project on no team) list what to run next
(`cli/next.go`): with no actor, `r3v help agents` (which says to run
`agent setup --tool NAME`); a tool found by its marker or `AI_AGENT`
while the copy has no identity, `r3v agent setup --tool <that tool>`
(`--tool NAME` when `AI_AGENT` names none); Claude Code while its hooks
aren't there, `r3v agent setup`. A person (`R3V_ACTOR=none`, `--as
person`) gets none of these.
