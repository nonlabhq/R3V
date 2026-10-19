# Agent field test

How to check, by hand, that an AI agent finds its own way with R3V when told
almost nothing: connects the project to the team, sets itself up, and leaves
its identity and steps in the history. Run it after changes to `r3v -h`,
`r3v help agents`, `r3v init`'s next steps, `agent setup` or steps; with
several tools (Claude Code, Codex, opencode, Pi, Gemini CLI, Cursor…) and
models.

## Before

- An R3V build with the changes, installed (the app, and `r3v` on PATH from
  it). `r3v version` says which.
- A team for the test: an R3V Cloud team you're an owner of, or a
  self-hosted one this computer has joined. Not a team with real work.
- An empty folder per tool, e.g. `D:\R3V-field\<tool>`.
- The tool's own setup done (signed in, model picked). Note the tool, its
  version and the model.

## The prompt

Give the agent only this, in the empty folder (fill in the team's name):

> Make a small game prototype here: an HTML5 canvas game, a
> player that moves with the arrow keys and collects coins. Keep this
> project's versions with R3V, on the team "<TEAM>". R3V is installed: see
> `r3v -h`. Work in a few steps, and keep the versions small.

Don't mention `r3v help agents`, `agent setup`, `--team` or AGENTS.md.
Answer the agent's questions plainly; if it asks for permission (signing in,
a connection code), act as the person would.

## What to check afterwards

In the folder, as yourself: `r3v status --json --as person`, `r3v log --json`;
and in the app, the team's home and the project.

| # | Check | Where | Pass |
|---|---|---|---|
| 1 | The agent read `r3v help agents` (or the AGENTS.md guide) before its first commit | its transcript | yes |
| 2 | The project is the team's | `status --json`: `connected: true`; the app lists it on the team | yes |
| 3 | No version stayed unshared | `status --json`: `unshared` 0 or absent; no `not_connected` warning | yes |
| 4 | It ran `r3v agent setup` | AGENTS.md has the R3V section; `.r3v/agent.json` names the tool | yes |
| 5 | Its versions say who made them | `log --json`: each agent version's `actor.tool` is the tool (and `model` when known) | yes |
| 6 | Steps are recorded | the app's steps for the project, or `r3v step` list: one per piece of work, closed | yes |
| 7 | It handled R3V's errors and warnings as the docs say | transcript: asked the user on `project_deleted`, `--break`/`--take`, never printed codes | yes |
| 8 | Nothing odd left behind | no `.r3v/lock` while idle, no workspace it forgot, no files a rule should have left out (`looks_built`) | yes |

Note for each tool: pass/fail per row, the R3V version, the commands it ran
first (the first ten), and anything it got wrong or asked about. A failure
in rows 1–6 is something R3V's output should have led it to; fix the output,
not the prompt.

## Variants

- Told only "see `r3v -h`", in a folder that already has an R3V project not
  on a team (checks that `status`/`commit` lead it to connect).
- A second agent in a workspace of the same project at the same time
  (`r3v workspace add`), both changing the same image (locks), then merged by
  the person.
- In a sandbox that can't write the user's settings folder (checks
  `no_write_access` and `R3V_CONFIG_DIR`).
