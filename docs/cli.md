# Command line tool

The desktop app covers everyday use, including creating a team on Cloudflare R2. The command line tool `r3v.exe` is for a few advanced tasks, scripts and AI agents. It is installed in the `bin` folder of the R3V install folder (`%LOCALAPPDATA%\Programs\R3V\bin`), and the installer puts that folder on your PATH: open a new terminal and type `r3v`. Run `r3v help` for the full list.

Coming from Git: [R3V for Git users](git-users.md) and the [glossary](glossary.md).

**Scripts and AI agents:** `status`, `log`, `commit`, `update`, `merge`, `switch`, `goto`, `discard`, `parked`, `workspace` and `version` take `--json` (one JSON object on stdout, errors with fixed codes), and no command ever waits for an answer. See [R3V for AI agents](agents.md), also printed by `r3v help agents`. Every command takes `--help` (its lines of `r3v help`); `r3v help rules` prints [the project's rules](profiles.md) (`.r3v.yaml`).

## Team storage (S3-compatible)

```
r3v connection-code --endpoint URL --bucket NAME --access-key K --secret-key S [--prefix P] [--name TEAM]
```

The same as **Create a team** in the app, for scripts: checks that the key can read and write the bucket (conditional writes included) and prints the connection code members paste into R3V. `--name` names the team for everyone. The code contains the key: send it privately.

## Everyday commands (inside an Ableton project folder)

```
r3v init [--author NAME]          # start tracking this project
r3v init --preset code            # a folder R3V doesn't recognise: as code and documents (or none: files as they are)
r3v remote <connection code>      # connect it to the team's storage
r3v commit -m "added drums"       # commit a version and share it (merges the team's versions first)
r3v commit -m "title" -m "details" # a title, then details (a blank line between, as git)
r3v commit -m "fix the menu" src/menu   # only the changes in those files and folders (the others stay)
r3v update [--preview]            # get the team's latest versions of your branch (or just look)
r3v status                        # what changed since your last version
r3v log                           # versions
r3v clone <connection code> "Song" [folder]   # download a team project
r3v teams                         # teams this computer is connected to
```

`r3v watch` keeps running while you work and tells you when someone commits a new version on your branch. It never changes your files. The desktop app does the same in the background.

`commit` and `update` merge Live Sets track by track. When you and a teammate changed the same track (or the same sample file) they stop and ask for `--strategy ours|theirs|both`. They refuse to rewrite sets while Ableton Live is running if the team's changes must be merged in.

Text files (code, on Nightly) merge line by line; where both changed the same lines, each such place (a hunk) is a decision of its own. `commit`, `update`, `merge` and `parked bring` take decisions one by one, by the conflict's key, besides `--strategy` (which decides whatever is left):

```
r3v commit -m "..." --resolve "text:src/app.ts#1=theirs" --resolve "Song.als#track:14=ours"
r3v commit -m "..." --decisions decisions.json   # {"<key>": "ours|theirs|both"}; "-" reads stdin
r3v commit -m "..." --resolved src/app.ts=merged.ts   # the file as it should be
```

Keys: `text:<path>#<n>` a hunk (`both`: yours, then theirs), `file:<path>` a whole file (settles all its hunks), `<set>#<key>` a part of a Live set. Nothing changes until every conflict is decided.

## Branches (advanced)

```
r3v branch                    # list branches and their latest versions
r3v branch new yi-ideas       # start a branch from your current version
r3v switch main
r3v merge yi-ideas --preview  # what would come in and what conflicts
r3v merge yi-ideas            # merge into your branch and share
r3v branch log main           # who moved the branch, when, from which version to which
r3v branch archive yi-ideas   # put it away: its versions stay
r3v branch archived           # what was archived, by whom, when
r3v branch unarchive yi-ideas # back where it was
r3v branch delete yi-ideas    # an archived branch, for good
```

On a team that keeps branch names (Nightly), a branch can be called anything
(`r3v branch new Mia's verse`, `r3v switch "Mia 的主歌"`); commands take its
name or its key. `r3v branch new NAME --agent` marks it an AI agent's,
working for you (`r3v branch` shows it so).

Versions an AI agent commits through the CLI record it (`r3v log` shows
"yi (claude-code)"): R3V reads `R3V_ACTOR` (the tool; `R3V_ACTOR_MODEL`
and `R3V_ACTOR_SESSION` too; `none` for a person), else the variables
tools set themselves (Claude Code's `CLAUDECODE`).

Teams on storage keep a log of every branch move (each move is its own record, never changed). A branch moved by mistake can be put back: check out the version it was on and commit from there.

## Workspaces: several agents on one project (Nightly)

```
r3v workspace add ../game.wt/renderer --branch renderer   # another copy, on a branch of its own
r3v workspace add ../game.wt/tauri --branch tauri --from main
r3v workspace list [--json]                               # each one: branch, ahead/behind, changes
r3v workspace remove ../game.wt/renderer [--force]        # once its branch is merged
```

A workspace is another folder with the same project, on this computer, for
you or an agent to work in while others work in theirs. With `--agent
claude-code` it is an agent's: its branch is marked an agent's (folded away
in the app's graph once merged) and Claude Code is set up in it; without,
its branch is an ordinary one. Every command
works inside it (`status`, `commit`, `update`, `merge`, `log`, …); its
commits share on its own team branch, and the main copy merges that branch
when the work is done (`r3v merge renderer`).

- It is made from what is here: the version's files come from the main
  copy's store or from a copy's file with the same content (on a ReFS or
  Dev Drive volume, block-cloned: no space taken until a file changes);
  nothing is downloaded that is on this computer. What the rules leave out
  (`node_modules/`, `target/`) isn't copied, unless `.r3v.yaml` names it
  under `workspaces: copy` (see [profiles.md](profiles.md)): those build
  folders are copied (block-cloned on ReFS and Dev Drive), links left out.
- `r3v result tests pass -m "…"` in a workspace records what its tests (or
  a build) said about the version it is on; `workspace list` and the app's
  Agents tab show it (`r3v results [VERSION]` lists a version's).
- The workspaces share the main copy's store and versions (`.r3v` of the
  main copy); each keeps its own place, files list and parked changes in
  its own `.r3v`. Freeing space (`gc`) in any copy keeps what every copy
  needs, and waits for a time when no copy is at work.
- `remove` refuses a workspace with versions not merged into the branch
  it was made from, changes not committed, or parked changes (`--force`
  loses them). Its branch stays on the team. A folder left by an `add` or
  `remove` that stopped half-way is finished by `add` again, or cleared by
  `remove`.
- A workspace can't be inside the project's folder (or hold it). Its team
  is the main copy's: connecting, renaming or moving the project is done
  there.

## Results (Nightly)

```
r3v result <check> pass|fail|running [-m NOTE] [--version V]   # record what a check said about a version
r3v results [VERSION]                                         # the checks' latest results
```

What a build, the tests or any check said about a version, recorded by whoever ran it, as a Git host's commit status: the latest per check, with the changes not committed then counted (the result is about those files). Kept on this computer beside the version (never in it: its id stays), for every copy of the project; gone with the version. `workspace list` shows each workspace's.

## Older versions

```
r3v goto <id|HEAD~N> [--force]       # put the project in the state of a version (samples relinked)
r3v goto latest                      # back to the latest version
r3v discard                          # throw your changes away: the files as in the version you're on
r3v export <id|HEAD~N> <folder>      # write a version as a separate project folder
r3v commit --local -m "message"      # commit a version on this computer only
```

On an older version, newer versions are kept. Committing, getting updates and merging wait until you go back to the latest version; to continue from the older one, start a branch there (`r3v branch new NAME`). An exported copy has no R3V history; samples from outside the project are copied into its `Samples/Imported`.

Versions store file contents by SHA-256 in `.r3v/objects` inside the project (deduplicated). Samples referenced from outside the project are stored too and, on a computer that lacks them, placed under `.r3v/external/` with the set's sample paths rewritten. Samples from Live packs are only recorded by pack name. `Backup/` and `*.asd` are ignored.

## Checking the history

```
r3v verify            # read every version and stored file again, report damage
r3v verify --repair   # bring back what can be: from the project folder or the team's storage
```

The app does the same from a project's ⋯ menu in the sidebar (**Check project…**). A file damaged here (a failing disk, say) comes back from a file in the project folder with the same content, or from the team's storage; what has no copy left anywhere is listed with the version it's in.

## Backing up the team

```
r3v backup run                   # back up where the app backs this team up (a folder or a bucket)
r3v backup run D:\Backups\Band   # ...or to any folder: empty, or this team's earlier backup
r3v backup status                # this computer's backup, and who else backs up the team
r3v backup restore --preview     # what restoring would bring back (and the runs to pick from)
r3v backup restore [folder] [--run 20261004-153000]
r3v backup restore <connection code>   # a backup in a bucket (r3v connection-code makes the code)
```

`--team NAME` picks the team; otherwise it is the team of the project you're in, or the current one. A backup holds every project of the team with all its versions, as the team's storage keeps them. Runs only copy what is new and never delete: what the team deleted stays in the backup. Each run records where every branch was (`runs/<time>.json`).

Or in the app: the team's settings (⚙) → **Backup**: a folder (a drive, a NAS), or **Another bucket…** (any S3-compatible storage, with its own keys; not the team's own storage). The app backs up once a day while it is open. Works on teams that use S3 or R2 storage. `restore` copies back only what the team's storage lacks (deleted projects, lost files, or everything into a new, empty bucket) and never changes or deletes anything there. It restores from this computer's backup, a folder, or a backup bucket (also one this computer doesn't back up to, e.g. a teammate's: in the app **Restore… → Bucket…**; its keys only need to read it and aren't kept); `--run` restores as of an earlier backup run (projects made later stay out; branches the team still has stay as they are). See [design/backup.md](design/backup.md).

## Cleaning up the team's storage

```
r3v storage-cleanup            # files in the team's storage no version of any project uses
r3v storage-cleanup --delete   # delete those that are due
```

Or in the app: the team's settings (⚙) → **Storage cleanup**. Deleted projects and uploads that stopped leave files behind. A file is deleted only once a cleanup has found it unused a day before and it is at least a week old; files a teammate's share in progress relies on are never deleted. Deleted files go to a trash in the storage for two weeks first: should a cleanup ever be wrong, R3V takes a file back from there when it's needed (and `r3v verify --repair` does for a whole project). Works on teams that use S3 or R2 storage.

## AI agents on one computer (Nightly)

```
r3v agent setup [--tool claude-code]   # in a project or workspace folder: Claude Code's hook, R3V_ACTOR, the guide
r3v lock <paths…> [--claim]            # lock files or folders for this workspace, all or none
r3v unlock <paths…>                    # free this workspace's locks
r3v locks                              # every agent lock here (stale ones marked), and the files the person changed
r3v lock --break <path>                # take anyone's lock off a path (the person's call)
```

Agent locks keep two agents (each in a workspace of the project) from changing the same file that can't be merged: textures, models, sounds, `.blend`, Unity and Unreal assets (the team's lock kinds when it has them, `.r3v.yaml`'s `file_locks` narrowing them). Code never locks: it merges hunk by hunk; `--claim` only tells other agents you're on it. `agent setup` writes `.claude/settings.local.json` (a `PreToolUse` hook running `r3v hook pre-tool`, and `R3V_ACTOR` in `env`, merged with what is there; this copy's, never tracked) and, in the main copy, adds the agents' lines to `CLAUDE.md` (commit it); `workspace add --agent claude-code` sets a workspace up; the hook locks a file when the agent first changes it, or refuses the change saying who holds it. A commit by an agent changing a file another workspace holds, or one the person changed and hasn't committed, is refused (`files_locked_by_agent`); a person's commit never is. A lock is held until its branch is merged; one whose agent hasn't been heard of for 15 minutes is stale, and the next lock asked over it takes it. Kept on this computer only, beside the project's store (`.r3v/agent-locks`). See [design/agent-workspaces.md](design/agent-workspaces.md).

## Steps: an agent's moves (Nightly)

```
r3v steps [--session ID] [--list]      # the latest session's prompts and steps (--list: the sessions)
r3v step [-m NOTE]                     # a step of the files as they are now
r3v step back <n> [--session ID]       # the files as at step n (the files now are a step first)
r3v step diff <a> [<b>] [--session ID] # what changed from step a to step b (or to the files now)
```

A step is the project after one of an agent's moves: after each tool call that changed files (Edit, Write, a shell command, an MCP tool), R3V keeps the files as they are, under the prompt that asked for it. `agent setup` adds the hooks that make them (`UserPromptSubmit`: `r3v hook prompt`; `PostToolUse`: `r3v hook post-tool`); run it again in a copy set up before 0.1.44. Steps stay on this computer (`.r3v/steps/<session>.jsonl`, their files in the local store), never in the history or the team's storage; a session is kept while it has steps on the version the project is on, then 7 days, within 2 GB. Prompts are kept with keys and passwords taken out; the app's project settings turn prompts, or steps, off. `step back` is the person's: it changes the files of the copy it runs in. See [design/steps.md](design/steps.md).

## What changed in a file (Nightly)

Each kind of file the way it reads: text line by line, a Live Set track by
track, a picture its size, any other file its size (`--json`: `kind`,
`hunks`, `set_changes`, `picture`, `size_old`, `size_new`).

```
r3v diff [PATHS]                            # your changes, since the version you're on
r3v diff --version V [PATHS]                # what a version changed
r3v diff --from A --to B [PATHS]            # between two versions
```

## Working with Live Sets directly

```
r3v info <set.als>                          # tracks, devices, clips, automation, plugins, samples
r3v diff <a.als> <b.als>                    # two sets on disk, track by track
r3v merge-sets <base> <ours> <theirs> -o out.als [--strategy fail|ours|theirs|both]
```
