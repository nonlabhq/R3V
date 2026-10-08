# File locks

Files that can't be merged (an Unreal level, a Unity scene, a .blend, a
.psd) lose someone's work when two people change them at once: one of the
two versions has to go. A lock says, before the work starts, who is
changing a file; the others see it and wait. Live sets don't need it: R3V
merges them track by track.

Locks are an R3V Cloud feature, on the Nightly channel (built: the client
on `feat/locks`, the service on R3V-Cloud): only a service can say who
holds a file, at once and for everyone. A team on its own storage has
none.

## Off unless a team wants them

Many teams split their work so that two people never touch the same file;
for them locks are only in the way. Two levels, three states:

1. **File locking**, the team's switch, off at first. Off: no locks at
   all (no padlocks, no manual locks; the service checks nothing). On:
   anyone locks a file or folder by hand, and the service enforces locks
   on shares. When a hosted team gets a project of a kind that benefits
   (Unreal, Unity, Godot, Blender), the app offers its admins once to
   turn it on (dismissed: not offered again).
2. **Auto-lock**, only with file locking on: which kinds of file lock by
   themselves when they change, ticked in the team's settings (the
   defaults below). None ticked: manual locks only.

So: off | manual only | manual + auto-lock (the kinds chosen).

In team.json (admins write it): `"locks": {"on": bool, "kinds": [...]}`.
`on` is level 1, all the service reads; `kinds` is the auto-lock list
(ids: `unreal`, `unity`, `godot`, `blender`, `source-art`, `godot-text`,
`images`, `models`, `audio`, `video`; see `profile.LockKinds`). Turning it
on adds the team feature `locks`.

**A project's own**, in its `.r3v.yaml`, versioned with it. A project
only narrows the team's settings, never widens them: with the team's
locking off, `enabled: true` does nothing (the service reads only the
team's switch), and the app says so.

```yaml
file_locks:
  enabled: false          # this project: no locks at all
  auto_lock:              # adds to / removes from the team's auto-lock kinds
    add: ["Content/Maps/**"]
    remove: ["*.uasset"]
  # or: auto_lock: off    # this project: manual locks only
```

`add` and `remove` take patterns as rules do (folders allowed); `remove`
wins.

Auto-locked at first (when on): Unreal `.umap .uasset`; Unity `.unity
.prefab .asset`; Godot `.scn .res` (binary); Blender `.blend`; source art
`.psd .psb .kra .spp .ma .mb .max .c4d`. Not: `.als` (merged by track),
audio, images and models exported from elsewhere, Godot's text `.tscn
.tres` (offered, off).

## A lock's life

- **Taken** when R3V sees a file of a locked kind change (the folder's
  watch), or by hand: right-click a file or folder › Lock. A folder's lock
  is a path prefix: files added in it later are in it too.
- **Held** until the change is committed and shared, the changes to the
  file are discarded, its holder unlocks it, or an admin breaks it.
  Closing the app keeps it; heartbeats only say who is online.
- **Across branches**: a lock is the project's, whatever the branch.
  (Later, maybe: held until the change reaches main, as Plastic's smart
  locks do.)
- **Someone else's**: when you change a file another person holds, the
  app says so at once ("Kai is editing Harbor.umap: your change can't be
  shared"), not at commit. Your change stays; it can't be shared until the
  lock goes (or you discard it).
- **Offline**: a change to a locked kind waits to be locked; the app tries
  when it's back online, and says so if someone took it meanwhile.
- **Old locks** show their age ("locked 3 days ago") so a team can ask, or
  an admin break them.

## The service is what enforces it

The app shows and takes locks; the service refuses a share (a branch
moving) whose new versions change a file someone else holds. Older R3Vs,
a CLI, a computer that was offline: none get past it. Turning locks on is
therefore a team feature (older R3Vs must update before sharing).

## Later

1. An option to keep locked kinds read-only on disk until locked (as
   Perforce and Git LFS do): safest, more in the way.
2. An Unreal Editor plugin that takes the lock when an asset is first
   changed in the editor.
3. Locks kept until the change is merged into main.

## What a share changes

A branch moving on a hosted team always carries the paths it gives new
content to, locks on or off: `PUT …/keys/projects/<pid>/branches/<key>`,
`content-type: application/vnd.r3v.branch+json`, `{"head", "changed"}`
(stored as `<head>\n`, as before). `changed` is worked out from the
versions not on any of the team's branches yet, each against its
parents: a path counts when the version's content there is none of its
parents' (added, changed, deleted; a merge decision that made something
new). Content a merge takes from the team's versions is a parent's, so
merging main into a branch doesn't count teammates' changes as the
sender's (`project.changedPaths`). Worked out from the team's branches,
not from which versions storage lacks, so a share stopped after its
versions went up says the same next time.

Refused, the share's versions stay committed here, shared once the locks
go: the app says who holds what (`files_locked` on the command line).

## The app

- **Shows** a padlock with the holder's picture on locked files and
  folders (Files tab: tree, list, grid; the Versions tab's changes), "you"
  for yours, its age in the tooltip.
- **Takes** locks by hand (right-click › Lock / Lock folder) and by
  itself: the project's folder watch, and every look at the project's
  changes, lock changed files of an auto-locked kind. Out of reach, they
  wait (`.r3v/locks-waiting.json`, written whole then renamed) and are
  taken once the team answers; one someone took meanwhile is told of.
- **Tells at once** when you change a file someone else holds ("Kai is
  editing Harbor.umap: your change can't be shared until it's unlocked"),
  once per file and holder; and when it's unlocked again.
- **Frees** yours: Unlock, discarding the changes to a file (its file
  lock; folder locks stay), a share (the service frees the file locks in
  `changed`). Admins break others' (asked first).
- **Live notices** (`lock`, `unlock` with `shared`) read the locks again.

Stopped half-way, nothing is lost: a lock taken but not yet shown is read
from the service next time; the waiting list is whole or as it was; a
share refused or cut off is committed here and shared later.

## API (R3V Cloud, as built)

| | |
|---|---|
| `GET …/projects/:pid/locks` | `{items: [{path, prefix, memberId, workspace, since}]}` |
| `POST …/projects/:pid/locks` `{lock: [path], unlock: [path], workspace}` | `{locked, refused: [{path, memberId}]}`; `path` ending in `/` is a folder's prefix. Refused when another member holds it or a folder over it (a folder: also anything under it). A member's own lock never blocks them, on any computer; unlocking is by member. With the switch off, locking is `409 locks_off` (unlocking still works) |
| `POST …/projects/:pid/locks/heartbeat` `{workspace}` | presence only: nothing expires |
| `DELETE …/projects/:pid/locks/<path>` | admins: break a lock |
| a branch moving | the body above; with locks on, the plain body is `409 update_r3v`, and a path someone else holds `409 locked` with `locks: [{path, memberId}]` (at most 100), nothing written; a holder's share frees their own file locks in `changed` |
