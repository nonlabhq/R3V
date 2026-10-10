# R3V for AI agents

How an AI agent (Claude Code, Codex, Cursor…) uses R3V in a user's
project. Also in the CLI: `r3v help agents` (this page, for the installed
version) and `r3v help agents --snippet` (a few lines for a project's
`AGENTS.md` or `CLAUDE.md`).

R3V is version control for creative projects: Ableton Live sets, Unity,
Unreal and Godot projects. A project folder with a `.r3v` folder is tracked.
**`commit` makes a version and shares it with the team in one step**; there
is no separate push. (`save` and `snapshot`, its names before 0.1.33, still
work.) Coming from Git: [git-users.md](git-users.md); the words:
[glossary.md](glossary.md).

## Rules

- **Use `--json`** with `status`, `log`, `commit`, `update`, `merge`, `switch`, `checkout`, `parked` and `backup`, and
  read the result, not the text.
- **Commands never wait for an answer.** When one is needed they stop with
  an error code (below). Never pipe answers into R3V.
- **Don't use `--force` on your own.** It makes R3V rewrite sets while
  Ableton Live may have them open, or discard changes. Ask the user.
- **Never print connection codes** (`r3v connection-code`, `r3v
  remote <code>`, `r3v clone <code>`): they contain the team's storage
  keys. Don't put them in files, logs or messages.
- **Don't edit `.r3v/`.** It is R3V's own data. `.r3v.yaml` (the
  project's rules: what is tracked) is fine to read and edit; `r3v profile
  check` validates it.
- Version ids are long hex strings; any unique prefix (10 characters, as
  the text output shows) works where a command takes one.

## If you know Git

- One command commits and shares (`commit`); there is no staging, push,
  rebase or force push.
- A conflict is a track of a Live set or a whole file, never markers in a
  file: decide with the user, then `--strategy ours|theirs|both`.
- `update` never runs by itself, and keeps uncommitted changes.
- Every version keeps the branch it was made on (`log --json`:
  `made_on`); merging another branch always makes a version.
- Switching with uncommitted changes parks them (Nightly); they come back
  on return (`switch --json`: `parked`, `restored`).
- Branches are archived (`branch archive`), not deleted; only an archived
  branch is deleted for good (`branch delete`: ask the user).

## Everyday flow

```sh
r3v status --json            # what changed; is the team ahead?
r3v update --preview --json  # what the team shared: versions, changes, conflicts
r3v commit -m "Brighter mix on the chorus" --json
```

`commit` merges what teammates shared in the meantime before sharing. When the
same track (or file) changed on both sides it stops with `merge_conflict`:

```sh
r3v update --preview --json  # result.conflicts: file, unit (track), description
# decide with the user, then:
r3v commit -m "..." --strategy theirs --json
```

Strategies: `ours` (keep this computer's), `theirs` (keep the team's),
`both` (keep both: the track twice, or the file under a new name, when
`can_keep_both`). A strategy applies to every conflict of that run.

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
| `on_older_version`, `latest` | an older version is checked out; `r3v checkout latest` goes back |
| `team` | `null` without a team, else `reachable`, `incoming` (others shared versions: update), `error` |
| `changes` | `path`, `status` (`added`, `modified`, `deleted`, `renamed` with `from`, `untracked`: still on disk but the rules leave it out), `set_changes` (for Live sets: tracks, devices, clips changed, one line each), `weight` (for Live sets, the biggest kind of change: `noise` a plugin re-saving its own state, `tidy` names/colors/order/groups, `mix`, `sound` devices, `arrangement` clips/notes/tracks/tempo) |
| `suggestions` | tool projects found in folders the rules don't cover: `r3v profile preset <folder> <preset>` |
| `in_use` | files another program holds (Live writing a Freeze file): not read yet, they count as in the version you're on; run `status` again once they're free |

`log` (`-n N` for the newest N): `versions`, newest first: `id`, `time`
(RFC 3339), `author`, `message`, `parents`, `branches` (team branches at
that version), `made_on` (the branch it was made on, for good; none on
versions from before R3V kept it). Merging another branch always makes a
version.

`branch archive NAME` puts a branch away (versions stay; not main, not the
one you're on); `branch archived` lists them; `branch unarchive NAME` brings
one back; `branch delete NAME` deletes an archived branch for good (ask the
user first).

`commit`, `update` and `merge`: `action` (`published`, `local`,
`nothing-changed`, `up-to-date`, `ahead`, `fast-forward`, `merged`),
`saved` (the version committed, or null), `from`, `to`, `merged` (what a merge
took from each side), `relinked` (sample paths rewritten for this computer),
`reopen_sets` (sets changed under Live: the user must reopen them).

`update --preview` and `merge <branch> --preview`: `action`, `versions`
(incoming), `changes`, `conflicts` (`key`, `file`, `unit`, `description`,
`can_keep_both`).

`switch <branch>` and `checkout <version|latest>`: `branch`, `to` (the version
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

## Errors

| exit | code | what to do |
|---|---|---|
| 2 | `usage` | fix the command (`r3v help`) |
| 3 | `merge_conflict` | show `conflicts` to the user; run again with `--strategy ours\|theirs\|both` |
| 4 | `set_open_in_live` | ask the user to save and close `set` in Live, then run again |
| 5 | `team_unreachable` | network or storage down: try again later |
| 5 | `project_busy` | the R3V app or another command is using the project: try again in a moment |
| 6 | `not_a_project` | not in a tracked folder: `r3v init`, or `cd` into the project |
| 6 | `newer_version_needed` | the team uses a newer R3V: the user must update |
| 6 | `needs_nightly` | a Unity/Unreal/Godot (…) project: the user must switch R3V to the Nightly channel |
| 6 | `team_needs_features` | the team turned on features this R3V lacks: update, or switch to Nightly |
| 1 | `unsaved_changes` | commit first (`r3v commit -m ...`); `update` keeps them unless you also have versions not shared |
| 1 | `on_older_version` | an older version is checked out: `r3v checkout latest` |
| 1 | `unshared_versions` | `r3v commit -m ...` shares them |
| 1 | `changes_parked_here` | changes parked here are waiting (they need choices): `r3v parked bring …` or `discard` them before leaving with new changes |
| 1 | `files_locked` | the team uses file locks and someone else holds files the versions change (`locks`: `path`, `member_id`); the versions stay committed here: tell the user, `commit` again once they are unlocked |
| 1 | `not_connected` | the project is not in a team (`commit` still commits on this computer) |
| 1 | `files_not_here` | an old version's files are only in the team's storage |
| 1 | `not_downloaded` | a download was cut off (no version here yet): `r3v update`, or `r3v clone` into the same folder, finishes it |
| 1 | `backup_folder_missing` | the backup folder isn't there: ask the user to connect the drive |
| 1 | `backup_folder_not_empty` | `backup run` needs an empty folder or this team's backup |
| 1 | `not_a_backup` | the folder holds no R3V backup |
| 1 | `no_such_run` | `--run` names no run of that backup (`--preview` lists them) |
| 1 | `backup_folder_taken` | the folder holds another team's backup |
| 1 | `error` | anything else: show `message` to the user |

Codes are stable; messages are for people and may change.

## Good to know

- R3V tracks whole projects, including big binary files (audio, Unreal
  levels). Saving can upload a lot the first time; later only what changed.
- Live sets (`.als`) merge track by track; other files merge only when one
  side changed them.
- `r3v status` shows what changed inside a modified set
  (`set_changes`); `r3v diff a.als b.als` compares two sets.
- `r3v watch` keeps running and reports teammates' new versions; it is
  for people (no `--json`).
- The CLI is `r3v` on PATH after installing the app (a new terminal sees
  it); otherwise `%LOCALAPPDATA%\Programs\R3V\bin\r3v.exe` (or
  `R3V Pro\bin`). `where r3v` shows which one runs.
