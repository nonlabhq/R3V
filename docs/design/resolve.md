# DaVinci Resolve projects

Status: researching. Findings from Resolve 20.2 (Free) on Windows, October
2026; Resolve 21.1 is current.

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
without a published schema, clip metadata Qt `QDataStream` maps. Resolve
running doesn't stop another program reading the file.

Outside the library, under the first Media Storage folder (or the
project's Working Folders), named by `SM_Project_id`:

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
but external scripting needs Studio, and from 21.1 Python scripting in any
form does. R3V can't count on it.

## Design (to be settled by the experiments)

R3V reads and copies Resolve's files itself; it never asks Resolve to
export, so Free works.

What a version of a Resolve project holds: `Project.db`, `Batch Renders/`,
and the project's `.gallery/<id>/`. Left out: caches, Resolve's backups,
renders, `*-journal`.

Two ways to give it a folder, decided by experiments 1 and 4:

- **A. In place.** R3V lists the projects in the libraries Resolve knows;
  adding one tracks its folder where it is, the gallery folder alongside.
  Nothing to set up in Resolve. Taking a project to another computer
  puts it into that computer's default library.
- **B. A library per project folder.** The R3V project is an ordinary folder
  (`D:\Work\Trailer\`) holding a Resolve library and the project's Working
  Folders, and the footage if the user keeps it there. Everything about the
  project is in one place, like any other R3V project; Resolve needs the
  library added once per computer.

### Safety

Never lose a user's file:

- **Reading.** A copy is good only if the file didn't change while it was
  read: no `Project.db-journal`, same size and time before and after, and
  the copy opens as SQLite. Otherwise try again later.
- **Saving while Resolve is open.** What's on disk may lag what's on
  screen (experiment 3). R3V saves what's on disk and says so; it doesn't
  pretend to have the latest edit.
- **Writing.** R3V replaces a project's files only when Resolve isn't
  running, or (if experiment 2 finds a reliable sign) when that project
  isn't open. Otherwise it fails and says to close Resolve.
- **Newer Resolve.** Each version records the Resolve that last wrote the
  project. Taking back a version written by a newer Resolve than this
  computer's is refused, naming the version needed. Opening an older
  project in a newer Resolve is allowed, with a warning that it will no
  longer open in the older one.
- **Missing footage.** Like Live's missing samples, a yellow note listing
  the folders that aren't there; nothing is changed.

## Experiments

Run on Resolve 20.2 (Free), Windows 11, with a throwaway project in a
throwaway library.

| # | Question | Result |
|---|---|---|
| 1 | A project folder copied into another library's `Projects/`: does Resolve list it and open it? Does the folder name or `ProjectName` win? | |
| 2 | While a project is open, is there a reliable sign of it on disk (handle on `Project.db`, a lock row, `recentprojects.conf`)? | |
| 3 | When does `Project.db` change: on every edit, on save, with Live Save on and off? | |
| 4 | `Project.db` replaced while Resolve runs and the project is closed: does opening it show the new content, or a cached one (`ProjectMetadataCache`)? | |
| 5 | Can a project's gallery and cache live in its own folder (Working Folders)? | |
| 6 | Does a `.r3v` folder (or other unknown files) in a library or project folder bother Resolve? | |
