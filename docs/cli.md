# Command line tool

The desktop app covers everyday use. The command line tool `r3v.exe` is for
a few advanced tasks, scripts and AI agents. It is installed in the `bin`
folder of the R3V install folder (`%LOCALAPPDATA%\Programs\R3V\bin`), and
the installer puts that folder on your PATH: open a new terminal and type
`r3v`.

`r3v help` lists every command, in the groups below; `r3v <command>
--help` prints a command's lines of it, and `r3v help rules` prints [the
project's rules](profiles.md) (`.r3v.yaml`). Coming from Git: [R3V for Git
users](git-users.md) and the [glossary](glossary.md).

**Scripts and AI agents:** most commands take `--json` (one JSON object on
stdout, errors with fixed codes; the list is under `--json` in `r3v help`),
and no command ever waits for an answer. The fields and codes are in [R3V
for AI agents](agents.md), also printed by `r3v help agents`.

## Everyday (inside a project folder)

```
r3v commit -m "added drums"       # commit a version and share it (merges the team's versions first)
r3v commit -m "title" -m "details" # a title, then details (a blank line between, as git)
r3v commit -m "fix the menu" src/menu   # only the changes in those files and folders (the others stay)
r3v update [--preview]            # get the team's latest versions of your branch (or just look)
r3v status                        # what changed since your last version
r3v watch                         # keep running: tell you when someone commits on your branch
r3v log                           # versions
r3v log lane-design               # another branch's versions (its key or name)
r3v log --first-parent            # only the main line: each merge once, "(+N versions)" it brought in
```

`watch` never changes your files; the desktop app does the same in the
background.

`commit` and `update` merge Live Sets track by track and text files (code)
line by line. When you and a teammate changed the same track, the same
lines or the same file that can't be merged, they stop, change nothing, and
ask. They refuse to rewrite files while Live (or Unity, Unreal, Godot, a
design app) has the project open, if the team's changes must be merged in.

Decide all conflicts at once with `--strategy ours|theirs|both`, or one by
one by the conflict's key (`commit`, `update`, `merge` and `parked bring`
take them all):

```
r3v commit -m "..." --resolve "text:src/app.ts#1=theirs" --resolve "Song.als#track:14=ours"
r3v commit -m "..." --decisions decisions.json   # {"<key>": "ours|theirs|both"}; "-" reads stdin
r3v commit -m "..." --resolved src/app.ts=merged.ts   # the file as it should be
```

Keys: `text:<path>#<n>` a hunk of a text file (`both`: yours, then
theirs), `file:<path>` a whole file (settles all its hunks), `<set>#<key>`
a part of a Live set. `update --preview` lists them. Nothing changes until
every conflict is decided.

When `status` says a new folder looks like a build's output (named like
one the rules leave out, or full of built files, or big and new), committed
it would stay in the team's storage for good: `r3v rules ignore
"<pattern>"` adds the rule the note shows to `.r3v.yaml` (as the app's
**Leave it out**), then `status` again.

## Setup (the first time, in this order)

```
r3v login                         # sign in to R3V Cloud (in the browser): your teams appear in r3v teams
r3v teams                         # teams this computer is connected to
r3v init [--author NAME]          # start tracking this project (it says what to run next)
r3v init --preset code            # a folder R3V doesn't recognise: as code and documents (none: files as they are)
r3v init --team NAME              # make the folder a project of a team this computer has joined
r3v remote --team NAME            # connect a project made before to that team
r3v remote <connection code>      # or to a self-hosted team's storage (the code holds its keys)
r3v clone --team NAME <project> [folder]   # download a project of a team this computer has joined
r3v clone <connection code> <project> [folder]
r3v agent setup [--tool NAME]     # an AI agent, once per folder (see AI agents below)
r3v logout                        # sign this computer out of R3V Cloud
r3v version
```

A project on no team keeps its versions on this computer only: `init`
ends with what to run next (connecting it to a team, or signing in), and
`commit` and `status` say so until it's connected; `update` and `merge`
refuse (`not_connected`). Once connected, the next commit shares the
versions committed before.

**Creating a team on your own storage**, for scripts (the app's **Create a
team** does the same):

```
r3v connection-code --endpoint URL --bucket NAME --access-key K --secret-key S [--prefix P] [--name TEAM]
```

It checks that the key can read and write the bucket (conditional writes
included) and prints the connection code members paste into R3V. `--name`
names the team for everyone. The code contains the key: send it privately.

## Branches (advanced)

```
r3v branch                    # list branches and their latest versions
r3v branch new yi-ideas       # start a branch from your current version
r3v branch new NAME --agent   # an AI agent's branch, working for you
r3v switch main
r3v merge yi-ideas --preview  # what would come in and what conflicts
r3v merge yi-ideas            # merge into your branch and share
r3v merge yi-ideas -m "..."   # with your own description (else it lists what came in)
r3v branch log main           # who moved the branch, when, from which version to which
r3v branch archive yi-ideas   # put it away: its versions stay
r3v branch archived           # what was archived, by whom, when
r3v branch unarchive yi-ideas # back where it was
r3v branch delete yi-ideas    # an archived branch, for good
```

On a team that keeps branch names, a branch can be called anything (`r3v
branch new Mia's verse`, `r3v switch "Mia 的主歌"`); commands take its name
or its key.

A merge without `-m` describes what came in: `Merge yi-ideas: 3 versions`,
then each version's title (oldest first, at most 10; titles saying `WIP`,
`draft`, `do not merge` or `not for merge` left out). The app's merge
preview starts with the same text.

`merge` with changes not committed goes ahead, as `git merge` does, when
what comes in writes or deletes none of those files: they stay as they
were, uncommitted. When it would touch one, it refuses before changing
anything and names the files; a change of `.r3v.yaml` coming in touches
them all.

Switching with changes not committed parks them at the place they were
made; they come back when you return ([parking](design/parking.md)).

Every branch move is kept in the team's log (`branch log`). A branch moved
by mistake can be put back: go to the version it was on and commit from
there.

## Advanced

```
r3v commit --local -m "message"      # commit on this computer only (your next commit shares it)
r3v goto <id|HEAD~N> [--force]       # put the project in the state of a version (samples relinked)
r3v goto latest                      # back to the latest version
r3v discard                          # throw your changes away: the files as in the version you're on
r3v discard scenes/a.scn tmp/        # only those files and folders (--preview: list them first)
r3v parked [list]                    # changes kept while you're on another branch or version
r3v parked bring|discard BRANCH[@VERSION]
r3v export <id|HEAD~N> <folder>      # write a version as a separate project folder
r3v rules [check]                    # the project's rules (.r3v.yaml) and whether they work
r3v rules explain <file>...          # why a file is tracked or ignored
r3v rules preset <folder> <preset|none>
r3v rules ignore <pattern>
r3v gc                               # free space in .r3v
r3v verify [--repair]                # check the history (below)
r3v backup ...                       # back up the team (below)
r3v storage-cleanup [--delete]       # files in the team's storage no version uses (below)
```

On an older version, newer versions are kept. Committing, getting updates
and merging wait until you go back to the latest version; to continue from
the older one, start a branch there (`r3v branch new NAME`). An exported
copy has no R3V history; samples from outside the project are copied into
its `Samples/Imported`.

A version committed `--local` isn't on the team's branch until the next
commit shares it: the team, and the project's other copies on this computer
(workspaces), don't see it. `status` counts such versions, `workspace list`
each workspace's.

Versions store file contents by SHA-256 in `.r3v/objects` inside the
project (deduplicated). Samples referenced from outside the project are
stored too and, on a computer that lacks them, placed under
`.r3v/external/` with the set's sample paths rewritten. Samples from Live
packs are only recorded by pack name. `gc` frees space in `.r3v`: in a
team's project, its copies of files the team's storage has (Live Sets
stay); in any project, what no version uses.

### Checking the history

```
r3v verify            # read every version and stored file again, report damage
r3v verify --repair   # bring back what can be: from the project folder or the team's storage
```

The app does the same in a project's settings (**Check project…**). A file
damaged here (a failing disk, say) comes back from a file in the project
folder with the same content, or from the team's storage; what has no copy
left anywhere is listed with the version it's in. Once a version is shared,
R3V lets go of its copies here of files the team's storage has, so few
files stored here is expected, not history lost; going back to an old
version then needs the team's storage.

### Backing up the team

```
r3v backup run                   # back up where the app backs this team up (a folder or a bucket)
r3v backup run D:\Backups\Band   # ...or to any folder: empty, or this team's earlier backup
r3v backup status                # this computer's backup, and who else backs up the team
r3v backup restore --preview     # what restoring would bring back (and the runs to pick from)
r3v backup restore [folder] [--run 20261004-153000]
r3v backup restore <connection code>   # a backup in a bucket (r3v connection-code makes the code)
```

For teams on their own storage. `--team NAME` picks the team; otherwise it
is the team of the project you're in, or the current one. A backup holds
every project of the team with all its versions. Runs only copy what is new
and never delete. `restore` copies back only what the team's storage lacks
(deleted projects, lost files, or everything into a new, empty bucket) and
never changes or deletes anything there; `--run` restores as of an earlier
run. In the app: Team settings › **Backup** ([getting
started](getting-started.md#6-back-up-the-team), [design](design/backup.md)).

### Cleaning up the team's storage

```
r3v storage-cleanup            # files in the team's storage no version of any project uses
r3v storage-cleanup --delete   # delete those that are due
```

Or in the app: Team settings › **Storage cleanup**. For teams on their own
storage. Deleted projects and uploads that stopped leave files behind. A
file is deleted only once a cleanup has found it unused a day before and it
is at least a week old; files a teammate's share in progress relies on are
never deleted. Deleted files go to a trash in the storage for two weeks
first: should a cleanup ever be wrong, R3V takes a file back from there
when it's needed (and `r3v verify --repair` does for a whole project).

## Projects of the team

```
r3v project list              # the team's projects, archived ones marked
r3v project archive           # put this project away for the whole team
r3v project unarchive         # bring it back
r3v project restore           # bring back a project its team deleted (or moved away), with its history
```

Archiving hides the project for everyone in the team; nothing is deleted
and this folder stays as it is. With uncommitted changes or versions not
shared it's refused unless `--force`. On an R3V Cloud team only its owners
and admins archive and restore.

## R3V Cloud

```
r3v move-team --to ADDRESS --key-id ID [--secret S] [--project ID]... [--no-finish]
r3v move-team --finish
```

Moves the team of the project here from its own storage to R3V Cloud, as
the app's Team settings › **Move to R3V Cloud** does: the service copies
the team's history itself with a read-only key for the bucket
(`R3V_MOVE_SECRET` keeps the secret off the command line), then this
computer finishes; the team's projects follow. `--no-finish` only starts
the copy; `--finish` finishes a move started before. See
[design/moving.md](design/moving.md).

## Files

Each kind of file the way it reads: text line by line, a Live Set track by
track, a picture its size, any other file its size.

```
r3v diff [PATHS]                            # your changes, since the version you're on
r3v diff --version V [PATHS]                # what a version changed
r3v diff --from A --to B [PATHS]            # between two versions
r3v diff --files <a> <b>                    # two files on disk, any kind (two Live Sets: track by track)
r3v info <file>                             # what it is (a set's tracks, audio's format, a picture's
                                            # size, text's lines) and whether R3V keeps it
r3v merge-files <base> <ours> <theirs> -o out [--strategy fail|ours|theirs|both] [--as text|whole]
r3v merge-sets <base> <ours> <theirs> -o out.als [--strategy fail|ours|theirs|both]
```

`merge-files` merges each file the way a branch merge would: with the
handler the project's rules give `out` when it is in a project, else the one
the presets give its name (text line by line, a Unity scene with
UnityYAMLMerge). A kind no handler takes, a `.fbx` say, is merged whole: a
conflict when both sides changed it. Outside a project, a file no preset
knows goes by its content: text line by line, anything else whole. `--as
text` merges it line by line anyway, `--as whole` chooses it whole.
`merge-sets` is the same for Live Sets.

## AI agents on one computer

Several agents can work on one project at once, each in a workspace of its
own (below). How it works: [design/agent-workspaces.md](design/agent-workspaces.md);
for the person setting it up: [getting started](getting-started.md#7-ai-agents-on-your-project).

```
r3v agent setup --tool claude-code     # once in a project or workspace folder (any tool: codex, cursor…)
r3v lock <paths…> [--claim]            # lock files or folders for this workspace, all or none
r3v unlock <paths…>                    # free this workspace's locks
r3v locks                              # every agent lock here (stale ones marked), and the files the person changed
r3v lock --take <paths…> --reason <why>  # take over locks whose holder is gone, when the user says so
r3v lock --break <path>                # take anyone's lock off a path (the person's call)
```

`agent setup` works for any agent tool: it writes the agents' guide into
the project's `AGENTS.md` (R3V's part between two markers, the rest of the
file left as it is; commit it) and the copy's identity (`.r3v/agent.json`:
this folder's only, never tracked), so its versions say which agent made
them. For Claude Code it also writes `.claude/settings.local.json` (R3V's
hooks and `R3V_ACTOR`, merged with what is there; never tracked) and
`@AGENTS.md` in `CLAUDE.md`. Without `--tool` it sets up the tool R3V tells
from the environment. `--as person` makes any command the person's own in a
folder set up for an agent; the app's versions are always the person's.
Who made a version: [design/agent-identity.md](design/agent-identity.md).

Agent locks keep two agents from changing the same file that can't be
merged (textures, models, sounds, `.blend`, Unity and Unreal assets). Code
never locks: it merges hunk by hunk; `--claim` only tells other agents
you're on it. Claude Code's hook locks a file when the agent first changes
it, or refuses the change saying who holds it; a commit by an agent
changing a file another workspace holds, or one the person changed and
hasn't committed, is refused. A person's commit never is. A lock is held
until the work is merged into the branch the workspaces merge into; one
whose agent hasn't been heard of for 15 minutes is stale, and the next lock
asked over it takes it. Locks are kept on this computer only
(`.r3v/agent-locks`); the team or the project can turn them off (Workflow
settings).

## Milestones

```
r3v milestone add "Sent to the label" [VERSION] [-m NOTE]   # name a shared version for the team (a tag)
r3v milestones                # the project's milestones, newest first (also: r3v milestone list)
r3v milestone rename NAME NEW
r3v milestone note NAME NOTE
r3v milestone remove NAME     # the version stays
```

`r3v milestone NAME` names one too, unless NAME is one of the command's
words (`add`, `list`, `ls`, `rename`, `note`, `remove`, `rm`, `delete`,
`show`, `help`). `tag` and `tags` work as well.

## Results

```
r3v result <check> pass|fail|running [-m NOTE] [--version V]   # record what a check said about a version
r3v results [VERSION]                                         # the checks' latest results
```

What a build, the tests or any check said about a version, recorded by
whoever ran it, as a Git host's commit status. Kept on this computer beside
the version (never in it), for every copy of the project. `workspace list`
and the app's Workspaces tab show each workspace's.

## Sections

```
r3v section list              # the team's sections, with their projects
r3v section new Engine        # add a section (last)
r3v section add Engine        # put this project in it
r3v section remove            # take this project out of its section
r3v section rename Engine Game
r3v section delete Game       # its projects become ungrouped
```

Sections group the team's projects in the app's left panel and the team's
home, the same for everyone in the team; a project is in one at most (the
order they show in is each person's own). On an R3V Cloud team only its
owners and admins change them.

## Steps: an agent's moves

```
r3v steps [--session ID] [--list]      # the latest session's prompts and steps (--list: the sessions)
r3v step [-m NOTE]                     # a step of the files as they are now
r3v step start -m "WHAT"               # an agent tool without hooks begins a task
r3v step end [-m "WHAT I DID"]         # and ends it (a commit ends it too)
r3v step back <n> [--session ID]       # the files as at step n (the files now are a step first)
r3v step diff <a> [<b>] [--session ID] # what changed from step a to step b (or to the files now)
```

A step is the project after one of an agent's moves: after each tool call
that changed files, R3V keeps the files as they are, under the prompt that
asked for it (Claude Code, through the hooks `agent setup` adds; other
tools with `step start` and `step end`). Steps stay on this computer
(`.r3v/steps/`), never in the history or the team's storage; a session is
kept while it has steps on the version the project is on, then 7 days,
within 2 GB. Prompts are kept with keys and passwords taken out; the
project's Workflow settings turn prompts, or steps, off. `step back` is the
person's: it changes the files of the copy it runs in. See
[design/steps.md](design/steps.md).

## Taking back a merge

```
r3v undo-merge <version> [-m MESSAGE] [--strategy keep|undo|both] [--resolve KEY=keep|undo|both]
```

A new version on your branch without what that merge brought in; the
history stays. What changed since the merge stays; where the same thing
changed again it asks as a merge does: `keep` keeps it as it is now,
`undo` puts it back as before the merge. It refuses with uncommitted
changes.

## Workspaces: several agents on one project

```
r3v workspace add --branch renderer                       # another copy, on a branch of its own (../game.workspaces/renderer)
r3v workspace add ../elsewhere/tauri --branch tauri --from main   # or in a folder you choose
r3v workspace add --branch renderer --agent claude-code   # set up for the agent
r3v workspace list                                        # each one: branch, ahead/behind, changes
r3v workspace list --against dev                          # ahead/behind dev instead of each one's base
r3v workspace base ../game.workspaces/renderer dev        # it merges into dev from now on
r3v workspace remove ../game.workspaces/renderer [--force]        # once its branch is merged
```

A workspace is another folder with the same project, on this computer, for
you or an agent to work in while others work in theirs. With `--agent
claude-code` it is an agent's: its branch is marked an agent's (folded away
in the app's graph once merged) and Claude Code is set up in it. Every
command works inside it; its commits share on its own team branch, and the
main copy merges that branch when the work is done (`r3v merge renderer`).

- It is made from what is here: nothing is downloaded that is on this
  computer, and on a ReFS or Dev Drive volume files are block-cloned (no
  space taken until a file changes). What the rules leave out
  (`node_modules/`, `target/`) isn't copied, unless `.r3v.yaml` names it
  under `workspaces: copy` ([profiles.md](profiles.md#workspaces)).
- The workspaces share the main copy's store and versions; each keeps its
  own place, file list and parked changes. `gc` in any copy keeps what
  every copy needs.
- A workspace merges into the branch it was made from (its base) until
  `workspace base` names another.
- `remove` refuses a workspace with versions not merged into its base,
  changes not committed, or parked changes (`--force` loses them). Its
  branch stays on the team. A folder left by an `add` or `remove` that
  stopped half-way is finished by `add` again, or cleared by `remove`.
- A workspace can't be inside the project's folder (or hold it). Its team
  is the main copy's: connecting, renaming or moving the project is done
  there.

## For programs and AI agents

- `--json`: one JSON object on stdout ([agents.md](agents.md#output)).
- `--wait SECONDS`: while the app or another command is working on the
  project, commands that change it wait up to 10 seconds, then fail saying
  who has it (`r3v commit (pid 1234) since 14:02:11`). Commands that only
  read it (`status`, `log`, `diff`, `branch`…) read it as it is, without
  waiting, and say so.
- `--as person`: the command is the person's own, in a folder set up for an
  agent.
- `R3V_CONFIG_DIR=<folder>` (below).
- `r3v help agents [--snippet]`, `r3v help rules`.

### Sandboxes and R3V's settings folder

R3V's own settings (the teams this computer joined and their keys,
`teams.json`; the app's lists of projects; logs) are in `%APPDATA%\R3V`.
`R3V_CONFIG_DIR=<folder>` keeps them in that folder instead: for a sandbox,
a CI job or an agent that may only write in its workspace. Projects are
unaffected (each keeps its own `.r3v`). The teams joined in the usual
folder aren't in a new one: copy `teams.json` over (the same Windows user
can read its keys), or join again; an R3V Cloud sign-in stays in Windows'
Credential Manager.

## When something goes wrong

The codes are what `--json` gives ([agents.md](agents.md#errors) has
them all); the text output says the same in words.

- **`project_busy`**: the app or another command is working on the
  project. Try again in a moment, or `--wait` longer.
- **`no_write_access`**: R3V may not write a file at all (a sandbox, a
  read-only folder). No one holds it, so waiting won't help: when the path
  is R3V's settings folder, set `R3V_CONFIG_DIR` (above); in the project,
  give R3V access.
- **`switch_unfinished`**: R3V was closed while a `goto`, `switch`,
  `update`, `merge` or `undo-merge` rewrote the files. The next command
  puts them back as they were before it (the app offers to), unless a tool
  has the project open or a file changed since: then it says so and leaves
  the files as they are. Close the tool and run any command; for files
  changed since, `r3v goto <version> --force` replaces them (or the app's
  offer, which lists them first). Until then no commit takes the files.
- **Damage** (a failing disk, a file R3V can't read): `r3v verify`, then
  `r3v verify --repair`.
- **`needs_nightly`**, **`team_needs_features`**,
  **`newer_version_needed`**: the project or the team uses something this
  R3V doesn't have ([getting started](getting-started.md#stable-and-nightly)).
- **`not_connected`**: the project is on no team; `r3v teams`, then `r3v
  remote --team NAME`.
