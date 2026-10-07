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

(Designed with the maintainer, 2026-10-08; part 1 being built, part 2
waits for pricing.)

A team that used its own storage for a while moves to R3V Cloud, often
the moment it starts paying. Decided:

- **A trial, the move included** (part 2): during it the old storage is
  kept (the app says not to delete it yet), so going back is always
  possible; a trial that ends without paying leaves the hosted team
  read-only (downloads and export always work), then deleted after a
  grace period, said several times before.
- **Pricing**: not decided. The estimate shows sizes, not prices, until it
  is.
- **No freeze while the bulk is copied**: copying can take hours; the team
  keeps working meanwhile. Then a short freeze, a second pass for what was
  shared since (the copy only sends what is missing: cheap), and the
  switch.
- **Choosing projects**: some can stay behind (archives), e.g. to fit a
  plan.
- **Leaving again**: back to a team's own storage, the same way backwards
  (part 1, later in it).

## Part 1 (now)

1. **Estimate**, no account needed: per project, the size on R3V Cloud
   (each project's contents apart: a sample used by three projects counts
   three times, so it can be more than the bucket holds; said plainly),
   versions, people; and, from AWS, roughly what the egress costs.
2. **Sign in**, pick or make the hosted team, pick the projects.
3. **Read-only key** for the bucket, made by the person (shown how for R2
   and S3), given to the service; never the connection code's own key.
4. **Bulk copy, not frozen**: the app sends the service each project's
   plan (below); the service copies the contents in the background
   (progress kept: the app can close; stopped, it resumes).
5. **Short freeze**: `moving` in the old `team.json` stops every R3V's
   sharing there; the app sends the plan again (only what changed since),
   waits for it.
6. **Records and branches**, by the app, through the hosted team's
   normal API: each project's branch names and colours, milestones, its
   members' looks, then its branches, then its record (only then listed).
7. **Switch**: `movedTo` in the old `team.json`; the service forgets the
   key. Teammates' apps say the team moved: sign in, accept, claim who
   they were, and their projects reconnect in place.
8. **Afterwards**: the app reminds, later, that the old storage can be
   deleted (and how); it never deletes it.

### What the service does (R3V-Cloud)

Only the contents, the big part; the app writes everything else.

| Call | What |
|---|---|
| `POST /v1/teams/:team/moves` `{source: {endpoint, region, bucket, prefix, accessKey, secretKey}}` | checks it can read (`team.json`), keeps the key encrypted: `{move}` |
| `POST /v1/teams/:team/moves/:move/plan` `{project, items: [{src, dst, size}]}` | up to 1,000 items a call; `src` a key in the bucket (`r3v/objects/ab/…`), `dst` relative to the project (`objects/ab/…`, `chunked/…`, `snapshots/<id>.json`); items already there are skipped |
| `POST …/moves/:move/start` | copies what the plans list, a Queue, in batches; a big file's pieces before its mark and list (the plan's order within a project is kept) |
| `GET …/moves/:move` | `{state: copying \| done \| paused \| failed, items, itemsDone, bytes, bytesDone, failed: [{src, error}]}`; `paused` with `over_limit` when the plan's room runs out (resumes after an upgrade) |
| `POST …/moves/:move/cancel`, `DELETE …/moves/:move` | stops; forgets the key (also on `done` + switch, and after a day idle) |
| `POST /v1/teams/:team/members/import` `[{id, name, color, picture}]` | members to claim: an accepted invitation can claim one (`claim: id`), the account then being that member id in the team |

Checked as it copies: sizes, and R2's checksum where the source gives a
SHA-256 of what it stores. The key is never logged or returned.

### What the app does (R3V)

The wizard (Team settings › Move to R3V Cloud); the estimate; the plans
(walking each project's versions, folder lists and chunk lists, the same
walk as moving a project); `moving` and `movedTo` in `team.json`, and
every R3V that knows them stopping its shares there; the records and
branches at the end; the teammates' banner, reconnecting and claiming;
reminding about the old storage. Leaving R3V Cloud the other way uses the
same walk, the app copying (`MoveToTeam` per project, then the team's
records).

## Part 2 (with pricing)

Plans and what the estimate maps to; the trial; Stripe; a trial ending.

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
| estimating, choosing | as it was | maybe made, empty | the wizard again | start over |
| bulk copy | in use as before | contents in part, nothing listed | "Copying to R3V Cloud, n of m" | the copy resumes (also after the app closed) |
| frozen, second pass | read-only for R3V | contents almost all | "Moving: the team can't share for a few minutes" | the pass resumes; or unfreeze (`moving` taken out) |
| records and branches | read-only | projects listed one by one | the same | the app goes on (each write is safe to repeat) |
| switched | `movedTo` | everything | "moved to R3V Cloud" | teammates join |

A project is listed on the hosted team only once its branches are there,
so a half-copied one is never offered. Every step is safe to repeat (same
keys, same bytes; branches created only if absent).

## Security

The read-only key is held by the service, encrypted, only while the move
runs, never logged or shown, deleted at the switch or when the move is
given up. The app never sends the team's own connection code.

