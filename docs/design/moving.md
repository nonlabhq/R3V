# Moving projects and teams

Two different things:

- **A project moves to another team** (built, Nightly only:
  `project.MoveProjects`): one
  project, to a team with maybe other people and another kind of storage
  (its own storage ↔ R3V Cloud, either way). Like transferring a
  repository.
- **A team moves to R3V Cloud** (designed, not built): the same people and
  projects, kept somewhere else.

Both copy a project's whole history from one storage to another with the
same steps (below); they differ in what is around it.

## Copying a project's history

`project.MoveToTeam` (internal/project/moveteam.go), from the computer
that has the project, straight from one team's storage to the other's:
nothing is kept on this computer on the way, and only what the other team
lacks is sent. The two kinds of storage keep contents differently (one
pool for a team's projects, or each project's apart on a hosted team), so
the copy walks the project's versions to find what they use.

1. Its versions, here (small): every branch's, every deleted branch's and
   milestone's, with their ancestors.
2. The files they use: folder lists, files, big files' pieces (the pieces,
   then their mark, then their list), as they are stored (compressed or
   not), only those the other team lacks; leased there meanwhile.
3. The versions; the branches' names and colours; the milestones.
4. The branches (again if a branch moved while copying: until they stay).
5. The project's record: only now does the other team list it.

| Stopped after | The other team | This team | Next |
|---|---|---|---|
| 1-4 | files and versions it doesn't show (cleaned up in time) | as it was | moving again goes on (nothing sent twice) |
| 5 | has it all | has it too | moving again finishes the move |

## A project moves to another team

Project settings › Move to another team…: pick the team (one on this
computer this person is in), and whether to keep it in its team too (a
copy). Versions not shared yet must be shared first (a project moves with
what its team has). Moved (not copied), the first team lets it go
(`DeleteProject`: gone from its list for everyone) and this folder belongs
to the other team; the people of the first team keep their copies, which
their app shows as no longer on the team. Authors stay as their versions
name them (no member of the other team is needed). The branch log and
workspaces stay behind; names, colours, deleted branches and milestones go.

# A team moves to R3V Cloud

(Designed with the maintainer, 2026-10-08; not built.)

## Why not through the app

Self-hosting stays a bucket and a connection code (no server); a hosted
team keeps each project's contents apart. A team moves with the copying
done by the service, planned by the app (the steps above, run by it).

Moving through the app means every byte down to someone's computer and up
again: twice their line, their disk for the whole history, hours for a
sample library. The service can read the old storage itself and write to
its own R2: from R2 to R2 it stays inside Cloudflare's network, and R2
charges no egress. The app only plans: the two layouts differ (a team on
its own storage keeps one pool of contents, `objects/`, for all projects;
a hosted team keeps them per project, `projects/<pid>/objects/`), and only
the app reads R3V's formats (versions, folder lists, chunk lists).

## What moves

A whole team, every project (moving one project between teams is later).

| Old team (its storage) | Hosted team |
|---|---|
| `objects/…`, `chunked/…` (contents a project's versions use, found by walking them) | `projects/<pid>/objects/…`, `projects/<pid>/chunked/…` (copied per project that uses them) |
| `projects/<pid>/snapshots/…` | the same keys |
| `projects/<pid>/project.json`, `branchinfo/`, `milestones/`, `branchlog/`, `workspaces/` | small keys, the same names |
| `projects/<pid>/branches/<key>` | small keys, **written last**, per project |
| `members/<id>.json`, `pictures/members/…` | members to be claimed (below), their pictures |
| `team.json` (name) | the team's name; no features carried over (none apply) |
| `setups/` | copied (names only) |
| `backups/`, `gc/` | not moved (per computer; the service cleans its own) |

## Steps

1. **Start** (Team settings › Move to R3V Cloud, by someone with the team's
   connection code): sign in, pick or create the hosted team. The app
   counts what moves (sizes per project) and says so, with the plan's room
   and, where the storage is AWS S3, roughly what its egress costs.
2. **Read access**: the person makes a read-only key for the bucket (the
   wizard shows how on R2 and S3) and gives it to the service. The
   connection code's own key is never sent: it can write and delete.
3. **Freeze**: the app writes `moving: {to, by, time}` into the old
   `team.json`. Every R3V that knows it stops sharing to the team and says
   it is moving (committing here still works: shared once on R3V Cloud).
   There are no users of older versions to catch up with, so there is no
   copy of what comes after (if that changes, a second pass copies the
   versions added meanwhile before the switch).
4. **Plan**: per project, the app walks every version's folder lists and
   chunk lists (small reads) and sends the service the list of keys to
   copy, source → destination, with sizes.
5. **Copy** (the service, a Queue, in batches): each key read from the old
   storage with the read-only key and written to R2, checked by its
   SHA-256 where R2 can (`x-amz-checksum-sha256`) and by size; failures
   retried; progress kept, so a stop resumes. The app shows the progress
   and can be closed.
6. **Per project, in order**: contents → folder lists → versions → small
   keys (records, milestones, looks, log) → branches. A project's branches
   are written only once all it needs is there, as in a share.
7. **Switch**: the app writes `movedTo: <hosted team address>` into the old
   `team.json` and the service forgets the read-only key. The old storage
   is left as it was: R3V never deletes it; the person does, when happy.
8. **Teammates**: their app reads `movedTo` and says the team moved to R3V
   Cloud: sign in, accept the invitation, and their projects reconnect in
   place (same project ids: nothing downloaded again; versions committed
   while frozen are shared then).

Going back before the switch: take `moving` out of `team.json` (the hosted
team is deleted). After it, the old storage still has everything up to the
freeze.

## Members

Versions, the branch log and pictures name members by their id. The move
brings the old members along as **members to claim**: an invitation per
person, and on accepting it they pick who they were ("I'm Mia"), so their
account is that member in the hosted team (the service keeps the old id as
their member id there). Their names, pictures and authorship stay. Someone
not claimed shows by the name the old record had.

## When the service can't reach the storage

A NAS or a MinIO on a private network: the wizard says so and the move
goes through the app instead (leave the team keeping the **whole** history,
join the hosted team, share every project's versions — tested in
`internal/project/movehosted_test.go`). Leaving without the whole history
is refused when the next step is a move (older versions' files would not be
anywhere the new team can get them).

## In between

| Stopped | Old team | Hosted team | The app shows | Next |
|---|---|---|---|---|
| before freeze | as it was | maybe created, empty | the wizard again | start over |
| frozen, copying | read-only (by R3V) | contents in part, no branches | "Moving to R3V Cloud, n of m" | the copy resumes |
| a project's branches written | read-only | that project usable | the same | the rest go on |
| switched | `movedTo` | everything | "moved to R3V Cloud" | teammates join |

Nothing is shown on the hosted team before its branches are there, so a
half-copied project is never offered. The copy is idempotent (same keys,
same bytes): resuming sends nothing twice that matters.

## Security

The read-only key is held by the service, encrypted, only while the move
runs, never logged or shown, deleted at the switch or when the move is
given up. The app never sends the team's own connection code.

## Work

- **Core (R3V)**: the wizard; `moving` and `movedTo` in `team.json` (and
  stopping shares when frozen); the plan (walking versions, folder lists,
  chunk lists); the teammates' banner and reconnecting; claiming members
  on accepting; refusing to leave without the whole history before a move.
- **Cloud (R3V-Cloud)**: the move API (start, plan in batches, progress,
  cancel), the Queue copy from S3-compatible storage with a read-only key,
  keeping and forgetting the key, members to claim, branches written last.
- **Tests**: the two layouts both, with the fake hosted service
  (`internal/project/hostedfake_test.go`): a move stopped at each step and
  resumed; then an end-to-end run from a real R2 bucket to api-dev.

Later: the other way (R3V Cloud → a team's own storage), with the same
plan read backwards, so nobody is held in.
