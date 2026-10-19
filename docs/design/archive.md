# Archiving a project

Status: agreed (2026-10-19), built; in every build since 0.2.2.

A project the team is done with, put away for everyone in the team: out of
everyone's lists, never deleted. Its versions and stored files stay, and it
comes back whole with Unarchive. Like archiving a branch, for a project.

## Who

- **R3V Cloud teams:** owners and admins (as Workflow settings). The
  service refuses the write from anyone else; the app and the CLI check
  first and say so (`remote.ErrNotAdmin`, CLI `not_admin`).
- **Teams on their own storage:** anyone in the team.

## Where it's kept

One team-level record, `archived.json` beside `team.json`:

    {"projects": {"<project id>": {"by": "<member id>", "at": "<RFC3339>"}}}

Not a field of the project's record (`project.json`): an older R3V rewriting
that record (a rename, a colour) would drop it. Both levels keep fields a
newer R3V adds (jsonx.Extra).

It's written whole with a conditional write: if-match the etag read, or
if-none-match when there's none. When someone else wrote it meanwhile, it's
read again and the change made again (a few times, then an error). A
record that doesn't read is an error, never written over (that would bring
every archived project back). On R3V Cloud it goes through the generic key
store (`/keys/archived.json`); the Worker lets only owners and admins write
unnamed team-level keys.

The team log records `project-archived` and `project-unarchived`, after the
write, as well as it can; Team settings › Activity shows them.

### Older R3Vs

An older R3V never reads the record: it shows the project as before, and
can open and share it. Nothing it does loses or misreads anything, so it's
not a team feature.

## On this computer

- The folder stays as it is.
- The app stops watching an archived project: no polling, no notices, no
  uploads before commits. It reads the record with the team's projects
  (each time the team's list is read) and keeps it as last read
  (`team-archived.json` in the app's settings), so it knows at start what
  not to watch, and follows a teammate's archiving or unarchiving the next
  time it reads the team. `r3v watch` doesn't watch one either.
- On an R3V Cloud team, archiving and Unarchive are offered only to
  owners and admins (the app knows the role once it has asked the team).
- It leaves the left panel. The team's home lists it under
  **Archived (N)**, folded by default, with Unarchive; from there it can
  still be opened, or its settings.
- Archiving asks first when the copy here has uncommitted changes or
  versions the team hasn't got, says which, and is refused unless the
  person confirms (CLI: `unsaved_changes` / `unshared_versions` unless
  `--force`).

## Where it's done

- The left panel's project menu: **Archive project…**
- Project settings › General, by the danger zone: **Archive project…**, or
  **Unarchive** when it is.
- The confirmation says it in plain words: hidden for everyone in the team,
  nothing deleted, can be brought back.
- CLI: `r3v project list | archive [--force] | unarchive` in the project's
  folder (`--json`); `status` gives `team.archived`.

## Stopped half-way

Archiving is one conditional write of one key: stopped before it lands,
nothing changed; after, it's archived (the team log line may be missing).
This computer's copy of the record is written after, whole then renamed;
missing, the next read of the team puts it right.
