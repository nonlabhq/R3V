# DaVinci Resolve projects

Status: parked (2026-10-07). Designed (in place, as a plugin), not built.
Findings from Resolve Studio 20.2.2 on Windows 11, October 2026; Resolve
21.1 is current.

## Parked

Video isn't where R3V should spend its effort now; music, games and 3D are.

- What hurts video teams most is footage (terabytes to store, move and
  stream), relinking, and several people on one project. Footage never
  changes once shot, so it needs storage, not versions; the rest is
  covered by fixed drive letters, Mapped Mount, Blackmagic Cloud (US$5 a
  month a library) and PostgreSQL.
- What R3V could do is narrow: versions and hand-over of a ~3 MB project
  for teams of one to five on disk libraries. They already duplicate
  timelines inside the project (v01, v02…), export `.drp`s, or use
  Resolve's Project Backups; R3V adds history, own storage and working
  offline, without the merge that sets it apart for Live.
- The cost is high for one program: five new plugin hooks, stills outside
  the project folder, a table of Resolve versions. And editors use Macs a
  lot; R3V is Windows only.

Look again when one of these holds: the plugin interface gets built for
another kind of project anyway; R3V runs on macOS; several users ask.
The research and experiments below stay valid for Resolve 18–20.

## Why

Resolve keeps a project in its own database, not in a file next to the
footage. Backing one up, moving it to another computer or going back to
last week's edit means exporting a `.drp` by hand (and remembering to), or
copying folders out of `%APPDATA%` without knowing which. Syncing the
database folder with Dropbox and the like corrupts it (Blackmagic says not
to). Blackmagic Cloud shares a live project but keeps no history, and the
project lives on their servers.

R3V can keep versions of a Resolve project, on the team's own storage, and
bring any of them back on any computer.

## Scope

In:

- Saving versions of whole projects, as they are, and taking any one back.
- Moving a project to another computer (or a teammate) through the team.
- Saying what a version needs that isn't there (footage, a newer Resolve).

Out, on purpose:

- Diffs or merges of timelines. The structure is readable (see below), but
  grades and effects aren't, and a wrong merge breaks a project quietly.
  Two people editing one project take turns.
- Changing anything inside a project (relinking footage, renaming). R3V
  copies Resolve's files whole; Resolve's own Relink and Mapped Mount
  handle footage paths.
- Footage. It is versioned like any big file only if it's in the R3V
  project's folder; most footage lives elsewhere and stays there.
- PostgreSQL and Blackmagic Cloud libraries.

## How Resolve stores a project

Resolve 18 and later. A *project library* (formerly *database*) is a
folder; the libraries Resolve knows are listed in
`%APPDATA%\Blackmagic Design\DaVinci Resolve\Preferences\dblist.conf`
(`Local Database:C\Users\…\Resolve Project Library:*:::DISK`). The default
one is `%APPDATA%\…\Support\Resolve Project Library`; Project Manager →
Project Libraries → Add makes one in any folder.

```
<library>/Resolve Projects/
  SoundLib.db, Settings/*.xml              library-wide
  Users/guest/
    User.db, Configs/, ProjectMetadataCache/Metadata.db
    Projects/<project name>/
      Project.db                           the project
      Batch Renders/<uuid>.xml             the render queue
```

`Project.db` is SQLite (about 3 MB, `journal_mode=delete`, no WAL), one per
project. Its schema is Resolve's PostgreSQL one (`uuid`,
`character varying`): `SM_Project` (one row: `SM_Project_id`,
`ProjectName`, `ProjectVersion`, `LastModTimeInSecs`), `Sm2Timeline` →
`Sm2Sequence` → `Sm2SequenceContainer` → `Sm2TiTrack` → `Sm2TiItem` (clip
name, start, duration, `MediaFilePath`), `Sm2MpFolder`/`Sm2MpMedia` (the
media pool), `database_upgrade_log`. Blobs are `0x81` followed by a zstd
frame, sometimes after an 8-byte header: grades and effects are protobuf
without a published schema, clip metadata Qt `QDataStream` maps.

Resolve holds `Project.db` open exactly while that project is open, and
writes it in place (a `Project.db-journal` appears for the moment of a
write). Other programs can read it meanwhile. Each library also has
`ProjectMetadataCache/Metadata.db`, a cache of what Project Manager shows,
keyed by folder name; Resolve brings it up to date by itself.

Project Manager lists a project by its **folder** name, not `ProjectName`.
`Preferences/activedb.conf` names the library in use; `recentprojects.conf`
the recent projects.

Outside the library, under the first Media Storage folder, or wherever the
project's Working Folders (Project Settings → Master Settings) put them,
named by `SM_Project_id`:

| Folder | Holds | Keep? |
|---|---|---|
| `.gallery/<id>/` | Gallery stills (DPX) and `Info.txt` naming the project | yes |
| `CacheClip/<id>/` | render cache | no, Resolve makes it again |
| `Resolve Project Backups/<id>/` | Resolve's own timed backups | no |
| `ProxyMedia/`, `OptimizedMedia/` | proxies | no |

Users also save renders into the project's folder in the library (one here
had 1.6 GB of `.mp4`/`.mov` in `Export/`). Resolve ignores files it didn't
make there.

Footage is referenced by absolute path (`D:\_Video Proj\…`).

### Versions of Resolve

Opening a project in a newer Resolve upgrades it, one way: a project opened
in 21.1 no longer opens in 20.x. Resolve doesn't warn per project. A
library can hold projects of several versions.

### Exports

- `.drp`: one project, a zip; stills and LUTs optional; no footage.
- `.dra`: an archive *folder*, a `.drp` with the footage.
- `.drt` (timeline), `.drb` (bin), `.drx` (grade still).

Only Resolve writes them. Scripting can (`ProjectManager.ExportProject`),
but external scripting needs Studio (Preferences → System → General →
External scripting using: Local), and from 21.1 Python scripting in any
form does. R3V can't count on it.

## Design: in place, as a plugin

Decided 2026-10-07. R3V tracks a Resolve project where Resolve keeps it,
and the Resolve-specific logic lives in a plugin (below). R3V reads and
copies Resolve's files itself; it never asks Resolve to export, so Free
works, and nothing inside a project is ever changed.

### What a version holds

The R3V project's folder is the Resolve project's own folder in its
library, `<library>/Resolve Projects/Users/guest/Projects/<name>/`; `.r3v/`
goes in it (experiment 6). A version holds:

- `Project.db` and `Batch Renders/`.
- `Gallery/`: the project's stills, copied from `<gallery folder>/<id>/` by
  the plugin before each save, and back after a version is taken (below).
  The gallery folder is the project's Working Folders setting; each
  library's `ProjectMetadataCache/Metadata.db` has it (`galleryPath`), and
  empty means the first Media Storage folder.

Everything else in the folder is left out: renders and other files users
put there, `*-journal`. Caches, proxies and Resolve's own backups are
outside the folder anyway.

### Adding projects

The way in for projects a user already has, and for new ones (made in
Resolve as usual, then added):

- The plugin lists the projects in every disk library Resolve knows
  (`Preferences/dblist.conf`): name, library, size, last change, the
  Resolve that last wrote it, whether R3V tracks it already.
- The user ticks projects; each becomes an R3V project in place, and its
  first version is saved. Nothing moves; Resolve sees no difference.
- App: **Add from DaVinci Resolve…** next to adding a folder. CLI:
  `r3v resolve list`, `r3v resolve add <name>…` (`--json` like the rest).

### On another computer

Taking a project from the team puts its folder into this computer's
default library (`Local Database` in `dblist.conf`), or one the user picks,
and its stills into this computer's gallery folder. Resolve lists it the
next time Project Manager shows that library (experiment 1). No library to
connect, no Resolve setting to change.

A folder of that name already there:

- the same project (same `SM_Project_id`): it is that project; R3V takes it
  over like any untracked copy, without deleting anything;
- another project: R3V stops and asks for another name (the folder name is
  the name Resolve shows).

### Rejected: a library per project folder

The R3V project as an ordinary folder holding its own Resolve library
(stills and cache pointed into it through Working Folders) fits R3V's
"one project, one folder" best, but:

- every computer must connect every project's library in Resolve (Add
  Project Library → Connect), or R3V must edit `dblist.conf`;
- existing projects would have to move into new libraries, and their
  Working Folders still point at the old stills: changing that means
  editing the project, by hand in Resolve for each one.

The cost lands on exactly the users with many projects to bring in.

### Safety

Never lose a user's file:

- **Reading.** A copy is good only if the file didn't change while it was
  read: no `Project.db-journal`, same size and time before and after, and
  the copy opens as SQLite. Otherwise try again later.
- **Saving while the project is open.** With Live Save on (Preferences →
  User → Project Save and Load) every edit is on disk within a second;
  with it off, only what was last saved. R3V saves what's on disk, and
  says so when Live Save is off and the project is open.
- **Writing.** R3V replaces a project's files only while Resolve doesn't
  hold its `Project.db` (asked through the Restart Manager, which opens
  nothing). Resolve may stay running: a project replaced while closed opens
  with the new content (experiment 4). Otherwise it fails and says to
  close the project. Files are written beside and renamed into place.
- **One id, one project.** Two folders with the same `SM_Project_id` in
  reach of the same gallery folder share stills and cache. R3V never puts
  a second copy of a project next to the first; taking a version back
  replaces the project's own folder.
- **Newer Resolve.** Each version records the Resolve that last wrote the
  project (read from its `Project.db`). Taking back a version written by a
  newer Resolve than this computer's is refused, naming the version
  needed. Opening an older project in a newer Resolve is allowed, with a
  warning that it will no longer open in the older one.
- **Missing footage.** Like Live's missing samples, a yellow note listing
  the folders that aren't there; nothing is changed.
- **Resolve's own files.** R3V only reads `dblist.conf`, `Metadata.db` and
  the preferences; it writes nothing of Resolve's outside the project's
  folder and its stills.

## The plugin

Presets (`presets/`) describe a kind of project in YAML, with named Go
handlers for a few fixed points (`ext.RegisterRunning`, `RegisterOpener`,
`RegisterCheck`, `RegisterMerge`). Resolve needs more than that: its own
way of adding projects, a project that isn't one plain folder, and its
own rules around saving and taking versions. A **plugin** is a Go package,
built in the same way (`presets/resolve`, Nightly first), that registers a
preset and, on top of it, logic of its own through `ext`:

| What | Resolve uses it for | Exists? |
|---|---|---|
| Preset (detect, ignore, kinds) | recognise a project folder; leave out renders, journals | yes |
| Running check | Resolve holding `Project.db` | yes |
| Opener | start Resolve | yes |
| Pre-save checks (warnings) | Live Save off while the project is open | yes |
| Before a version is made | copy stills into `Gallery/`; wait for a stable `Project.db` | new |
| Before a version is applied (may refuse, with a reason) | newer Resolve; project open | new |
| After a version is applied | copy `Gallery/` out to the gallery folder | new |
| Where a project goes on this computer (clone, take from the team) | the default library's `Projects/` | new |
| Sources of projects to add (list, add) and their CLI and app entries | **Add from DaVinci Resolve…**, `r3v resolve …` | new |

The new points are general: Unity could refuse a version made by a newer
Editor (`ProjectVersion.txt`) through the same "before applied" hook. Each
is added when its first user needs it, not ahead of time. A plugin that
changes what a team stores is still a team feature (see
[channels.md](channels.md)).

## Open questions

- The gallery folder on another computer: when the project's Working
  Folders name a path that doesn't exist there (another user's
  `C:\_Video Proj\…`), what does Resolve do, and where should the plugin
  put the stills?
- Telling Resolve versions apart: `SM_Project.ProjectVersion` (14 for
  older projects, 15 for 20.2) and `database_upgrade_log` (stops at 18.1);
  needs a table across 18–21.
- Sizes: stills are DPX, about 8 MB each; whether a project's gallery
  grows enough to matter.

## Experiments

Resolve Studio 20.2.2, Windows 11, 2026-10-07, in throwaway libraries
under `D:\_ResolveLab`. A watcher logged every file change and which
processes held the databases (Restart Manager).

| # | Question | Result |
|---|---|---|
| 1 | A project copied into another library: listed, opened? | **Yes**, into a connected library (`Lab1` copied as `Lab1-moved`, listed as `Lab1-moved`: the folder name wins). A whole library folder copied and connected (**Connect**) lists and opens its projects. A bare `Resolve Projects/Users/guest/Projects/<x>/` without the library's `Settings/` and `User.db` isn't recognised. |
| 2 | A reliable sign that a project is open? | **Yes**: Resolve holds `Project.db` open from opening the project until going back to Project Manager, and not otherwise. `SM_Project.LockId` stays empty. |
| 3 | When does `Project.db` change? | Live Save on: within a second of each edit (a cut, a grabbed still). Off: only on Save. |
| 4 | `Project.db` replaced while Resolve runs, project closed? | Opens with the new content, no warning; Project Manager's cache catches up by itself. |
| 5 | Stills and cache in the project's own folder? | **Yes**: Working Folders → Gallery stills / Cache files location; stills land in `<that folder>/<id>/`. |
| 6 | Unknown files (`.r3v/`, a `.txt`) in a library or project folder? | Ignored, no warnings. |
