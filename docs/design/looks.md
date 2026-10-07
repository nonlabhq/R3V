# Looks: pictures, icons and colours

Status: built, Nightly only (`remote.Looks`, set in
`internal/remote/looks_nightly.go`); hosted teams too.

How people and projects show, the same for the whole team:

- **A person**: their picture, else their initial on their colour. Set in
  **User settings** (click yourself at the bottom of the sidebar). One look
  for all of your teams: the app gives it to each team you're in, and to a
  team you join later.
- **A project**: an icon (50 built in, `lib/projectIcons.ts`) or an emoji,
  else its initial, on its colour (an emoji on none). Set in the project's
  settings (click its icon before the name), for the whole team. An emoji
  is kept as its code points (`e-1f3b5`), so it is a name like the icons;
  the picker (emoji-picker-element) has its data for the app's languages
  built in. The project's page shows the icon before its name, under its
  team's name (with the team's settings, ⚙).
- **The programs' icons** come first in the picker (`lib/appIcons.ts`,
  named `app-…`): Ableton Live, Bitwig, Pro Tools, Reason, Max, Audacity,
  Unity, Unreal, Godot, Blender, Houdini, Cinema 4D, Maya, DaVinci Resolve,
  Figma, Krita. A project just added gets the icon of the one program R3V
  recognises (its preset: Live, Unity, Unreal, Godot; or design files all
  `.blend`, `.c4d` or Maya's), once its first share makes it the team's,
  and only when it has no icon yet.
- **The history graph** shows each version's author as their picture, else
  their initial on their colour. The ring stays the branch's colour.

## One palette

`tokens.css` has one palette, the R3V VI's 12 colours (`--palette-b1..b12`;
the main branch alone has the brand orange). People, projects and the
branches' lanes (`--lane-1..12`, in order) use it. A stored colour is the
palette's number (`b3`), never a value or a name: the colours can be tuned,
the numbers stay. With no pick, a colour is picked from the member's id
(the project's id for a project), so everyone sees the same one.

Numbers and icon names are what a team stores: add, never reuse or rename
one. One a build doesn't know shows as no pick.

### The programs' icons and their licences

The programs' icons are the [Simple Icons](https://simpleicons.org) glyphs
(simple-icons 16.34.0), copied as path data, not a dependency: their SVG
data is CC0, except Godot's logo (by Andrea Calabró, CC BY 4.0: credited in
`appIcons.ts` and here). The brands and trademarks stay their owners'; the
icons only say which program a project is made with, shown in the icon's
colour, as Simple Icons offers them. Brands Simple Icons doesn't have (some
taken out at their owners' request: Ableton, Adobe, FL Studio, Reaper…)
aren't added from elsewhere: Live has a glyph of our own (a Session view's
clips), not its logo. A new one is checked against the brand's guidelines
(`guidelines` in simple-icons' data) and `license` first.

## What the team stores

| What | Where |
|---|---|
| A member's colour and picture | `members/<id>.json`: `color` (a name), `picture` (the picture's SHA-256) |
| The picture | `pictures/members/<id>/<sha256>`: a square PNG or JPEG, at most 256 pixels (the app makes 128) and 64 KB |
| A project's icon and colour | `projects/<pid>/project.json`: `icon`, `color` (names) |

Records are rewritten whole by every R3V, keeping fields it doesn't know
(`jsonx.Extra`), so a rename by an older R3V keeps a look.

Changing a picture (`remote.SetMemberLook`):

1. Put the picture: one request, checked by storage against its SHA-256.
   One cut off is never there.
2. Write the member's record naming it.
3. Delete that member's other pictures, except the one the record names
   when read again (the same member may have changed theirs on another
   computer meanwhile: `TestMemberLookOnTwoComputers`).

| Stopped after | The team has | The app shows | Next time |
|---|---|---|---|
| 1 | a new picture no record names | the old look | step 3 deletes it |
| 2 | the new look, the old picture too | the new look | step 3 deletes it |

Nothing is sent twice that matters: the picture's key is its hash.
Storage cleanup (`gc.go`) only looks under `objects/` and `chunked/`, so
it never deletes a picture (`TestCleanupKeepsPictures`). Backups copy
`pictures/` with everything else.

A first share that stopped after writing the project's record keeps that
record when it's done again (a look or a name given meanwhile stays).

A project's own picture (later) would go the same way under
`pictures/projects/<pid>/`, named by the record's `picture`.

## Not a team feature

Looks only add to what a team stores, and every R3V since 0.1.0 works
with them as before: it rewrites records whole keeping the fields it
doesn't know, its storage cleanup never looks under `pictures/`, and it
shows initials. So they are not a team feature (see
[channels.md](channels.md)): a Nightly member picking a colour never stops
Stable teammates. (A test build turned a `looks` feature on; every build
knows the name, so such a team still opens.)

Stable doesn't reach looks: `remote.Looks` is false, `SetMemberLook`,
`SetProjectLook` and the pictures refuse (`ErrLooksNotInBuild`), the app's
`Profile` says `available: false` and the sidebar stays as it was
(`looks_stable_test.go` in `internal/remote` and `desktop`).

## On this computer

- `teams.json`: `look: {color, picture}` (the picture by its SHA-256).
  Stable keeps it untouched.
- `pictures/<sha256>` in the settings folder: your picture, and teammates'
  as their teams gave them (named by their hash, so never stale). Written
  whole (`store.WriteAtomic`), checked against the name when read. The app
  deletes those not shown for 30 days when it starts (not yours): they
  come again from the team.
- `MemberLooks(root)` maps a project's team members to their looks, asking
  the team at most once a minute; the team watch makes it ask sooner.

## Hosted teams (R3V-Cloud)

The service keeps records as R3V writes them, looks included, and member
pictures as small keys at the same names, checked when written (square
PNG/JPEG, 256 pixels, 64 KB, named by its SHA-256) and served with an
immutable cache header (R3V-Cloud's docs/api.md). So the broker is a
`PictureStore` like any bucket (`brokerBucket.KeepsLooks`), and the same
steps apply. Its people live in the service, not in `members/`: the
first look a member gives a hosted team writes their record, with their
name (`shareLook`). No team feature (see above).

## Seeing changes

A storage team (S3, R2) has no way to tell: the app asks for looks again
when the team watch sees something (a new version), and at most once a
minute. That is enough for looks.

A hosted team tells at once. Besides `key` (a branch moved) and
`team`/`access`, its live connection sends one more kind for every record
the service writes, so the app needn't poll for any of them
(`cloud.Hub.OnRecord`, followed by `App.onRecord`):

    {"type": "record", "kind": "member" | "project" | "team" | "lock",
     "id": "<member, project or file id>", "sum": "<picture hash, if any>"}

| Kind | When | The app |
|---|---|---|
| `member` | a name, colour or picture changed | asks for looks again; the history and sidebar redraw |
| `project` | a project added, deleted, renamed, or its icon or colour changed | the team's project list again |
| `team` | the team renamed, or a feature turned on | the team again (a feature this build lacks: says to update at once) |
| `lock` | a file locked or unlocked | the project's locks again |

`member`, `team` and `project` go to everyone in the team (a project's
too: people don't listen to a project they don't have yet); `lock` to
whoever listens to its project. The Durable Object that writes a record
sends the message after the write, in order. The app ignores kinds it
doesn't know.
