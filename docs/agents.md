# R3V for AI agents

How an AI agent (Claude Code, Codex, Cursor…) uses R3V in a user's
project. Also in the CLI: `r3v help agents` (this page, for the installed
version) and `r3v help agents --snippet` (a few lines for a project's
`AGENTS.md` or `CLAUDE.md`).

R3V is version control for creative projects: Ableton Live sets, Unity,
Unreal and Godot projects. A project folder with a `.r3v` folder is tracked.
**`commit` makes a version and shares it with the team in one step**; there
is no separate push. (`save`, `snapshot` and `checkout`, the names of
`commit`, `commit --local` and `goto` before, still work.) Coming from Git: [git-users.md](git-users.md); the
words: [glossary.md](glossary.md).

## Before you start

A project that isn't on a team keeps every version on this one computer:
nobody else gets them, and nothing is backed up. Check, once per project:

1. **Is it a project, on a team?** `r3v status --json`: `connected`
   (`true`: on a team). `false` (or a new folder): the project's
   versions stay here; `next` lists the ways to connect it:
   - a team this computer has joined (`r3v teams` lists them):
     `r3v remote --team NAME`, or `r3v init --team NAME` for a
     folder that isn't a project yet;
   - none yet: the person signs in to R3V Cloud (`r3v login`, in the
     browser) and the teams they are in appear in `r3v teams`; or the
     person connects a self-hosted team with its connection code
     (`r3v remote <code>`). Never ask for the code or print it.

   Ask the user which team when there's more than one. Committing before
   it is connected is fine: the next commit once connected shares those
   versions.
2. **Are you set up as an agent?** The versions you make should say an
   agent made them (`actor` in `log --json`). Once per folder, run
   `r3v agent setup --tool NAME [--model MODEL]` with your tool's name
   (`claude-code`, `codex`, `opencode`, `gemini-cli`, `cursor`, `pi`...;
   left out, the tool R3V tells from its variables): it keeps who you
   are in this copy (`.r3v/agent.json`: this folder's only, never
   tracked or shared) and puts the agents' lines in `AGENTS.md` (commit
   it). For Claude Code it also writes `R3V_ACTOR` and its hooks in
   `.claude/settings.local.json`, and `@AGENTS.md` in `CLAUDE.md`.

`r3v init` says the same when it makes a project: its text ends with
what to run next, and `init --json` gives it as `next`: `[{"command",
"why"}]`, in order (connect, set up, the first commit). `commit` and
`status` in a project on no team add a warning `not_connected` and the
same `next` (see Output); `update` and `merge` fail with
`not_connected`, its `next` in `error`.

## Rules

- **Use `--json`** with `init`, `status`, `log`, `commit`, `update`, `merge`, `undo-merge`, `switch`, `goto`, `discard`, `parked`, `rules ignore`, `backup`, `verify`, `project`, `section`, `lock` and `locks`, and
  read the result, not the text.
- **Commands never wait for an answer.** When one is needed they stop with
  an error code (below). Never pipe answers into R3V.
- **Don't use `--force` or `r3v discard` on your own.** `--force` makes
  R3V rewrite sets while Ableton Live may have them open, or discard
  changes; `discard` throws the changes away. Ask the user. To show them
  what would go, `r3v discard [<path>...] --preview --json` changes
  nothing; with paths, `discard` drops only the changes in those files
  and folders (as `commit` takes paths), the rest stays. Its result
  lists them: `discarded: [{path, status, from}]`.
- **Never print connection codes** (`r3v connection-code`, `r3v
  remote <code>`, `r3v clone <code>`): they contain the team's storage
  keys. Don't put them in files, logs or messages, and don't ask the user
  for one: a new project goes on a team this computer has joined, by its
  name: `r3v teams` lists them, `r3v init --team NAME` makes the
  folder that team's project (`remote --team NAME` for one made before,
  `clone --team NAME <project>` to download one).
- **Don't edit `.r3v/`.** It is R3V's own data; keep it out of formatters,
  linters and search-and-replace too (Biome: `"!**/.r3v"` in
  `files.includes`; Prettier: `.r3v/` in `.prettierignore`). `.r3v.yaml` (the
  project's rules: what is tracked) is fine to read and edit; `r3v help
  rules` has its syntax, `r3v rules` validates it and `r3v rules
  explain <file>` says why a file is tracked. Leave out what a build or a
  tool makes again (caches, renders, build outputs) before the first
  commit: `rules: - ignore: "renders/"`.
- **Steps** (Nightly): R3V's hooks keep the files after each of your tool
  calls, under the person's prompt, so the person can go back to any of
  them. Nothing to do on your side; don't run `r3v step back` yourself
  (it changes the files of the copy, the person's edits included).
- **Don't commit unfinished work to merge it later.** A branch is merged
  whole: what isn't finished stays uncommitted (switching away parks it,
  Nightly: `r3v parked`), or is finished before the branch is merged.
  A version whose title says `WIP`, `draft`, `do not merge` or `not for
  merge` is left out of the merge's description and marked `wip` in
  `merge --preview`: don't merge such a branch without asking the user.
- Version ids are long hex strings; any unique prefix (10 characters, as
  the text output shows) works where a command takes one.

## If you know Git

- One command commits and shares (`commit`); there is no staging, push,
  rebase or force push.
- A conflict is a track of a Live set, a hunk of a text file or a whole
  file, never markers in a file: decide each (`--resolve <key>=<choice>`),
  or all at once (`--strategy ours|theirs|both`).
- `update` never runs by itself, and keeps uncommitted changes.
- Every version keeps the branch it was made on (`log --json`:
  `made_on`); merging another branch always makes a version.
- Switching with uncommitted changes parks them (Nightly); they come back
  on return (`switch --json`: `parked`, `restored`).
- Branches are archived (`branch archive`), not deleted; only an archived
  branch is deleted for good (`branch delete`: ask the user).
- A tag is a milestone: `milestone NAME [VERSION] [-m NOTE]`
  names a shared version for the whole team, `milestones` lists them
  (`tag` and `tags` work too). Only shared versions take one.

## Everyday flow

```sh
r3v status --json            # what changed; is the team ahead?
r3v diff --json              # what changed in each file (text: line hunks)
r3v update --preview --json  # what the team shared: versions, changes, conflicts
r3v commit -m "Brighter mix on the chorus" --json
r3v commit -m "Fix the renderer's lights" -m "They were twice too bright on HDR screens." --json
```

A version's message is a title (the first `-m`) and details (each further
`-m`, a blank line between, as Git does): keep the title short, say why in
the details.

The versions you make record that an agent made them (`actor`), besides
the member you work for (`author`). Who that agent is, in order:

1. `R3V_ACTOR` in the environment (the tool's name; `R3V_ACTOR_MODEL`,
   `R3V_ACTOR_SESSION` optional; `none`: a person, whatever else says);
2. the copy's own identity, `.r3v/agent.json` (`r3v agent setup`);
3. the variables tools set themselves in the commands they run (Claude
   Code's `CLAUDECODE`, Codex's `CODEX_THREAD_ID`, opencode's `OPENCODE`,
   Gemini CLI's `GEMINI_CLI`, Pi's `PI_CODING_AGENT`, and others: see
   [design/agent-identity.md](design/agent-identity.md)); one of another
   tool than the copy's wins over 2 (that tool runs here now);
4. else a person.

The model is `R3V_ACTOR_MODEL`, else the tool's own (Pi's `PI_MODEL`),
else the copy's when it is the same tool. The person's own commands in a
folder set up for an agent: `--as person` (any command), or
`R3V_ACTOR=none` in their shell; the R3V app's versions are always the
person's.

`commit` merges what teammates shared in the meantime before sharing. When the
same track (or file) changed on both sides it stops with `merge_conflict`:

```sh
r3v update --preview --json  # result.conflicts: file, unit (track), description
# decide with the user, then:
r3v commit -m "..." --strategy theirs --json
```

Strategies: `ours` (keep this computer's), `theirs` (keep the team's),
`both` (keep both: the track twice, or the file under a new name, when
`can_keep_both`). A strategy applies to every conflict of that run not
decided otherwise.

### Code: hunk by hunk

Text files (code: `.ts`, `.rs`, `.json`, `.toml`, `.glsl`…) merge line by
line. Where you and a teammate changed the same lines, each such place is
a conflict of its own, a **hunk**: key `text:<path>#<n>` (`n` from 1, in
file order), `unit` (`"lines 40–52"`, in your file), `hunk` (n), and its
lines on each side: `base`, `ours`, `theirs`, each `{"start", "count",
"text"}` (`start` from 1; `count` 0 when that side has none there).

1. Run the command (`commit`, `update`, `merge`, `parked bring`); on
   `merge_conflict`, read `error.conflicts`. Nothing was changed.
2. Decide each hunk: `ours`, `theirs`, or `both` (ours, then theirs).
   Or write the file as it should be (both sides' changes combined by
   hand) to a file outside the project.
3. Run the same command again with the decisions:

```sh
r3v commit -m "..." --resolve "text:src/render.ts#1=theirs" --resolve "text:src/render.ts#2=both" --json
r3v commit -m "..." --decisions decisions.json --json   # {"text:src/render.ts#1": "theirs", ...}; "-": stdin
r3v commit -m "..." --resolved src/render.ts=/tmp/render.ts --json   # the file as it should be
```

The file is written only once all its hunks are decided; with some left,
the run stops again listing only those (same keys). `--strategy` decides
whatever is left. `file:<path>` decides a whole file (all its hunks, the
other side's other changes too). A bad key or choice is a `usage` error.

Lockfiles (`package-lock.json`, `Cargo.lock`, `pnpm-lock.yaml`…) merge as
text too; when they conflict, don't combine hunks: take one side whole
(`--resolve file:package-lock.json=theirs`), then regenerate it (`npm
install`, `cargo update -w`) and commit.

To only get the team's versions: `r3v update --json`. Changes not
committed yet stay as they are (still not committed) and the team's versions are merged
into the files; when you and a teammate changed the same file or track it
stops with `merge_conflict` first (`--strategy` decides). `result.kept_work`
is true when uncommitted changes were kept.

## Output

Every `--json` command prints one object on stdout:

```json
{"schema": 1, "ok": true, "command": "status", "result": { ... }}
{"schema": 1, "ok": false, "command": "commit",
 "error": {"code": "merge_conflict", "message": "...", "hint": "...", "exit": 3, "conflicts": [ ... ]}}
```

Warnings (e.g. "wrote .r3v.yaml") go to stderr. `schema` changes only
when a field changes meaning; new fields may appear any time.

`status` result:

| field | |
|---|---|
| `project`, `branch` | names |
| `version` | the version the files are on (`""` before the first) |
| `on_older_version`, `latest` | on an older version; `r3v goto latest` goes back |
| `team` | `null` without a team, else `reachable`, `incoming` (others shared versions: update), `error` |
| `connected` | `false`: the project is on no team, its versions stay on this computer; then `next` (`[{"command", "why"}]`) lists the ways to connect it (see Before you start) |
| `changes` | `path`, `status` (`added`, `modified`, `deleted`, `renamed` with `from`, `untracked`: still on disk but the rules leave it out), `set_changes` (for Live sets: tracks, devices, clips changed, one line each), `weight` (for Live sets, the biggest kind of change: `noise` a plugin re-saving its own state, `tidy` names/colors/order/groups, `mix`, `sound` devices, `arrangement` clips/notes/tracks/tempo) |
| `base` | in a workspace, when the branch it was made from has versions this branch hasn't: `branch` (its name) and `behind` (how many); `r3v merge <branch>` brings them in (`update` gets this branch's only). Also in `update --json` |
| `suggestions` | tool projects found in folders the rules don't cover: `r3v rules preset <folder> <preset>` |
| `unshared` | versions committed here (`commit --local`) the team's branch doesn't have: no one else, not even this computer's other copies of the project, sees them until a commit shares them. Absent without a team or when it can't be reached |
| `in_use` | files another program holds (Live writing a Freeze file): not read yet, they count as in the version you're on; run `status` again once they're free |
| `warnings` | worth a look before committing (never a refusal): `code`, and its fields; check `code` before using the others. Code `not_connected`: the project is on no team (`reason` says so, the other fields are empty). Code `looks_built`: a folder new in the changes looks like a build's output, named like a folder the rules leave out (`target-dx12/` beside `target/`) or holding 50 built files (`.obj`, `.pdb`, `.rlib`…) or 200 MB (then `folder` is where most of those are: `tools/out/`, not all of a new `tools/`); `folder`, `kind` (why: `name`, named like `like`, the ignored folder, e.g. `"Backup/"`; `built`, built files; `size`, new bytes), `reason` (the same in English, for people), `files`, `bytes`, `rule` (the line for `.r3v.yaml`'s `rules:`, e.g. `ignore: "target-*/"`) and `pattern` (its pattern) |

Committed, a folder stays in the team's storage for good (ignoring it
later only helps later versions). On a `looks_built` warning, run `r3v
rules ignore "<pattern>" --json` (its `pattern` field) before committing,
then `status` again, unless the user wants the folder kept. It adds the
warning's `rule` under `rules:` in `.r3v.yaml`, as the app's "Leave it
out" does; result `rule` (the line) and `added` (false: it was the last
rule already, nothing changed). It waits for the project as other writing
commands do (`--wait`).

`r3v agent setup --json`: `tool`, `model`, `files_written` (relative to
the folder; `[]`: it was set up already; `changed` is the same list, as
older R3Vs gave it), `identity` (`tool`, `model`, `file`: what versions
committed in this copy say when `R3V_ACTOR` doesn't) and `folder`. No
tool given and none R3V can tell: `usage`.

Reading commands (`status`, `log`, `diff`, `results`, `merge --preview`,
`branch`, `workspace list`, `rules check`/`explain`) don't wait for the app
or another command working on the project: they read it as it is, write
nothing, and add `"busy": true` and `holder` (who has it, as in
`project_busy` below) to their result. Commands that change the project
wait for it up to 10 seconds; `--wait SECONDS` (any command) waits longer
or less, and with a reading command waits that long for the lock before
reading without it.

A switch that stopped half-way (R3V closed or killed while `goto`,
`switch`, `update`, `merge` or `undo-merge` rewrote the files) leaves them
partly one version, partly another. The next command that gets the
project's lock (reading ones too; not one reading it busy) finishes or
rolls it back itself first, as the app's "put the files back" does, and
adds `recovered` to its result (or its `error`): `action` `restored` (the
files back as `version`, the version the project is on; `changes`: true
when your uncommitted changes are back too) or `completed` (a project with
no version yet got `version`), and `relinked`. The step that stopped isn't
redone: run it again. When it can't, the files stay as they are and the result
has `unfinished` instead: `version`, `code`, `message`, `hint`.
`set_open_in_live` or `files_open_in_tool`: ask the user to close it,
then any command puts them back. `changed_since_switch`: `paths` changed
after it stopped (putting the files back would replace them): ask the
user. Meanwhile `commit` refuses them (`switch_unfinished`).

`log BRANCH` lists another branch's versions (its key or name, as `r3v
branch` shows it; fetched from the team when they aren't here yet), in the
same shape. A branch name that isn't one fails with code `error` and, when
one is close, `suggestion` (its name: `lane/design` → `lane-design`); the
message says "did you mean ...?". The same goes for `switch`, `merge` and
`branch archive`.

`log` (`-n N` for the newest N): `versions`, newest first: `id`, `time`
(RFC 3339), `author`, `message`, `parents`, `branches` (team branches at
that version), `made_on` (the branch it was made on, for good; none on
versions from before R3V kept it), `actor` (only on a version an agent
made: `kind` (`agent`), `tool`, `model`, `session`, each when known).
`message` is the title, then a blank line and the details, if any.
Merging another branch always makes a version.

`log --first-parent` lists only the main line (from the newest version back
through first parents: a branch's own versions, each merge once); a merge
on it has `merged_count`, how many versions it brought in (text: "(+N
versions)"). With `log BRANCH` too.

`verify [--repair]` checks the history: `versions` (every version record
on this computer), `in_history` (of them, this branch's history: what
`log` lists), `other_versions` (the rest: other branches' or workspaces',
or R3V's own, as work kept through an update), `folder_lists`,
`local_files` and `local_bytes` (stored here, read again),
`team_only_files` (only in the team's storage), `folder_only_files`,
`team_checked` (`team_unreachable`: the team's storage couldn't be asked),
`problems` (`kind`, `id`, `path`, `detail`, `fixed`, `how`),
`open_problems` (not repaired) and `summary`. With problems not repaired
it fails, as the text output does (exit 1): code `verify_problems`, these
same fields in `error.details`. Once versions are shared R3V lets go of
its copies here of files the team's storage has (Live Sets and the rules
stay) and downloads one when a version needs it: few `local_files` with
many `team_only_files` is expected, not history lost.

`branch new NAME --agent` starts an agent's branch (its kind is `agent`,
its owner the member you work for; Nightly, on a team that keeps branch
names): `r3v branch` shows them marked.

`branch archive NAME` puts a branch away (versions stay; not main, not the
one you're on); `branch archived` lists them; `branch unarchive NAME` brings
one back; `branch delete NAME` deletes an archived branch for good (ask the
user first).

`commit`, `update` and `merge`: `action` (`published`, `local`,
`nothing-changed`, `up-to-date`, `ahead`, `fast-forward`, `merged`),
`saved` (the version committed, or null), `from`, `to`, `merged` (what a merge
took from each side), `relinked` (sample paths rewritten for this computer),
`reopen_sets` (a merge changed Live Sets under Live: the user must reopen them; never in a project
without sets, a Unity one say),
`restored` (`project restore` only: the project is back on its team).
`commit` also has `warnings`, as `status` has them, about what the
version just took (a commit is never refused for one), and `shared`:
`false` when the versions are on this computer only (`--local`, or a
project on no team: then a `not_connected` warning and `next`, the ways
to connect it and the commit that shares them once connected). `update` and
`merge` (and their `--preview`) have `unshared_elsewhere` when another
copy of the project on this computer (the main copy, a workspace) has
versions on that branch not shared yet (`commit --local`): they aren't on
the team's branch, so `up-to-date` or `ahead` doesn't mean they are in.
Each: `workspace` (its folder), `main_copy`, `branch`, `count`; each also
a warning in `warnings`: `code` `unshared_versions`, `reason`,
`workspace`, `branch`, `count`. Ask that copy's agent or the user to
commit there (it shares them), then merge again. `merge` also has
`same_files_as` (a version id) when the merge version's files are
exactly that version's: the branch hadn't moved since it parted.

`update --preview` and `merge <branch> --preview`: `action`, `versions`
(incoming; `wip`: true on one whose title says it isn't finished),
`changes`, `conflicts` (`key`, `file`, `unit`, `description`,
`can_keep_both`; a text file's hunk also `hunk`, `base`, `ours`,
`theirs`). `action` is `fast-forward` only for `update`: merging another
branch always makes a merge version (`merge`), even when the branch you
are on hasn't moved; then `same_files_as` is the incoming head, whose
files the merge version will have exactly. `merge --preview` also has
`message`: the merge version's description `merge` writes without `-m`,
from what comes in:

```
Merge lane/vfx: 12 versions
- Add bloom pass
- Tune shadow cascades
… and 4 more
```

(each incoming version's title, oldest first, merges and unfinished ones
left out, at most 10; nothing to list: `Merge branch lane/vfx`). `-m`
gives your own, used as it is.

`results [VERSION]` (Nightly): `version`, `results` (each check: `state`,
`note`, `time`, `actor`, `changes`). A merge version with no results of
its own whose files are exactly a parent's (`same_files_as`: a lane's
head merged into a branch that hadn't moved) shows that parent's
results, each with `from` (where it was recorded); results recorded for
the merge itself replace them.

`undo-merge VERSION` (Nightly) takes back what a merge brought in: a new
version on the current branch without it, the history kept (the merge
stays in it; don't rewrite anything). What changed since the merge stays;
where the same thing changed again it stops with `merge_conflict` as a
merge does, its choices named for what they do: `keep` (keep it as it is
now) and `undo` (as before the merge), in `--strategy keep|undo|both` and
`--resolve <key>=keep|undo|both`; each conflict lists them in `choices`
(`ours` and `theirs` still work: `keep` is `ours`, `undo` is `theirs`).
Uncommitted changes stop it (`unsaved_changes`), and so does a tool with
the project open; a version that isn't a merge is `not_a_merge`, one
already undone `nothing_to_undo`. Stopped half-way (R3V closed), running
it again finishes it. Result as `commit`'s. Ask the user before undoing a
teammate's merge.

`switch <branch>` and `goto <version|latest>`: `branch`, `to` (the version
the files are on), `older` (an older version), `relinked`. On the Nightly
channel, uncommitted changes don't stop them: they are **parked** on this
computer at the place they were made (a branch's latest, or an older
version) and come back when the project returns there. `parked` (the
changes left behind: `branch`, `at` (the older version, else none), `base`,
`files`, `bytes`, `since`), `restored` (that place's own changes, back;
`merged`: with the versions since), `waiting` (they need choices first:
still parked). `--force` discards instead. `parked` lists them (`parked`);
`parked bring <branch[@version]> [--strategy …]` merges a set into the
files here, `parked discard <branch[@version]>` drops one (ask the user
first). Tell the user what was parked and brought back.

`workspace` (Nightly; several agents on one project, each in a folder of
its own on a branch of its own; see [cli.md](cli.md#workspaces-several-agents-on-one-project-nightly)):

- `workspace add [<folder>] --branch NAME [--from BRANCH|VERSION] [--agent claude-code]`
  (no folder: `<project>.workspaces/<branch>` beside the project):
  `workspace` (`folder`, `branch` its key, `branch_name`, `base` the
  branch it was made from, `created`). Made from the files on this
  computer; what the rules leave out (`node_modules/`, `target/`) isn't
  there: install or build it in the workspace. Work there with the usual
  commands; `status` gives `main_copy` (the main copy's folder) in a
  workspace.
- `workspace list`: `main` (the main copy's folder), `workspaces`: each as
  above with `head`, `ahead` and `behind` (versions of its branch not in
  `base`, and of `base` not on it; absent when the team can't be reached),
  `unshared` (versions committed there `--local`, not on the team's
  branch: a merge here doesn't see them), `changes` (files changed, not
  committed), `parked`, `last_activity`,
  `missing` (the folder is gone: remove it from the list).
- `workspace remove <folder> [--force]`: `removed`. Refused
  (`workspace_not_merged`) while its versions aren't all merged into
  `base` (`r3v merge <branch>` in the main copy), or it has changes not
  committed or parked; `--force` loses them (ask the user first). Its
  branch stays on the team.

`project` (archiving a project for the whole team):
`project list`: `projects` (`id`, `name`, `archived`, `archived_by` a
member id, `archived_at`, `here` the project in this folder). `project
archive [--force]` and `project unarchive`: `project`, `id`, `archived`,
`changed` (false: it already was, or wasn't). Archiving hides the project
for everyone in the team (nothing deleted; the folder here stays); it's
refused with `unsaved_changes` or `unshared_versions` unless `--force`
(ask the user first), and on R3V Cloud teams with `not_admin` for anyone
but owners and admins. Archive or unarchive only when the user asks.

A project its team deleted (moved to another team, or deleted for
everyone, from another copy or computer) while this copy still points at
it: `commit` commits here and stops with `project_deleted`; nothing is
shared, nothing brought back. `error.deleted`: `project`, `team`, `time`,
`by` (a member's name), `moved_to` (the team it moved to), `moved_to_here`
(this computer has joined that team), `not_allowed`; `error.next` lists the ways out
as commands (as `not_connected` does). **Never pick a way
out on your own: tell the user what happened and ask.** Their choices:
`r3v remote --team "<moved_to>"` points this copy at the team it moved to
(only for a project its team no longer has, and that team has; the next
`commit` shares the versions there, merging what was shared there
meanwhile); `r3v project restore` brings it back to its team with its
history and shares the versions (on R3V Cloud teams owners and admins
only, while the team keeps it, 30 days; result as `commit`'s, `restored`:
true).
`status` gives `team.archived: true` for an archived project.

`section` (sections group the team's projects, the same for
everyone in the team): `section list` (or `section`): `sections` in the
team record's order (the app orders them per computer; `id`, `name`,
`projects`: `id`, `name`, `here`),
`ungrouped` (the projects in none) and `here` (the id of this project's
section, "" for none). `section new NAME`, `rename NAME NEW-NAME`, `delete
NAME` (its projects become ungrouped), `add NAME` (puts the project in this
folder in it, out of any other) and `remove` (takes it out of its
section): `section` (as it is now; none after `remove`), `project` and
`id` (add, remove), `changed` (false: it already was so). A section is
found by its name (case aside) or id: `no_section` when there's none,
`section_exists` when a new name is taken. On R3V Cloud teams only owners
and admins change sections (`not_admin`). Change them only when the user
asks. `status` gives `team.section` (its name).

`backup run [folder]` (back up the whole team's storage into a folder; only
adds): `team`, `kind` (`folder`, or `s3` for a bucket set up in the app),
`folder` (where), `run` (its record: `<folder>/runs/<run>.json`),
`keys`, `copied`, `copied_bytes`, `total_bytes`. Without a folder it uses
the one chosen in the app. The first run can copy many gigabytes.

`backup status`: `team`, `supported` (storage teams only), `kind` and
`folder` (where this computer backs up, `""` if nowhere), `last_success`, `last_attempt`, `error`,
`failing`, `members` (who backs the team up: `name`, `last_success`,
`failing`), `covered` (one of them did in the last 7 days).

`backup restore [folder | connection code] [--run TIME] [--preview]`: `team`, `from`,
`backup_of`, `run`, `runs` (newest first), `projects` (brought back whole:
`id`, `name`, `versions`), `branches`, `files`, `bytes`, `restored` (files
copied; `null` with `--preview`). It only adds to the team's storage; still,
preview first and ask the user.

## Several agents on one project (Nightly)

For the person setting it up: [agents-quickstart.md](agents-quickstart.md).
The project's `AGENTS.md` (`r3v agent setup` in the main copy, or the
app's Agents tab) tells every agent the essentials.

Each agent works in a workspace of its own (another folder with the same
project, on its own branch). Files that can't be merged (textures,
models, sounds, `.blend`, Unity and Unreal assets) are changed by one
workspace at a time; code is never locked (it merges hunk by hunk).

- `r3v workspace add --branch NAME --agent TOOL` makes a workspace set
  up for that tool (or `r3v agent setup` in one): its identity in its
  `.r3v/agent.json`; for Claude Code, its hook and `R3V_ACTOR` in
  `.claude/settings.local.json` (that copy's, never tracked). The hook
  (Claude Code's): the
  first time you change such a file it is locked for your workspace; if
  another holds it, the change is refused with who holds it. **Don't
  wait or retry: go on with other work** and tell the user.
- Files the user changed in their own copy and hasn't committed are
  theirs: refused too.
- Lock ahead, all or none: `r3v lock <paths…> --json` (a folder covers
  what's under it; too wide is warned about). `r3v unlock <paths…>` frees
  yours. A lock protects work not committed, and work not merged yet:
  in a workspace, yours are freed when your branch is merged; on the
  branch the workspaces merge into (the main copy's, main), a commit
  frees the locks on what it committed (folder locks stay until
  unlocked).
- Working on code someone else might take too: `r3v lock src/renderer/
  --claim` tells the other agents; it blocks nothing. `r3v lock` on a
  file that never locks (code) claims it instead (`claimed_instead`, and a
  warning): a lock there would stop no one.
- `r3v locks --json`: `locks` (`path`, `kind` `lock`|`claim`,
  `workspace`, `branch`, `actor`, `since`, `heartbeat`, `stale`, `person`
  (the user's uncommitted change), `mine`). A lock not heard of for 15
  minutes is `stale`; the next lock over it takes it. A lock taken over
  also has `reason` and `taken_from` (who held it).
- `r3v lock --take <paths…> --reason "<why>" --json` takes over others'
  locks before they are stale. Ask the user first, as for `--break`: only
  when they say the holder is gone, and give their reason. Otherwise wait
  for the lock to go stale. Files the user changed stay theirs.
- `r3v lock --break <path>` is the user's: never break a lock on your own.
- `lock --json` result: `taken`, `others` (others' claims over the paths),
  `taken_over` (locks replaced: stale ones, or with `--take` any),
  `claimed_instead` (files claimed as they never lock), `warnings`.

## Errors

| exit | code | what to do |
|---|---|---|
| 2 | `usage` | fix the command (`r3v help`) |
| 3 | `merge_conflict` | read `conflicts` (decide with the user); run again with `--resolve <key>=<choice>`, `--decisions`, `--resolved <path>=<file>` or `--strategy ours\|theirs\|both` |
| 4 | `set_open_in_live` | ask the user to save and close `set` in Live, then run again |
| 4 | `files_open_in_tool` | another tool (the Unity, Unreal or Godot editor, a design app) has the project open: ask the user to save and close it there, then run again |
| 5 | `team_unreachable` | network or storage down: try again later |
| 5 | `project_busy` | the R3V app or another command is using the project: try again in a moment, or with `--wait SECONDS`. `holder`, when it said: `program` (`app`\|`cli`), `command`, `pid`, `since` (RFC 3339), `running` (false: that process ended, and whatever holds it now didn't say: an older R3V) |
| 6 | `not_a_project` | not in a tracked folder: `r3v init`, or `cd` into the project |
| 6 | `newer_version_needed` | the team uses a newer R3V: the user must update |
| 6 | `needs_nightly` | a Unity/Unreal/Godot (…) project: the user must switch R3V to the Nightly channel |
| 6 | `team_needs_features` | the team turned on features this R3V lacks: update, or switch to Nightly |
| 1 | `unsaved_changes` | commit first (`r3v commit -m ...`); `update` keeps them unless you also have versions not shared |
| 1 | `on_older_version` | on an older version: `r3v goto latest` |
| 1 | `switch_unfinished` | a switch stopped half-way and its files aren't put back (see `unfinished`): nothing was committed. Run any command to let R3V finish or roll back the step; if it reports `changed_since_switch`, ask the user, then `r3v goto <version> --force` (replacing those files) or the app's offer |
| 1 | `unshared_versions` | `r3v commit -m ...` shares them. (Also a warning's code, never a refusal, in `merge` and `update`: another copy here has versions not shared) |
| 1 | `changes_parked_here` | changes parked here are waiting (they need choices): `r3v parked bring …` or `discard` them before leaving with new changes |
| 1 | `files_locked` | the team uses file locks and someone else holds files the versions change (`locks`: `path`, `member_id`); the versions stay committed here: tell the user, `commit` again once they are unlocked |
| 1 | `files_locked_by_agent` | another copy of the project on this computer (an agent's workspace, or the project's main copy) holds files the commit or `lock` needs (`locks`: `path`, `holder` `agent`\|`person`, `workspace`, `branch`, `actor`, `folder` (that copy's), `main_copy` (true: the project's main copy holds it, for work there not committed yet: a commit there frees a file's lock)); the message says who and where. Nothing was committed or locked: leave those files, or ask the user |
| 1 | `no_write_access` | R3V may not write `path` (a sandbox, a read-only folder): no one holds it, **don't retry**. When it is R3V's settings folder (`%APPDATA%\R3V`, the hint says so), set `R3V_CONFIG_DIR` to a folder you can write (see [Good to know](#good-to-know)) or ask the user for access; in the project, ask the user |
| 1 | `cloud_quota` | the team's storage on R3V Cloud is full: the versions stay committed here; tell the user (they delete projects or versions, or get more room), `commit` again later |
| 1 | `cloud_beta_only` | R3V Cloud is in a closed beta: making a team needs an invitation; tell the user (the waitlist: https://r3v.so/waitlist) |
| 1 | `cloud_one_team` | the beta gives one team of the user's own, and it exists: use it |
| 1 | `not_admin` | on an R3V Cloud team only its owners and admins do this (archiving a project, changing sections): tell the user |
| 1 | `no_section` | the team has no section of that name: `r3v section list` |
| 1 | `section_exists` | another section has that name: pick another, or use that one |
| 1 | `not_a_merge` | `undo-merge` takes a merge version: `r3v log --first-parent` lists them |
| 1 | `nothing_to_discard` | `discard <path>...`: nothing changed in those paths |
| 1 | `nothing_to_undo` | what that merge brought in is already gone from the branch: nothing to do |
| 1 | `not_connected` | the project is not in a team (`commit` still commits on this computer): `next` lists the ways to connect it; ask the user which team |
| 1 | `files_not_here` | an old version's files are only in the team's storage |
| 1 | `project_deleted` | the team deleted the project (moved it to another team, or deleted it for everyone); the versions stay committed here. Read `deleted` and ask the user: never point the copy elsewhere or bring the project back on your own (see "A project its team deleted") |
| 1 | `not_downloaded` | a download was cut off (no version here yet): `r3v update`, or `r3v clone` into the same folder, finishes it |
| 1 | `workspace_not_merged` | the workspace has versions not merged into its base branch, or changes not committed or parked: merge it (`r3v merge <branch>` in the main copy) and commit; `--force` loses its work (ask the user) |
| 1 | `in_workspace` | only the project's main copy does this (connecting it to a team, moving it): run it there |
| 1 | `workspace_orphaned` | the project's main copy was moved or deleted: the workspace can't be used; make it again |
| 1 | `verify_problems` | `verify` found problems it didn't repair: `details` holds its result (`problems`, `open_problems`, `summary`, …). Show the user `details.problems`; `verify --repair` brings back what can be (ask the user first) |
| 1 | `backup_folder_missing` | the backup folder isn't there: ask the user to connect the drive |
| 1 | `backup_folder_not_empty` | `backup run` needs an empty folder or this team's backup |
| 1 | `not_a_backup` | the folder holds no R3V backup |
| 1 | `no_such_run` | `--run` names no run of that backup (`--preview` lists them) |
| 1 | `backup_folder_taken` | the folder holds another team's backup |
| 1 | `error` | anything else: show `message` to the user. A branch that isn't one: `suggestion`, the closest branch's name, when one is close |

Codes are stable; messages are for people and may change.

## Good to know

- R3V tracks whole projects, including big binary files (audio, Unreal
  levels). Saving can upload a lot the first time; later only what changed.
- Live sets (`.als`) merge track by track; other files merge only when one
  side changed them.
- `r3v merge-files <base> <ours> <theirs> -o <out>` merges three files on
  disk the way a branch merge would (the project's rules for `out`, else
  the presets' by its name; outside a project, a file they don't know by
  its content: text line by line): a scene or a model changed on both
  sides is a `merge_conflict`, not lines mixed. `--json` `handler` says how it merged
  (`text`, `whole`, `unity-yaml`, …); `--as text` merges line by line
  anyway, only when the user says the file can be.
- `r3v status` shows what changed inside a modified set
  (`set_changes`); `r3v diff` shows every changed file the way it
  reads (`--version V`, `--from A --to B`), and `r3v diff a.als b.als` compares
  two sets on disk.
- `r3v watch` keeps running and reports teammates' new versions; it is
  for people (no `--json`).
- R3V keeps its own settings (teams joined, their keys) in
  `%APPDATA%\R3V`. In a sandbox that can't write there, commands that
  change them (`init --team`, `remote --team`, joining a team) fail with
  `no_write_access`: set `R3V_CONFIG_DIR` to a folder you may write (in
  your workspace, outside the project) and ask the user to copy
  `teams.json` into it, or to give R3V access. Never retry on
  `no_write_access`; `project_busy` alone means someone is working.
- The CLI is `r3v` on PATH after installing the app (a new terminal sees
  it); otherwise `%LOCALAPPDATA%\Programs\R3V\bin\r3v.exe` (or
  `R3V Pro\bin`). `where r3v` shows which one runs.
