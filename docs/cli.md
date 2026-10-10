# Command line tool

The desktop app covers everyday use, including creating a team on Cloudflare R2. The command line tool `r3v.exe` is for a few advanced tasks, scripts and AI agents. It is installed in the `bin` folder of the R3V install folder (`%LOCALAPPDATA%\Programs\R3V\bin`), and the installer puts that folder on your PATH: open a new terminal and type `r3v`. Run `r3v help` for the full list.

**Scripts and AI agents:** `status`, `log`, `save`, `update`, `merge` and `version` take `--json` (one JSON object on stdout, errors with fixed codes), and no command ever waits for an answer. See [R3V for AI agents](agents.md), also printed by `r3v help agents`.

## Team storage (S3-compatible)

```
r3v connection-code --endpoint URL --bucket NAME --access-key K --secret-key S [--prefix P] [--name TEAM]
```

The same as **Create a team** in the app, for scripts: checks that the key can read and write the bucket (conditional writes included) and prints the connection code members paste into R3V. `--name` names the team for everyone. The code contains the key: send it privately.

## Everyday commands (inside an Ableton project folder)

```
r3v init [--author NAME]          # start tracking this project
r3v remote <connection code>      # connect it to the team's storage
r3v save -m "added drums"         # commit a version and share it (merges the team's versions first)
r3v update [--preview]            # get the team's latest versions (or just look)
r3v status                        # what changed since your last version
r3v log                           # versions
r3v clone <connection code> "Song" [folder]   # download a team project
r3v teams                         # teams this computer is connected to
```

`r3v watch` keeps running while you work and tells you when someone commits a new version on your branch. It never changes your files. The desktop app does the same in the background.

`save` and `update` merge Live Sets track by track. When you and a teammate changed the same track (or the same sample file) they stop and ask for `--strategy ours|theirs|both`. They refuse to rewrite sets while Ableton Live is running if the team's changes must be merged in.

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
name or its key.

Teams on storage keep a log of every branch move (each move is its own record, never changed). A branch moved by mistake can be put back: check out the version it was on and save from there.

## Older versions

```
r3v checkout <id|HEAD~N> [--force]   # put the project in the state of a version (samples relinked)
r3v checkout latest                  # back to the latest version
r3v export <id|HEAD~N> <folder>      # write a version as a separate project folder
r3v snapshot -m "message"            # commit a version on this computer only
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

## Working with Live Sets directly

```
r3v info <set.als>                          # tracks, devices, clips, automation, plugins, samples
r3v diff <a.als> <b.als>                    # semantic diff
r3v merge-sets <base> <ours> <theirs> -o out.als [--strategy fail|ours|theirs|both]
```
