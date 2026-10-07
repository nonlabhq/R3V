# Branch names, colours, deletes and milestones

Status: built, Nightly only (`remote.BranchRecords`, set in
`internal/remote/branches_nightly.go`).

## Names apart from keys

A branch's key is where it is kept (`projects/<pid>/branches/<key>`) and
what every R3V reads: letters, digits, `.`, `_`, `-`. People call a branch
anything: its **record** (`projects/<pid>/branchinfo/<key>.json`,
`{name, color}`) holds the name they gave it.

- A name is any text, any language: Unicode NFC, no space around it, no
  control characters, 1–64 characters (`remote.CleanBranchName`). Two
  branches can't have names equal but for case (`SameBranchName`).
- A new branch's key is its name when storage takes it as it is, else its
  Latin letters and digits (`Mia's Verse 2` → `mias-verse-2`), else a
  random one (`b-3f9a1c2e`) (`BranchKeyFor`).
- **Renaming changes the record only.** Nothing moves; a teammate on the
  branch keeps working, their next share goes to the same key.
- The CLI takes a name or a key (`r3v switch "Mia 的主歌"`;
  `Repo.ResolveBranch`), and `r3v branch new` takes several words.
- An R3V without records (Stable, older) shows the key. Records only add
  to what a team stores (no R3V reads `branchinfo/`; storage cleanup looks
  only under `objects/` and `chunked/`), so they are not a team feature.

Making a branch (`Repo.CreateBranchNamed`):

1. Write its record.
2. Make the branch (a conditional write: only if the key is free).

| Stopped after | The team has | The app shows | Next time |
|---|---|---|---|
| 1 | a record no branch has | nothing (records show only for branches) | making it again writes it anew |

## Colours

Each branch has a colour of the palette (`b1`–`b12`, see
[looks.md](looks.md)), kept in its record so it never changes as branches
come and go. The main branch alone has the brand orange (`--lane-0`).

- A new branch gets the palette's first colour no branch has yet (in
  order, each far from the one before; `freeColor` in `lib/branches.ts`).
- A branch with no colour kept (made before records, or by an R3V without
  them) gets one picked from its key, the same everywhere.
- Anyone in the team can change a branch's colour and name: **Branch
  settings**, from the branch menu (⋯) or the project's settings
  (Branches).

## Deleting and getting back

Anyone in the team can delete a branch other than main and the one they
are on (Branch settings, `r3v branch delete`). The versions stay: storage
cleanup never deletes a version, so a branch always comes back
(`RestoreBranch`: the key put back where it was, only if no branch has it).

Deleting (`Repo.DeleteBranch`):

1. Write where it is into its record (`deleted: {head, by, time}`).
2. Delete the key (conditional: only if it hasn't moved since read; it did:
   "someone shared on it just now", and step 1 is undone).
3. The branch log gets the move (as well as it can, like every move).

| Stopped after | The team has | The app shows | Next time |
|---|---|---|---|
| 1 | the branch, its record marked deleted | the branch (a branch there is never listed deleted) | deleting again |
| 2 | no branch; record and maybe not the log | it among Deleted branches (from the record) | Restore |

The list of deleted branches (`DeletedBranches`) is the branch log's
deletes and the records' marks, of keys not there now. Before deleting,
the app says how many versions are on no other branch (`OnlyOnBranch`).

A teammate on a branch deleted meanwhile sees it said on the project's
page (who, when; their files and versions as they were), with **Restore
it** and **Switch to main**. Committing there would make the branch again.

## Milestones

A milestone gives a version a name the whole team sees ("Sent to the
label, v1"), with a note: Git's tags, for people who don't know Git. Kept
at `projects/<pid>/milestones/<id>.json` (`{version, name, note, by,
time}`, the id random). A milestone only names a version, which storage
keeps for good, so it always leads to it; renaming one or taking it away
touches nothing else, and anyone in the team can.

- **Milestone…** on a version (its details) adds one; a flag shows on its
  dot in the history and in its card; the branch menu lists them, newest
  first, and goes to the version.
- Its name follows a branch name's rules; a note has at most 1000
  characters.
- Like records, milestones only add (no R3V reads the folder), so they are
  not a team feature; a team keeps them where it keeps branch records.

## Hosted teams (R3V-Cloud)

The service keeps `branchinfo/<key>.json` and `milestones/<id>.json` as
small keys, raw (readers read, writers write and delete; a read-only
collaborator can't), and tells the project's listeners of each change (a
`record` notice of kind `branch` or `milestone`: the page asks the team
again). `brokerBucket.KeepsBranchRecords` lets the client use them. The
service doesn't refuse deleting main (taking back a project's only
version does that); the client does, for a normal delete.
