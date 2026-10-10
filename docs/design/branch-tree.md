# Branches as a tree

Status: planned (Nightly). Changes the version format: the release that
ships it forces updating (`-MinVersion`).

Today a branch is a pointer, as in Git: which branch a version belongs to
is guessed by walking back from each branch's latest version, in an order
that starts with the branch you are on. So a branch's own early versions
can be drawn as another's (made from it later), and the graph moves round
as you switch (yours always to the right). People think of their project
as a tree: main, the branches made from it, the ones made from those. R3V
keeps that tree, and draws it.

How others do it: Git (and Diversion, as far as its documents say) keeps
pointers only, and tools guess the lanes. Perforce streams have a parent
each, drawn as a tree of streams. Plastic SCM and Mercurial's named
branches put every changeset on one branch for good. R3V does both: a
version keeps its branch, a branch keeps its parent.

## A version's branch

A version records the branch it was committed on: `branch`, the branch's
key (a rename changes the record's name, not the key), in the version
itself, so it is never guessed and never changes.

- A commit, a share, a take back (the version it makes): the branch the
  project is on.
- A merge version: the branch merged into.
- A version committed on an older version (`CommitOnOlderVersion`): the
  branch it is on.
- Versions from before (no `branch`): guessed as today, oldest R3V
  test data only.

A version committed on the wrong branch stays there: it can be merged
elsewhere, not moved. Before committing, Parked's **Bring changes here**
moves changes to the right branch.

Older R3Vs read the field and ignore it, but would commit without it:
hence the forced update.

## A branch's parent

A branch's record (`branchinfo/<key>.json`) gains `parent` (the key of
the branch it was made from) and `from` (the version it starts at),
written when it is made (`CreateBranchNamed`): the branch the project was
on, and the version it was on. main has none.

- **Fixed.** `from` is history; `parent` only orders the drawing and, later,
  where "merge back" goes. No reparenting by hand: Perforce needs it
  because streams only merge with their parent; R3V merges anything into
  anything.
- **Changed by itself in one case**: a branch archived while its children
  stay (below) hands them to its own parent.
- Branches from before: `parent` from the branch log's first entry for
  them (the version they started at, on the branch that had it), else
  main.

Teams keep their tree shallow (main, short branches, release lines), as
game teams do. (Later, maybe: asking "Start it from main instead?" when
a branch is made from a branch.)

## Merging into another branch

Always a merge version, never a fast forward: the branch merged into
gets a version of its own ("Merge Amazing Ideal"), so its line stays its
own and the history says what came in. Updating your branch with the
team's versions of the same branch still fast-forwards.

## Archiving, then deleting

Branches are archived, not deleted: out of the graph and the branch
menu, in the **Archived** list (and in the graph, faint, with "Show
archived"), and brought back with **Unarchive**. Only an archived branch
can be deleted.

- **Archiving takes the branch's children with it** (the subtree). The
  dialog lists them: who is on each, when they last committed, versions
  not merged anywhere. One with someone else's work on it (a teammate
  there, unshared or unmerged versions of theirs) is unticked by default
  and said so ("Kai is on Hello: 1 version not shared, 2 hours ago").
  Children left unticked stay, under the archived branch's parent.
- **Merging offers it**: the merge preview has "Archive Amazing Ideal
  after merging (and Test, Hello)", ticked by default, each child
  untickable as above.
- **Unarchive** brings the subtree back as it was.
- **Delete** (archived branches only, asked again): the branch's key and
  record go for good. Its versions stay (storage cleanup never deletes a
  version), drawn on another branch's line where one has them, else not
  drawn.
- main is never archived; neither is the branch you are on (switch
  first).

Archiving is the record's `archived: {head, by, time}` and the key
removed, as deleting is today (`deleted` becomes `archived`; the
interruption table in [branches.md](branches.md) stands, with the
subtree's branches one by one, parent last). Deleting removes the record
and the branch log keeps the move.

### A teammate's branch archived

Their work is never lost, whatever was known when archiving: their files
and versions stay on their computer. The project's page says "Test was
archived by Yi" with **Unarchive**, **Bring my changes to another branch**
(Parked's Bring changes here) and **Start a new branch here** (their
unshared versions go on it).

## Who is working where

The archive dialog needs to know who is on a branch and with what. Each
copy of a project already keeps a workspace record in the team's storage
(`workspaces/<id>.json`: member, branch, the team version it has, time),
written on update and share. It gains, written when they change (at most
every 5 minutes, and on every switch):

- `changes`: files changed, not committed;
- `unshared`: versions committed, not shared;
- `parked`: the branches with changes parked.

A last known state with its time, not a live one: shown as "Kai on Test:
3 changes not committed (2 hours ago)". It works on a team's own storage
as on R3V Cloud (where locks' heartbeat could make it fresher later).
The safety net is the teammate's own app, above.

## The graph

Drawn from the tree, the same wherever you are:

- main in the middle; each branch beside its parent, children in the
  order they were made, theirs further out. Columns change only when
  branches are made, archived or deleted.
- A version is drawn on its own branch's line; a branch's line starts
  at its `from`.
- The branch you are on is marked (glow, label), not moved.
- Archived branches: hidden, or faint with "Show archived".

## Build order

1. Version format: `branch` written on every new version (snapshot,
   save, merge, take back, keep this version); `-MinVersion`.
2. Branch records: `parent`, `from`; merges into another branch always
   make a version.
3. Graph: lines from versions' branches, columns from the tree.
4. Workspace record: `changes`, `unshared`, `parked`.
5. Archive (subtree, the dialog, Unarchive), delete archived only,
   the merge preview's option, a teammate's archived branch.
