# Steps

Status: design, first version on this computer only (not built yet).

An agent works in many small moves: one task can be fifty tool calls. When
the twenty-third breaks a scene, today the only way back is the last
version, and the twenty-two good moves go with it. A **step** is a
snapshot of the project after one move: smaller than a version, made by
itself, there to go back to, compare and review.

Agent tools have checkpoints of their own, but they keep what the tool
itself edited as text (Claude Code's rewind: its Edit and Write calls).
They don't see a file a shell command or an MCP tool wrote (a Blender
scene, a Live set, an engine's level, a rendered texture), and none of
them knows binary files. R3V sees what changed on disk, whatever changed
it, so its steps cover all of it.

## Step and version

| | Step | Version |
|---|---|---|
| What | the project after one move | a piece of work, made of steps |
| Made | by itself after each tool call that changed files; by hand with `r3v step` | by a commit |
| Where | this computer only (`.r3v`) | shared with the team |
| Kept | while the work is open, then a while after its commit | for good |
| For | going back, comparing, seeing what an agent did | everything else |

The first version keeps steps on this computer: nothing the team stores
changes, so no team feature and no new stored format. Sharing steps with
their version (to review an agent's work move by move) comes later, once
real use says how big they get and which are worth sending.

## What a step is

A version that is never shared, made as parked changes are
([parking.md](parking.md)): `workingManifest` and `save`, its contents in
the local store, deduplicated by hash. Its parent is the step before, or
the version the work started on. Plus a line in the work's step list,
`.r3v/steps/<work>.jsonl`:

```json
{"n": 23, "version": "<id>", "base": "<id>", "time": "2026-10-12T09:41:03Z", "tool": "mcp__blender__execute_code", "actor": "claude-code", "files": ["scenes/arena.blend"], "note": ""}
```

- `<work>` is the version the work started on: committing starts a new
  list. A workspace has its own lists (its own `.r3v`), its store shared.
- `files` are the paths that changed since the step before: the list, not
  the contents, says what the move did.
- `tool` and `note` are all that is said about the move: the tool's name,
  and a note given with `r3v step -m`. No prompt, no transcript (they hold
  keys and private text; see Privacy).

A step is made only when files changed since the step before: a tool call
that only read makes none.

## When steps are made

- **After an agent's tool call**: `r3v agent setup` adds a `PostToolUse`
  hook beside the `PreToolUse` one (`r3v hook post-tool`), for every tool
  (Edit, Write, Bash, MCP tools): a shell command or an MCP tool changes
  files too.
- **By hand**: `r3v step [-m NOTE]`, for a person or an agent that wants a
  point to come back to.
- Not for a person by itself in the first version: a person saves in their
  tool all the time, and steps per save would be noise. (An "auto steps"
  setting can come later.)

### Fast enough to run on every call

The hook runs after every tool call, so it must cost little. The scan is
the one `r3v status` does: the index's size and modified time for files
that didn't change, hashing only those that did. A few thousand files take
well under a second. When a scan takes longer than a budget (500 ms), the
hook still makes the step, and says once (in its output to the agent tool)
that the project is big for steps; projects that size get a setting to
turn steps off, or to keep them for some folders only.

The hook never stands in the agent's way: any error says nothing and
makes no step.

## Using them

```
r3v steps [--json]              the open work's steps: n, time, tool, files changed
r3v step [-m NOTE]              a step now
r3v step back <n> [--json]      the files as at step n
r3v step diff <a> [<b>]         what changed between two steps (b: the files now)
```

- **Back** makes a step of the files as they are first, so going back is a
  move of its own: it can be undone by going to that step. Then it writes
  the files of step n, as `goto` writes a version's (the same checks: a
  set Live has open is refused, not rewritten).
- Back changes only files; the list keeps every step, numbered on.
- The app shows the open work's steps above Your changes, as a thin
  timeline: each step's tool and files, Go back on one, Compare two.

## With the rest

- **Commit**: the version is made of the files as they are; the open list
  is closed, recorded as the version's steps (`.r3v/steps/<work>.jsonl`
  gets `"committed": "<version id>"`), and a new list starts.
- **Parked changes, switching, going to a version**: the open list stays
  with its work: parked with it, back with it.
- **Discard**: the open list is closed as discarded, kept like a committed
  one (going back to a step of discarded work is how to take some back).
- **Workspaces**: each its own lists; the app's Agents tab shows each
  workspace's step count.
- **Storage cleanup and `r3v gc`**: a step's contents are kept while its
  list is (step lists are roots, as parked records are). Steps are never
  in the team's storage, so storage cleanup doesn't see them.

## How long they are kept

- The open list: always.
- A closed list: 7 days after its commit or discard, then deleted, its
  contents freed by `gc` when nothing else uses them.
- A budget per project (2 GB of contents only steps use): over it, the
  oldest closed lists go first; the open list is never cut.

These are this computer's settings (the app's project settings), not
`.r3v.yaml`: they change nothing anyone else sees.

## Privacy

Nothing from the agent's conversation is kept: the tool's name, the paths
it changed, the note. A note given with `-m` is the agent's or person's
own text, kept only on this computer.

## Interruptions

A step is written as a version is: contents, then folder lists, then the
record, then the line in the list. Stopped half-way, there is no line:
the step doesn't exist, its contents are leftovers `gc` frees. `step back`
writes files as `goto` does (each file whole or not at all) after the
step of the files as they were: stopped half-way, the files are partly
back, and that step has them all.

| In between | What the app shows | What it offers |
|---|---|---|
| step being made | nothing | — |
| back, files being written | Your changes, as they are | going back again, or to the step before |

## Not in the first version

- Sharing steps with their version, and reviewing an agent's work step by
  step from another computer.
- Steps for a person by themselves (auto steps).
- What a step changed inside a file R3V understands (a Live set's tracks,
  a scene's objects): the version diff can show it for any two steps
  later.

## Open questions

- The budget and the 7 days: right for a game project with big files?
- Should `step back` also be offered to agents in the guide, or only to
  the person (an agent going back on its own can undo the person's moves
  made meanwhile in the same copy)?
