# Several agents on one project

Status: being built (Nightly). From "R3V 的機會" (direction 1, case A: one
computer, several agents), with the small things it needs first. Agent
tools: Claude Code first, then Codex.

Several AI agents (and the person) work on one project at once on one
computer: one writes the renderer, one the app's shell, one sorts the
textures. Each needs a copy of its own: editors open one copy of a project
at a time (Live one set per computer, Unity and Godot one editor per
folder), builds write into the folder, and binary files can't be shared
by hunk the way GitButler shares one working folder. So: a **workspace**
per agent, on a branch of its own; merging that settles code **hunk by
hunk**; **locks** on what can't be merged; an **overview** where the
person merges what's done.

## Workspaces

A workspace is another folder with the same project, on this computer:

```
r3v workspace add [<folder>] --branch <name> [--from <version|branch>] [--agent claude-code]
r3v workspace list [--json]
r3v workspace remove <folder>        # when merged; asks (fails) if not
```

- **Where**: unless a folder is given, in one folder beside the project,
  named after it: `Song.workspaces/<branch>` beside `Song/`, so
  workspaces don't spread over the folder the project is in; removed
  with its last workspace. Never inside the project: git, Godot or an
  agent reading `CLAUDE.md` upwards would take a workspace for part of
  it (a workspace has no `.git`: git there would work on the project's).
- **Made from what's here**: the files come from the store on this
  computer, never downloaded again. Copied with block cloning where the
  drive has it (ReFS, Dev Drive: no space taken until changed), else
  copied. Never hard links: tools write files in place, which would
  change every copy.
- **One store**: the workspaces share the main copy's store and versions
  (`.r3v/objects`, `.r3v/snapshots`), as `git worktree` does
  (`Repo.CommonDir`); each has its own `.r3v` for the rest (where it is,
  its index, its parked changes). The main copy keeps the list
  (`workspaces.json` in the common dir); local cleanup keeps what any of
  them needs.
- **Its own branch**: a team branch marked `kind: agent`, `owner` (the
  member it works for), made from the main copy's branch unless said.
  Commits share on it as any commit does (the team's storage backs the
  work up). Agent branches are folded away in the graph; once merged,
  archived (their versions stay).
- What the rules leave out (`node_modules/`, Rust's `target/`, Unity's
  `Library/`) each workspace builds for itself: `npm ci`, a first `cargo
  build`. Each keeps its own `target/` (a shared one makes builds wait
  for each other).

Built (`feat/workspaces`; `internal/project/workspaces.go`, `r3v
workspace` on Nightly):

| Shared: the main copy's `.r3v` (`CommonDir`) | Each copy's own `.r3v` (`Dir`) |
|---|---|
| `objects/` (the store, its `tmp/`) | `config.json`: branch, workspace id, tip (team, project id and name are read from the main copy's) |
| `snapshots/`, `trees/` (the versions) | `HEAD`, `index.bin`, `switching` |
| `chunks/` (chunk lists, by content) | `parked/`, `kept-work` (their versions are in the shared `snapshots/`) |
| `remote-objects` (what only the team's storage has) | `preupload/`, `preuploaded` |
| `workspaces.json` (the registry), `agent-locks/` | `external/` (samples for its own sets), `lock`, `locks-waiting.json`, `team-seen.json`, `workspace-noted.json` |
| | `workspace.json` (a workspace's only): the main copy's `.r3v`, the base branch and the version it started at |

- What is written whole and named by its content is shared; what says
  where a copy is, or what it has under way, is its own.
- Cleanup in any copy keeps every copy's needs: the shared versions
  include every copy's parked sets and kept work (GC keeps their files);
  pruning keeps every copy's parked content; dropping kept work never
  drops what another copy parked or kept; cleanup keeps every copy's early
  uploads listed. It takes every other copy's lock first (not waiting): a
  copy committing has files in the store no version names yet.
- A copy's files stand in for the store for every copy (each used only
  while as it was hashed, and checked again after it is copied): making
  a workspace after the store gave the team's files up downloads nothing.
- Files are put in place by the system's copy (`CopyFileW`: block cloning
  on ReFS and Dev Drive), then given the time they were written (a build
  tool must not take them for old ones).
- A workspace can't be in another copy's folder, or hold one. Connecting
  it to a team or moving it to another is refused (`in_workspace`): it
  follows the main copy. Its branch is made with the existing branch
  calls; marking it `kind: agent` comes with Records.
- Not yet: the app; `--agent` (agent setup); archiving the branch when
  the workspace goes; a workspace whose main copy moved (it says so,
  `workspace_orphaned`: make it again). Older R3V versions don't know
  workspaces: one run in a workspace folder finds no versions there (it
  changes nothing of the main copy's).

## The overview

`r3v workspace list --json` and the app's **Agents** view (in the
project): for each workspace its folder, branch, owner and agent (actor,
below), versions ahead of and behind its base branch, files changed,
locks held, last activity, and whether it has changes not committed.

**Merging** is the person's by default: a button per workspace, the
merge preview and decisions as today, then the branch archived and the
workspace offered for removal. A coordinating agent can merge from the
command line (`r3v merge <branch> --json`).

As built (`feat/agents-view`, Nightly: `desktop/agentworkspaces.go`,
`AgentsView.svelte`): the project's **Agents** tab (a team's project, its
main copy; the tab counts the workspaces). **Merge into <base>** opens the
merge preview of its branch (going to the base branch first when the
project is on another), with the archive offer after as for any branch.
**Open** shows the workspace in the sidebar under its project (the app's
`teams.json` `workspaces`; nothing runs in the background there).
**Remove** asks; refused, it says what's left and offers removing anyway.
**New workspace** makes one in `Song.workspaces` (`Song.workspacesenderer`;
  a folder chosen instead gets "Song (renderer)" in it), set up
for Claude Code by default (`internal/agentsetup`, shared with the CLI;
the hook runs the `bin\r3v.exe` installed with the app).

## Merging code hunk by hunk

Text files (the `code` preset's handlers: `.ts`, `.rs`, `.json`, `.toml`,
`.glsl`…) are merged line by line already; where both sides changed the
same lines, the whole file is one conflict today. Instead, each place
both changed is a conflict of its own:

- `conflicts[]` gains, for a text file, one item per hunk: its key
  (`text:<path>#<n>`), lines, and base, ours and theirs as text.
- Each is decided apart: `ours`, `theirs`, `both` (ours then theirs);
  the file is written once all are.
- Or the agent gives the file as it should be:
  `--resolved <path>=<file>` (any text it made, merged by hand).
- On the command line: `--resolve <key>=<choice>` (repeatable),
  `--decisions <file.json>` (`{"<key>": "<choice>"}`, `--decisions -`
  from stdin). `--strategy` still decides everything else.
- The app's merge decisions go hunk by hunk for text, as track by track
  for sets.
- Lockfiles (`package-lock.json`, `Cargo.lock`) merge as text; when they
  don't, the agents' guide says: take one side, then regenerate (`npm
  install`, `cargo update -w`).

## Locks

One kind of lock to the person: a padlock with who holds it, a person
("Kai") or an agent ("renderer", one of your workspaces). Taken,
freed and broken the same way. Underneath, two scopes:

| | Team locks (built) | Agent locks (new) |
|---|---|---|
| Between | people of the team | the workspaces on this computer |
| Held by | a member | a workspace (and its session) |
| Kept | by R3V Cloud | on this computer, beside the shared store |
| Enforced | by the service, at a share | by the hook before a change, and at commit |
| Until | shared | merged into the base branch (on the base branch: committed) |

What locks itself is the team's choice of kinds (`profile.LockKinds`;
a project's own settings narrow it: team-settings.md), the same for both: files that
can't be merged (`.glb`, textures, `.hdr`, `.exr`, sounds, `.blend`).
**Code never locks**: it merges hunk by hunk. On R3V Cloud with team
locks on, an agent's first change to such a file takes both.

Agent locks:

- **Taken on first touch**: the hook locks a file of a locking kind the
  moment an agent first changes it; refused with who holds it. No agent
  waits for a lock (refused, it moves on), so nothing deadlocks. An agent
  can also lock ahead (`r3v lock <paths…> --json`, all or none; a folder
  covers what's under it; too wide is warned about).
- **The person's changes count**: a file changed and not committed in a
  copy a person works in (no workspace's agent) is held by them.
- **Held until merged** into the base branch: conflicts happen at the
  merge, not the commit. A lock protects work not committed, and work not
  yet merged into the base. On the base branch itself (the main copy's,
  where nothing merges it anywhere) a commit frees it: what's committed
  there is what the workspaces merge from, so there's nothing left to
  protect. Held until merged there, one commit of a scene in the main
  copy stopped every workspace until the lock went stale.
- **Taken over** on purpose: `r3v lock --take <paths…> --reason <why>`
  takes others' locks before they go stale, when their holder is gone;
  the new lock keeps the reason and who held it (`locks` shows both).
  The person's uncommitted changes still refuse it.
- **Two checks**: before a change (the `PreToolUse` hook refuses an
  agent's Edit or Write on a path another holds), and at commit (a commit
  changing such a path is refused: catches a tool's own save, an editor
  driven through MCP).
- **People always win**: the app shows every lock and breaks any; an
  agent's lock never stops a person's commit.
- **Left over** (an agent killed): each hook call is a heartbeat; a lock
  with none for a while is stale, shown so, and the next lock asked over
  it takes it.

As built (`feat/agent-locks`, Nightly: `internal/agentlock`, the commit
check in `project.Snapshot`, `cli/agentlocks.go`):

- The list is `CommonDir()/agent-locks`: one JSON file per lock (named by
  a hash of its path; a claim's also by workspace), written whole then
  renamed; takers take turns on a file lock of the folder. A heartbeat is
  a small file per workspace whose time each hook call sets; **stale**
  after 15 minutes without one.
- What locks: the team's kinds as last read (`agent-locks/team-locks.json`,
  refreshed by `agent setup` and `locks`: the hook never asks the team),
  else every kind but Godot's text scenes (`profile.AgentLockKinds`:
  textures and sounds too, which the team's defaults leave out; models
  are among the defaults now, a team that chose its kinds before keeps
  its list);
  the project's settings narrow them (team-settings.md; `.r3v.yaml`'s
  `file_locks` before 0.1.75); ignored files and files a
  preset merges as text never lock.
- The person's copy is the main copy (the folder of `CommonDir()`): a
  locking file there that differs from its version (size, then the
  index's time, then a hash) is theirs. A person in another workspace
  isn't seen yet.
- An agent's commit takes the locks of the locking files it changes (as a
  first touch would), or is refused (`files_locked_by_agent`) with
  nothing taken. A person's never is.
- Merging branch X (`r3v merge X`) frees X's locks; not a base branch's
  (a workspace merging its base in is catching up).
- A base branch is main, or any registered workspace's `base`
  (`workspaces.json`). After a commit there, the copy's file locks on that
  branch whose files are as committed (or as before, never changed) are
  freed; folder locks stay until unlocked. Stopped between the commit and
  that, the locks stay: the next commit there frees them, or they go
  stale.
- `r3v lock` on a file that never locks (code, ignored files) takes a
  claim instead (`claimed_instead`): a lock there would stop no one, and
  an ineffective record would mislead whoever reads the list.
- The hook denies with Claude Code's `permissionDecision: deny`; a note
  about others' claims goes in `additionalContext`, once per claim and
  workspace. It never answers `allow` (that would skip the person's
  permission prompts), and in Stable it says nothing.

**Claims** (optional, for code): "I'm on `src/renderer/`" warns other
agents (the hook says so once) and blocks nothing: it's about not doing
the same work twice, not about merging.

## Who did it: actor

A version records who made it, besides the member responsible:

```json
"actor": {"kind": "agent", "tool": "claude-code", "model": "…", "session": "…"}
```

- `agent setup` puts `R3V_ACTOR` (and the workspace) in the agent tool's
  own environment settings (Claude Code's `env`), which every command it
  runs inherits; without it, the CLI looks for the variables the tools
  set themselves. Nothing found: no actor (a person).
- `author_id` stays the member: an agent works for someone.
- The history marks an agent's versions and can filter them. Format: an
  `omitempty` field of the version record (format 2 readers ignore it),
  so no team feature.

## Title and details

A version's message: its first line the title, the rest details (Git's
way, no format change). The app shows the title, the details unfolded;
`r3v commit -m <title> -m <details>`.

## Setting agents up

`r3v agent setup [--tool claude-code]` in a workspace: writes the hooks
(`PreToolUse` → `r3v hook pre-tool`, reading the tool call and answering
allow or deny), `R3V_ACTOR`, and the agents' guide where the tool reads
them (`.claude/settings.local.json`, this copy's and never tracked;
`CLAUDE.md` in the main copy, committed); run by `workspace add
--agent claude-code`.

## Stopping half-way

Each step lists its states (docs/development.md#interruptions):

- `workspace add`: the folder written, then registered. Stopped before,
  the folder is a leftover that `workspace add` again finishes or
  `remove` clears; registered, it works. In order: its `.r3v/workspace.json`
  (from then on the folder is known as this project's leftover), its
  config, the files (a switch: `switching` until they are all there, then
  `HEAD`), its branch on the team (stopped after the branch but before
  the workspace knew it, `add` again takes that branch when it is at the
  workspace's version), the registry. The main copy is never written.
- `workspace remove`: checked first, then unregistered, then the folder
  deleted, only if all its work is merged (or the person said so).
  Checked: work not merged, changes, parked; a command at work in it:
  busy. Unregistered, then its parked and kept
  versions dropped (those no other copy holds), then its files, its
  `.r3v` last (so a stop leaves a folder still known as a leftover:
  `remove` again clears it).
- Locks: a lock file is whole or absent (made then renamed); asking for
  several, any refused takes back the ones just made.
- A hunk-by-hunk merge writes nothing until every hunk is decided.

## Lanes

Built in parallel, each lane on its branch in a worktree of its own,
reviewed and merged by Core:

1. **Records** (`feat/agent-records`): actor and its detection, title and
   details, branch `kind`/`owner`. Built: `manifest.Actor` (package
   `actor` reads it; the CLI sets `Repo.Actor`, the app never does),
   `commit -m … -m …`, `log --json` `actor`, `NewBranchWith(name,
   BranchOptions{Agent: true})` and `branch new NAME --agent` (refused
   without branch records, so Stable can't make one).
2. **Workspaces** (`feat/workspaces`): add, list, remove, the shared
   store, block cloning, cleanup over all.
3. **Hunks** (`feat/hunk-merge`): hunk conflicts, `--resolve`,
   `--decisions`, `--resolved`.
4. **Locks** (`feat/agent-locks`): the agent lock list, `lock`/`unlock`,
   first-touch locking, the commit check, claims, `hook pre-tool`,
   `agent setup`.
5. **The app**: the Agents view, merging and archiving, hunk decisions,
   locks shown and broken, the actor in the history. After 1–4.
6. **E2E**: a small web app (code and assets), two scripted agents in two
   workspaces: different lines of one `.ts` (merged), the same lines
   (hunk decisions), one texture (refused), both merged.
