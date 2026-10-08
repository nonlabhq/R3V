# File locks

Files that can't be merged (an Unreal level, a Unity scene, a .blend, a
.psd) lose someone's work when two people change them at once: one of the
two versions has to go. A lock says, before the work starts, who is
changing a file; the others see it and wait. Live sets don't need it: R3V
merges them track by track.

Locks are an R3V Cloud feature (designed, not built): only a service can
say who holds a file, at once and for everyone. A team on its own storage
has none.

## Off unless a team wants them

Many teams split their work so that two people never touch the same file;
for them locks are only in the way. So:

- **The team's switch, off at first.** When a team gets a project of a
  kind that benefits (Unreal, Unity, Godot, Blender…), the app offers once
  to turn locks on. Off, nothing about locks shows anywhere and the
  service checks nothing.
- **Which kinds of file**, in the team's settings: a switch per kind, the
  defaults below. A project follows them.
- **A project's own** in its `.r3v.yaml`, versioned with it: `locks: off`,
  or patterns that add to or take from the team's (`"!Content/Dev/**"`).

```yaml
locks:
  - "*.umap"
  - "*.uasset"
  - "Content/Maps/**"
  - "!Content/Dev/**"
```

Locked at first (when on): Unreal `.umap .uasset`; Unity `.unity .prefab
.asset`; Godot `.scn .res` (binary); Blender `.blend`; source art `.psd
.psb .kra .spp .ma .mb .max .c4d`. Not: `.als` (merged by track), audio,
images and models exported from elsewhere, Godot's text `.tscn .tres`.

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

## API (R3V Cloud, to change)

The service's `docs/api.md` has locks kept by heartbeats (freed five
minutes after the last). They change to:

| | |
|---|---|
| `GET …/projects/:pid/locks` | `[{path, prefix, memberId, workspace, since}]` |
| `POST …/projects/:pid/locks` `{lock: [path], unlock: [path], workspace}` | as now; `path` ending in `/` is a folder's prefix |
| `DELETE …/projects/:pid/locks/<path>` | admins: break a lock (audited) |
| the team's settings | `locks: {on, kinds}`; turning on is a team feature |
| a branch moving | carries the paths its new versions change (the service doesn't read versions); with locks on, a move without them is refused (an older R3V), and so is one changing a path someone else holds (`conflict`, the paths and holders); a holder's own share frees their locks on the paths it changed |

No heartbeat ends a lock; live notices (`lock`/`unlock`) as designed.
