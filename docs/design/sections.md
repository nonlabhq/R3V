# Sections

Status: agreed (2026-10-19), built behind Nightly.

One level of named groups of a team's projects (Game, Engine, Tools), the
same for everyone in the team. A project is in one section at most; those
in none are ungrouped and come first. Nothing about the projects changes:
a section only says where they show.

## Who

- **R3V Cloud teams:** owners and admins change the sections (make,
  rename, move, delete, put projects in). The service refuses anyone
  else's write; the app and the CLI check first (`remote.ErrNotAdmin`, CLI
  `not_admin`). Everyone sees them: a collaborator gets a copy without the
  projects they can't reach, and without sections left empty by that.
- **Teams on their own storage:** anyone in the team.

## Where it's kept

One team-level record, `sections.json` beside `team.json`:

    {"sections": [{"id": "<16 hex>", "name": "Engine", "projects": ["<project id>", ...]}]}

The array's order is the sections' order, team-wide. The order of the
projects inside a section is each person's own (the left panel's drag
order, kept on their computer), not the record's. Not a field of the
project's record (`project.json`): an older R3V rewriting that would drop
it. Both levels keep fields a newer R3V adds (jsonx.Extra), say a section's
look later.

It's written whole with a conditional write, as `archived.json` is
(archive.md): if-match the etag read, or if-none-match when there's none;
when someone else wrote it meanwhile it's read again and the change made
again. A record that doesn't read is an error, never written over. A
project listed in two sections (two writes crossing, another tool) counts
in the first. On R3V Cloud it goes through the generic key store
(`/keys/sections.json`), written by owners and admins only.

Names are 1 to 60 characters, unique in the team (case aside: the CLI
finds a section by its name). Deleting a section leaves its projects
ungrouped. An archived project keeps its section, shown only once it's
back; meanwhile it's under Archived in the team's home.

The team log records one kind, `sections-changed`, with `section` (its
name then) and `change`: `made`, `renamed` (`name`: before), `deleted`,
`moved`, `project-in`, `project-out` (`project`, `name`: the project's);
Activity shows them.

### Older R3Vs

An older R3V never reads the record and shows the projects ungrouped, as
before. Nothing it does loses or misreads anything: not a team feature.

## On this computer

- The app reads the record with the team's projects (each time the team's
  list is read: the same path as archived projects, so a push from R3V
  Cloud that refreshes the list refreshes the sections too) and keeps it
  as last read (`team-sections.json` in the app's settings), for the
  first draw and while the team can't be reached.
- The left panel: the ungrouped projects, then each section under a header
  that folds it (folded or not is kept on this computer; the project shown
  stays visible). Those who may drag a project onto a section's header or
  list, or back among the ungrouped, and change sections from a header's
  right-click (or ⋯): Rename…, Move up, Move down, Delete section…, New
  section…; New section… is also in the team's menu, and on the Projects
  band's right-click.
- The team's home groups its cards the same way (folded alike).
- Project settings › General: a Section picker (the section's name alone
  for those who may not change it).
- The project's header: `TEAM / Section` when it's in one.
- CLI: `r3v section list | new NAME | rename NAME NEW-NAME | delete NAME |
  add NAME | remove` (`add` and `remove`: the project in this folder;
  `--json`); `status` gives `team.section`.

## Stopped half-way

Each change is one conditional write of one key: stopped before it lands,
nothing changed; after, it's done (the team log line may be missing). This
computer's copy is written after, whole then renamed; missing or stale,
the next read of the team puts it right. A project dropped on a section
shows there at once, and goes back if the write fails.
