# Branch names, colours, deletes and milestones

Status: names, colours, deleting and getting branches back built, Nightly
only (`remote.BranchRecords`, set in `internal/remote/branches_nightly.go`).
Milestones follow.

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

## Hosted teams (R3V-Cloud)

Not yet: the service keeps only the keys it knows. What it needs:
`branchinfo/<key>.json` as small keys (readers read, writers write), a
`record` notice of kind `branch`, and `brokerBucket.KeepsBranchRecords`.
Until then a hosted team's branches are called by their keys.
