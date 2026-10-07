# Looks: pictures, icons and colours

Status: built, Nightly only (`remote.Looks`, set in
`internal/remote/looks_nightly.go`).

How people and projects show, the same for the whole team:

- **A person**: their picture, else their initial on their colour. Set in
  **User settings** (click yourself at the bottom of the sidebar). One look
  for all of your teams: the app gives it to each team you're in, and to a
  team you join later.
- **A project**: an icon (50 built in, `lib/projectIcons.ts`), else its
  initial, on its colour. Set in the project's settings, for the whole team.
- **The history graph** shows each version's author as their picture, else
  their initial on their colour. The ring stays the branch's colour.

## One palette

`tokens.css` has one palette (`--palette-<name>`, 13 colours); people,
projects and the branches' lanes (`--lane-0..4`) use it. A stored colour is
the palette's name (`teal`), never a value: the look can change, the names
stay. With no pick, a colour is picked from the member id (from the
project's name for a project: one of lanes 1–4, as before), so everyone
sees the same one.

Names (colours and icons) are what a team stores: add, never rename or
remove one. A name a build doesn't know shows as no pick.

## What the team stores

| What | Where |
|---|---|
| A member's colour and picture | `members/<id>.json`: `color` (a name), `picture` (the picture's SHA-256) |
| The picture | `pictures/members/<id>/<sha256>`: a square PNG or JPEG, at most 256 pixels (the app makes 128) and 64 KB |
| A project's icon and colour | `projects/<pid>/project.json`: `icon`, `color` (names) |

Records are rewritten whole by every R3V, keeping fields it doesn't know
(`jsonx.Extra`), so a rename by an older R3V keeps a look.

Changing a picture (`remote.SetMemberLook`):

1. Turn the team feature on (see below).
2. Put the picture: one request, checked by storage against its SHA-256.
   One cut off is never there.
3. Write the member's record naming it.
4. Delete that member's other pictures.

| Stopped after | The team has | The app shows | Next time |
|---|---|---|---|
| 1 | the feature on, the old look | the old look | as new |
| 2 | a new picture no record names | the old look | step 4 deletes it |
| 3 | the new look, the old picture too | the new look | step 4 deletes it |

Nothing is sent twice that matters: the picture's key is its hash.
Storage cleanup (`gc.go`) only looks under `objects/` and `chunked/`, so
it never deletes a picture (`TestCleanupKeepsPictures`). Backups copy
`pictures/` with everything else.

A project's own picture (later) would go the same way under
`pictures/projects/<pid>/`, named by the record's `picture`.

## The team feature

Looks change what a team stores, so they are a team feature (`looks`, see
[channels.md](channels.md)). The first change in a team turns it on, with
no question asked (the maintainer's choice): from then on, a Stable R3V
stops before working with that team and says to update or switch to
Nightly.

Stable doesn't reach looks: `remote.Looks` is false, `SetMemberLook`,
`SetProjectLook` and the pictures refuse (`ErrLooksNotInBuild`), the app's
`Profile` says `available: false` and the sidebar stays as it was
(`looks_stable_test.go` in `internal/remote` and `desktop`).

## On this computer

- `teams.json`: `look: {color, picture}` (the picture by its SHA-256).
  Stable keeps it untouched.
- `pictures/<sha256>` in the settings folder: your picture, and teammates'
  as their teams gave them (named by their hash, so never stale). Written
  whole (`store.WriteAtomic`), checked against the name when read.
- `MemberLooks(root)` maps a project's team members to their looks, asking
  the team at most once a minute; the team watch makes it ask sooner.

## Hosted teams (R3V-Cloud)

Not yet: their records go through the service, which keeps only what it
knows. The app shows initials there, and User settings says the team can't
keep pictures yet. What R3V-Cloud needs to add:

- keep a member's `color` and `picture` (names and a hash) and a project's
  `icon` and `color` in its records, returning them in the lists;
- an API to upload a member's picture (checked: square PNG/JPEG, 256 pixels,
  64 KB, its SHA-256) and to read one by member and hash;
- the `looks` feature in the team's features.

The client side is then a `PictureStore` for the broker.
