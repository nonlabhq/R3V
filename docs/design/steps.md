# Steps

Status: first version built, in every build since 0.3 (`project.Steps`):
on this computer only. The second (the steps' records shared with their version) is to
come.

An agent works in many small moves: one task can be fifty tool calls. When
the twenty-third breaks a scene, today the only way back is the last
version, and the twenty-two good moves go with it. A **step** is a
snapshot of the project after one move: smaller than a version, made by
itself, there to go back to, compare and review, and it keeps the prompt
that asked for it.

Agent tools have checkpoints of their own, but they keep what the tool
itself edited as text (Claude Code's rewind: its Edit and Write calls).
They don't see a file a shell command or an MCP tool wrote (a Blender
scene, a Live set, an engine's level, a rendered texture), and none of
them knows binary files. Tools built on Git that record agents' sessions
(per tool call, or per commit with the prompts and transcript) work on
text code too: for an asset they can only say the file changed. R3V sees
what changed on disk, whatever changed it, and understands some of those
files, so its steps cover all of it.

## Step, turn, session, version

| | What | Made |
|---|---|---|
| **Step** | the project after one move | by itself after each tool call that changed files; at the end of a task (tools without hooks); by hand with `r3v step` |
| **Turn** | one prompt and the steps it led to | when the person sends the agent a prompt; when a tool without hooks starts a task |
| **Session** | an agent's conversation: its turns | by the agent tool (its session id) |
| **Version** | a piece of work, shared with the team | by a commit |

A session can make several versions, and a version can hold steps of
several sessions (two sessions one after the other in the same copy).
Steps record the version they were made on, so either way they can be
listed under a version or under a session.

## What a step is

A version that is never shared, made as parked changes are
([parking.md](parking.md)): `workingManifest` and `save`, its contents in
the local store, deduplicated by hash. Its parent is the step before in
its session, or the version it was made on. Plus a line in the session's
list, `.r3v/steps/<session>.jsonl`:

```json
{"n": 23, "turn": 4, "version": "<id>", "on": "<id>", "time": "2026-10-12T09:41:03Z", "tool": "mcp__blender__execute_code", "actor": "claude-code", "files": ["scenes/arena.blend"]}
```

and one line per turn, before its steps:

```json
{"turn": 4, "time": "2026-10-12T09:40:51Z", "prompt": "Make the arena lighting warmer and move the spawn points off the walls", "transcript": "C:/Users/…/.claude/projects/…/<session>.jsonl"}
```

- `<session>` is the agent tool's session id (`by-hand` for steps made with
  `r3v step` outside a session). A workspace has its own lists (its own
  `.r3v`), its store shared.
- `on` is the version the step was made on; `files` the paths that changed
  since the step before: the list, not the contents, says what the move
  did.
- `prompt` is the person's prompt as they sent it, after redaction (see
  Privacy). `transcript` is where the agent tool keeps the conversation: a
  pointer, never a copy.

A step is made only when files changed since the step before: a tool call
that only read makes none.

## When they are made

`r3v agent setup` adds two hooks beside the `PreToolUse` one:

- **`UserPromptSubmit`** (`r3v hook prompt`): a new turn, with the prompt.
- **`PostToolUse`** (`r3v hook post-tool`), for every tool (Edit, Write,
  Bash, MCP tools): a shell command or an MCP tool changes files too. A
  sub-agent's tool calls are in its parent's session, as the agent tool
  reports them.

And by hand: `r3v step [-m NOTE]`, for a person or an agent that wants a
point to come back to (a note is a turn of its own, its text the note).

### Tools without hooks

Claude Code is the only tool R3V sets hooks up for. Others (Codex,
opencode, Pi, Gemini CLI, Cursor…) read `AGENTS.md`, whose R3V part
tells them to wrap each task in two commands; Claude Code set up with
`r3v agent setup` is told its hooks do it (the guide is one text for
every tool, so it says which).

- **`r3v step start -m "what I'm about to do"`**: a turn whose `note` is
  the agent's words (redacted, as a prompt is; kept even when prompts
  aren't: they are the agent's, not the person's), then a step (`tool`
  `step start`) when the files changed since the session's step before:
  the person's edits between two tasks, or work done outside one, kept
  apart from the task's.
- **`r3v step end [-m "what I did"]`**: a step of the files as they are
  (`tool` `step end`, `detail` the agent's words), in the start's turn.
  None when nothing changed.

The lines are the hooks' own (a turn: `turn`, `time`, `note`; a step:
the keys above), so the app, `r3v steps` and an older R3V read them
alike. Who is asking is `actor.Resolve`'s: the tool and its session
(`R3V_ACTOR_SESSION`, or the tool's own: Codex's thread, opencode's,
Pi's). A tool that gives no session id goes on in its latest session
while that was written to in the last 4 hours, else starts
`<tool>-<UTC time>`. A person with no agent runs them in `by-hand`.

What is open is kept beside the lists, one file per agent (tool and
session): `.r3v/steps/.open-<tool>.<session>.json` (a hash of them when
the session id isn't a plain name), `{"session",
"open": {"session", "turn", "since", "note"}}` (the latest session, for
a tool without ids, stays after the end). This copy's, read by nothing
but start and end; the lists never depend on it.

The rules, so that an agent told to run them always can:

- **A start while a task is open ends it first**, a step of the files
  now in that task's turn (`detail` says it wasn't ended), rather than
  refusing: an agent that forgot the end, or crashed, loses nothing and
  needn't handle an error. Open longer than **12 hours** (an agent gone
  long ago; the person has likely worked since), it is closed without a
  step: the files go to the new task's first step, kept apart.
- **An end with none open** changes nothing and says so (`ended`: null).
- **Steps off**, or **Claude Code whose hooks make the steps here**:
  both change nothing, ok (`off`, `hooked`), so the same guide works
  everywhere.
- **A commit ends the task first** (`r3v commit`, when an agent runs
  it and has one open; not with Claude Code's hooks making the steps):
  a step of the files then in that task's turn, its `detail` `ended by
  commit: <the commit's title>`, then the version, which lists the
  task's steps (made on its parent before it). `commit --json` gives it
  as `step_ended` (as `step end`'s `ended`). An agent that commits
  without `step end` loses nothing; ending first is still the way for a
  task that doesn't commit. A person's commits, the app's included, end
  nothing.
- Two agents of one tool with no session ids in one copy share an open
  task; each in a workspace of its own (the guide's way) has its own.

Stopped half-way:

| In between | What there is | What comes next |
|---|---|---|
| start: the turn written | a turn with no steps (shown as a note) | nothing open: an end is a no-op, the next start begins anew |
| start: the first step written | the turn and its step | as above |
| start: the open record written | a task open | the end, or the next start, ends it |
| end: the step written, the record not cleared | the step | ending again makes no second step (the files are as at it); the next start ends it, nothing changed |
| commit: the task ended, no version yet (stopped, or the commit refused: a locked file, a conflict) | the step, on the version the files are on, nothing open | the next commit's version lists it, as this one would have |

The open record is written whole or not at all (written aside, then
renamed); one that can't be read counts as nothing open.

Not for a person by themselves in the first version: a person saves in
their tool all the time, and a step per save would be noise. (An "auto
steps" setting can come later.)

### Fast enough to run on every call

The hook runs after every tool call, so it must cost little. The scan is
the one `r3v status` does: the index's size and modified time for files
that didn't change, hashing only those that did. A few thousand files take
well under a second. When a scan takes longer than a budget (500 ms), the
hook still makes the step, and says once (in its output to the agent tool)
that the project is big for steps; projects that size get a setting to
turn steps off, or to keep them for some folders only.

The hooks never stand in the agent's way: any error says nothing and makes
no step.

## Using them

```
r3v steps [--session ID] [--json]   a session's turns and steps (the latest session by default)
r3v step [-m NOTE]                  a step now
r3v step back <n> [--json]          the files as at step n
r3v step diff <a> [<b>]             what changed between two steps (b: the files now)
```

- **Back** makes a step of the files as they are first, so going back is a
  move of its own: it can be undone by going to that step. Then it writes
  the files of step n, as `goto` writes a version's (the same checks: a
  set Live has open is refused, not rewritten). The list keeps every step,
  numbered on.
- `step back` is for the person in the first version: the agents' guide
  doesn't offer it (an agent going back in a copy the person also works in
  could undo the person's moves).
- The app shows the latest session above Your changes, as a thin
  timeline: each turn's prompt, its steps under it (tool, files), Go back
  on a step, Compare two.

## With the rest

- **Commit**: the version is made of the files as they are. Its steps are
  those made on its parent since the commit before (`on`), whatever their
  session; the app shows them under the version, folded.
- **Parked changes, switching, going to a version**: steps stay with the
  version they were made on; a session going on elsewhere goes on in the
  same list.
- **Discard**: the steps stay (going back to a step of discarded work is
  how to take some back).
- **Workspaces**: each its own lists; the app's Agents tab shows each
  workspace's latest session.
- **Storage cleanup and `r3v gc`**: a step's contents are kept while its
  list is (step lists are roots, as parked records are). In the first
  version steps are never in the team's storage, so storage cleanup doesn't
  see them.

## How long they are kept

- A session with steps not yet in a version (made on the version the copy
  is on): always.
- Otherwise 7 days after its last step, then deleted, its contents freed by
  `gc` when nothing else uses them.
- A budget per project (2 GB of contents only steps use): over it, the
  oldest sessions go first; one with steps not yet in a version is never
  cut.

These are this computer's settings (the app's project settings), not
`.r3v.yaml`: they change nothing anyone else sees.

## Privacy

Prompts are kept by default, on this computer only; the project settings
turn it off (steps are kept without prompts then).

- Before a prompt is written it is redacted: R3V connection codes, the
  shapes of common keys and tokens (cloud keys, API tokens, private keys),
  and long random-looking strings are replaced by `[redacted]`.
- Nothing else from the conversation is kept: no answers, no tool output.
  The transcript stays where the agent tool keeps it; the step only points
  at it.
- A note given with `-m` is kept as written, on this computer only.

## Interruptions

A step is written as a version is: contents, then folder lists, then the
record, then the line in the list. Stopped half-way, there is no line:
the step doesn't exist, its contents are leftovers `gc` frees. A turn line
without steps is harmless. `step back` writes files as `goto` does (each
file whole or not at all) after the step of the files as they were:
stopped half-way, the files are partly back, and that step has them all.

| In between | What the app shows | What it offers |
|---|---|---|
| step being made | nothing | — |
| back, files being written | Your changes, as they are | going back again, or to the step before |

## Second version: the steps' records shared with their version

The records are small (prompts, tools, which files, when); the contents
are what is big. So a commit shares its steps' records with the version,
and the contents stay on this computer:

- A teammate sees how a version was made: its turns' prompts and each
  step's tool and files, in the app under the version.
- Going back to a step, or comparing two, needs the contents: on the
  computer that made them, or later for steps whose contents were shared
  too (a per-project choice, with a size limit per step).
- Prompts shared with the team are redacted again at commit, and the
  person sees them before they go (they can leave prompts out of a commit).
- This is stored data older R3V doesn't know: a team feature, the records
  beside the version (not inside it, so its id doesn't change), kept by
  storage cleanup as long as the version is.

## Later

- What a step changed inside a file R3V understands (a Live set's tracks,
  a scene's objects), and from that, who made each object: "these three
  tracks came from an agent, from this prompt". Tools built on Git can only
  say this per line of text.
- Steps for a person by themselves (auto steps).
- Undo for R3V's own operations (a merge, an update), as an operation log.

## Open questions

- The budget and the 7 days: right for a game project with big files?
- Codex and other agent tools: which of them give a prompt hook and a
  post-tool hook (steps per tool call, as Claude Code's), beyond `step
  start` and `step end` per task.
- The 12 hours and the 4 hours of tools without hooks: right once tried
  with them?
