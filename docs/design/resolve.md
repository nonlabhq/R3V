# DaVinci Resolve projects

Status: researching; experiments done. Findings from Resolve Studio 20.2.2
on Windows 11, October 2026; Resolve 21.1 is current.

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

## Design

R3V reads and copies Resolve's files itself; it never asks Resolve to
export, so Free works.

What a version of a Resolve project holds: `Project.db`, `Batch Renders/`,
and the project's stills (`<gallery folder>/<id>/`). Left out: caches,
Resolve's backups, renders and anything else users drop in the folder,
`*-journal`.

Both ways of giving it a folder work (experiments 1, 4, 6); to choose:

- **A. In place.** R3V lists the projects in the libraries Resolve knows
  (`dblist.conf`); adding one tracks its folder where it is, and its stills
  from the gallery folder. Nothing to set up in Resolve, and the projects
  a user already has can be added at once. On another computer, taking a
  project puts its folder into a library there (the default one unless
  chosen), and Resolve lists it.
- **B. A library per project folder.** The R3V project is an ordinary folder
  (`D:\Work\Trailer\`) holding a Resolve library (`Resolve Projects/…`,
  with its `Settings/` and `User.db`) and, through the project's Working
  Folders, its stills and cache; the footage too if the user keeps it
  there. Everything is in one place like any other R3V project. Resolve
  needs the library connected once per computer (Add Project Library →
  **Connect**, not Create: Create makes a new library in a subfolder of a
  folder that isn't empty).

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
  with the new content. Otherwise it fails and says to close the project.
  Files are written beside and renamed into place.
- **One id, one project.** Two folders with the same `SM_Project_id` in
  reach of the same gallery folder share stills and cache. R3V never puts
  a second copy of a project next to the first; taking a version back
  replaces the project's own folder.
- **Newer Resolve.** Each version records the Resolve that last wrote the
  project. Taking back a version written by a newer Resolve than this
  computer's is refused, naming the version needed. Opening an older
  project in a newer Resolve is allowed, with a warning that it will no
  longer open in the older one.
- **Missing footage.** Like Live's missing samples, a yellow note listing
  the folders that aren't there; nothing is changed.

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
