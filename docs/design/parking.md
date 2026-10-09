# Parked changes

Status: built, Nightly (`project.Parking`); on this computer only.

Changes not committed shouldn't stop anyone going elsewhere: to another
branch, to an older version to listen, and back. Today a switch with
changes asks to discard or commit them. With parking, the changes stay
with their branch: going away parks them, coming back brings them back.
Nothing to remember, no stash list to manage.

## The model

- **One parked set per place.** Leaving a branch with changes parks them
  on it; coming back to it brings them back. A place is a branch's latest
  version, or an older version of it: at most one set each.
- **An older version** (Go to this version) is a look back: changes made
  on the latest version are parked on the branch and come back with
  Back to latest. Changes made while on the older version are parked at
  that version, and come back when you go to it again.
- **Where it lives**: this computer only (`.r3v/parked/`). Never shared,
  not a version of the history, not seen by the team.
- **What it holds**: everything a commit would hold (Repo.Only aside):
  changed, added, deleted and renamed files the rules keep. Files the rules
  leave out stay on disk, as in any switch.

So no team feature is needed: nothing the team stores changes.

## What a parked set is

A version that is never shared, made as `updateKeepingWork` makes its
kept work (`workingManifest`, `save`): its parent is the version the
changes were made on, its contents in the local store, deduplicated by
hash (every new content copied there, even what the team's storage has).
Plus a record, `.r3v/parked/<branch key>.json` (`<key>@<version>.json`
for an older version):

```json
{"version": "<id>", "base": "<id>", "branch": "<key>", "at": "<id>", "files": 3, "bytes": 182733, "since": "2026-10-09T10:12:00Z"}
```

written whole then renamed. The record is what says a set is parked: a
version with no record (stopped between the two) is a small leftover, as
kept work is.

Local cleanup must keep them: `PruneObjects` never removes a parked set's
contents, even when the team's storage has them (preuploaded contents
aren't referenced by any shared version, so storage cleanup may remove
them later). The kept-work cleanup (`kept-work`) leaves parked versions
alone.

## Switching with changes

`SwitchBranch`, `GoTo`, and Back to latest, when the project has changes:

1. The usual checks first: Live running with unsaved changes, files
   another program holds (`InUse`), an unfinished switch. Parking can only
   keep what is saved on disk; these fail as they do today.
2. **Park**: save the changes as a version (contents into the store), then
   write the record. Nothing in the project folder has changed yet.
3. **Switch**: put the target's files in place (`putFiles`), the
   destination's own parked set merged in when it has one (below).
4. **Done**: remove the destination's record (its set is back in the
   folder); the version itself stays until the next tidy.

The CLI does the same (`r3v switch`, `r3v checkout`): `--json` gives `parked`, `restored`,
`merged` and `waiting` (docs/agents.md). `--force` still discards.

## Coming back

Leaving a place with changes while its own set waits (it came back with
a conflict, below) is refused (`changes_parked_here`): bring it back or
discard it first.

Arriving at a place with a parked set:

- **The branch hasn't moved** (its latest is the set's base): the set's
  files are put in place as they are. The changes are back, uncommitted.
- **It moved** (you or a teammate shared, an Update came in, a version
  taken back): the set is merged onto the branch's latest, as Update
  keeps work today (`mergeManifests`, base = the set's base). Live sets
  merge track by track.
  - No conflict: the merged files are put in place, and the app says so
    ("Your parked changes came back, merged with 2 new versions").
  - A conflict: the switch still happens, the set stays parked, and the
    app offers **Bring back now** (opens Merge decisions) or **Later**.
    Never a merge forced on arrival.

## Moving a set

What stash's "pop on another branch" was for (changes made on the wrong
branch): **Bring changes here** on a parked set from another branch merges it
into the current branch's files (the same merge, the set's base as base),
and removes its record. With changes in the folder already, they are
merged too; a conflict opens Merge decisions with nothing changed until
it's done.

## In the app

- **Graph**: a parked set is a hollow dashed node beside its base,
  labelled `Parked · 3 files` (`{n} file parked` plural forms), its age in
  the tooltip. Its card: **Switch and bring back** (on another branch) /
  **Bring changes here** / **Show changes** (its files, as a version's) /
  **Discard** (red, asked first).
- **The current node** (your changes) is unchanged.
- **Notices**, once per switch: "Your 3 changes on main are parked. They
  come back when you switch back." and "Brought back your 3 parked
  changes." Short, in the toast area, with **Undo** for a few seconds
  where it is safe (switch back).
- **Size**: a set over 2 GB (new recordings) says how much it keeps on
  disk when parked; nothing is ever removed by itself. Sets older than 30
  days show their age in the graph, nothing more.
- **Branch deleted** (by you, or a teammate): its set is kept and shown
  as `Parked · from <name>` on its base, Bring changes here or Discard.

Behind Nightly: Stable keeps "discard or commit" (a Stable test checks a
switch with changes still asks). A Stable R3V opening a project with
parked sets ignores `.r3v/parked/` and keeps the set versions (it removes
only the kept work it names in `kept-work`). One gap: `PruneObjects`
could drop the local copy of a parked file the team's storage holds only
as a preupload, which storage cleanup may later remove. So the prune rule
(keep what `.r3v/parked/` names) goes into both channels, in the release
before parking: every R3V from then on keeps them. Only an R3V older than
that, opened on a project with parked sets, has the gap; updates never
go back to one by themselves.

## Stopping half-way

States, and what the app shows in each:

| Stopped | Folder | Records | Next time |
|---|---|---|---|
| while parking (step 2) | as it was | none, or a set version without record | nothing to do (a leftover version, a few KB). The switch can be made again |
| after parking, before the files change | as it was, the changes in it | the left place's record | `tidyParked` (the app reading the project, the next switch): a record here whose files are the folder's goes |
| the files start changing | as it was | the record; `switching` written | `UnfinishedSwitch` → `RecoverSwitch` puts the parked files back ("work <id>"), drops that record |
| mid-switch (step 3) | partly the target | the left place's record | the same: as before the switch |
| after the switch, before step 4 | the target, the destination's set in it | the destination's record, and `parked/arrived` naming it | `arrived` (the next switch, or a recovery) drops it once the project is on the version it names |

The five checks (docs/development.md#interruptions): the team's data
untouched (nothing shared); files as they were or complete (`putFiles`
and `switching`); what the app offers next safe (no record is acted on
without matching the folder); nothing twice (a set brought back has no
record); nothing left behind (a set version with no record is a few KB).
Tests: `parked_test.go` (each stop above; killed at every file written).

## Not now

- Sharing parked sets with the team, to back them up or go on from
  another computer (Plastic's and Perforce's shelves): a team feature,
  later.
- Several sets per branch.
- Parking by hand without switching ("park for now").

## Build order

0. Both channels, a release ahead: `PruneObjects` keeps what
   `.r3v/parked/` names (see above).
1. `internal/project`: parked records, park on switch, bring back (same
   base and merged), Bring changes here, Discard, tidy, prune keeping them; the
   interruption tests.
2. CLI: `switch` / `goto` keep by default, `--json` fields, `r3v parked`
   (list, bring changes here, discard).
3. App: switching without the discard/commit question (Nightly), notices,
   graph node and card; locales.
4. Stable test, smoke, docs (`docs/agents.md`, user docs).
