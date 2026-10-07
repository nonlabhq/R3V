# Design: pluggable storage backends

Status: steps 1–5 of the implementation plan are done. The direct object storage backend passes the contract tests against an in-memory fake, Versity S3 Gateway v1.8.0 and Cloudflare R2 (including concurrent branch updates). Step 6 is open.

## Summary

This document describes the `Backend` interface in the client and its implementations sharing one storage layout:

| Mode | Backend | Runs where |
|---|---|---|
| **Direct object storage** | clients read and write an S3-compatible bucket directly | any S3-compatible service, or a self-hosted S3-compatible server (e.g. on a NAS) |
| **Hosted service** (possible later) | HTTP API for metadata; file transfers go straight to object storage via presigned URLs | a managed deployment |

Because both use the same object layout, a team can move between them by copying data.

## What the backend has to provide

Almost everything R3V stores is immutable and named by its content hash. Only two kinds of data change over time:

| Data | Mutability | Consistency needed |
|---|---|---|
| File contents (`objects/<sha256>`) | immutable | read-after-write |
| Version manifests (`snapshots/<id>.json`, id = sha256 of the bytes) | immutable | read-after-write |
| Project metadata (`project.json`) | rarely changes | last writer wins |
| **Branch heads** | changed by several people | **compare-and-swap** |
| Workspace states (unsaved work, soft locks) | each written by one workspace only | last writer wins |

Branch heads are the only data that needs an atomic update: a save moves a branch from the version the client last saw to its new version, and must fail if someone else moved it in between. The client then merges and retries (this loop exists today).

## The interface

Every backend implements:

```go
type Backend interface {
	// Projects
	Projects() ([]Project, error)
	PutProject(p Project) error

	// Branches: name -> version id. UpdateBranch is compare-and-swap:
	// old == "" creates, new == "" deletes, ErrConflict if the branch moved.
	Branches(pid string) (map[string]string, error)
	UpdateBranch(pid, name, old, new string) error

	// Versions (immutable, id = sha256 of the manifest bytes)
	MissingSnapshots(pid string, ids []string) ([]string, error)
	PutSnapshot(pid, id string, data []byte) error
	GetSnapshot(pid, id string) ([]byte, error)

	// File contents (immutable, named by sha256)
	MissingObjects(hashes []string) ([]string, error)
	PutObject(hash string, r io.Reader) error
	GetObject(hash string) (io.ReadCloser, error)

	// Workspace states (unsaved work, soft locks)
	PutWorkspace(pid, wsid string, state any) error
	Workspaces(pid string, out any) error
}
```

`internal/project` depends only on this interface. A backend is chosen from the connection settings stored in `.r3v/config.json`.

## Storage layout

Used by the object-store backend:

```
<prefix>/objects/<ab>/<cdef…>                    file contents, sha256
<prefix>/projects/<pid>/project.json             {id, name}
<prefix>/projects/<pid>/snapshots/<id>.json      version manifests
<prefix>/projects/<pid>/branches/<name>          one object per branch; body = version id
<prefix>/projects/<pid>/workspaces/<wsid>.json   workspace states
```

Objects are shared by all projects under a prefix, so a sample used in several songs is stored once. One object per branch avoids contention between branches.

## Rules that keep the data consistent

With direct object storage nothing checks writes for the team, so every client must follow these rules:

1. **Write order**: file contents, then the manifest, then the branch. A branch never points to a version whose manifest or files are missing.
2. **Verify on read**: every downloaded object and manifest is checked against its hash. A corrupt or truncated object is treated as missing.
3. **Immutable writes are idempotent**: uploading an object that already exists is harmless, so interrupted uploads are simply retried.
4. **Only branch updates need compare-and-swap.** Everything else is either immutable or has a single writer.

## Direct object storage backend

### Branch compare-and-swap

S3-compatible services support conditional writes:

| Operation | Request |
|---|---|
| create branch | `PUT branches/<name>` with `If-None-Match: *` |
| move branch | `GET` (remember `ETag`, check body == old), then `PUT` with `If-Match: <etag>` |
| delete branch | `DELETE` with `If-Match: <etag>` where supported |

`412 Precondition Failed` maps to `ErrConflict`. Support and consistency guarantees differ between services, so each supported service must pass the backend contract tests (below) before it is listed as supported.

### Existence checks

`MissingObjects` / `MissingSnapshots` use `HEAD` requests with bounded parallelism. Clients remember what they have already uploaded to a given backend to avoid repeated checks.

### Polling

There is no push channel, so the agent polls. To keep request counts low:

- read branch heads and known workspace states with `GET` on fixed keys rather than listing prefixes;
- list the workspaces prefix only occasionally to discover new members;
- use a long interval (e.g. 15–30 s or more), and back off while nothing changes.

Soft locks and new-version notices therefore arrive with a delay of up to one interval.

### Credentials and access

Each member gets their own access key limited to the team's bucket (or prefix), so one member can be removed without affecting the others. Every key can read and write everything in the bucket; this mode assumes a small group that trusts each other. Services that offer object locking or version retention can be used to protect history against accidental deletion.

To avoid asking musicians for an endpoint, bucket and two keys, the desktop app accepts a single **connection code**: an encoded bundle of endpoint, bucket, prefix and credentials, created by whoever sets up the bucket.

### Garbage collection

Deleting data is optional and never automatic. A later `r3v gc` can remove objects that are not reachable from any branch, version or workspace backup, and that are older than a grace period, so that uploads in progress are never collected.

## Two layers: Bucket and BucketBackend

Storage teams are built in two layers, so a new way to reach storage gets
every feature at once:

- **`Bucket`** (`internal/remote/bucket.go`): keys and bytes only. Get (with
  an etag), Open, Exists, Put (optionally checked against a SHA-256, and
  conditional: only if absent, or only if the etag still holds), Delete,
  List (in key order, a page at a time, from a key) and Folders. It knows
  nothing about R3V. Implementations: `s3Bucket` (signed S3 requests,
  multipart for big objects, retries for small requests); later the hosted
  service's brokered access (presigned URLs, branch moves through its API).
- **`BucketBackend`** (`internal/remote/s3.go`, `gc.go`, `keys.go`):
  everything a team stores, written once on top of a Bucket: projects,
  versions, branches (compare-and-swap through conditional writes), members,
  workspaces, compressed and chunked contents, setups, backup records,
  cleanup with leases. Backups and restores (`internal/backup`) read and
  write through it too.

Every branch move is also recorded (`projects/<pid>/branchlog/<time>-<random>.json`:
branch, from, to, member, time), one new key per move, after the move
succeeded: storage itself keeps only where branches are now.

## Hosted service mode (R3V-Cloud, Nightly, in progress)

A hosted team is reached through another `Bucket`, the broker bucket
(`internal/remote/broker.go`, addresses `r3v-cloud+https://…/v1/teams/<team>`),
so `BucketBackend` and every feature on it work unchanged:

- **Keys that change** (branches, workspaces, members, the team's name) go
  through the service, which keeps them with ETags (conditional writes as
  above) and checks who may do what: people can be in a team, or only in
  some of its projects.
- **File contents** go straight between the client and object storage
  through short-lived presigned URLs the service hands out. A URL writes
  only its key, once, with the SHA-256 that was signed; transfers are
  retried with fresh URLs and given up when they make no progress.
- **Contents per project**: an upload can't be checked against its name
  (blobs are stored compressed), so each project keeps its own `objects/`
  and `chunked/` under `projects/<pid>/` (`ForProject`), where only that
  project's writers can write. Storage teams keep the shared folder.
- **Batches**: "which of these are missing" goes to the service's index,
  1,000 hashes a question (`ContentsAsker`).

- **Signing in** (`internal/cloud`, `r3v login`): through the browser, a
  loopback redirect with PKCE; the app's session lives in the system's
  credential store (`internal/keyring`), never in teams.json. The
  account's teams are kept in the teams store as hosted teams (no keys;
  one member id per person, given by the service) and brought up to date
  when listed.

Still to come: live notices instead of polling, file locks, the app's
screens, and cleanup and backup project by project.

Client-side encryption of file contents is compatible with this design because all diffing and merging happens in the client: the backend only ever needs hashes and bytes. Deduplication would then use a keyed hash per team.

## Migration between modes

With a shared layout, moving a team is a copy: objects and manifests first, then workspaces, then branches (following the write-order rule). A `r3v migrate <from> <to>` command can do this incrementally and verify hashes.

## Implementation plan

1. Extract the `Backend` interface; `internal/project` uses the interface. No behaviour change.
2. Backend contract tests: one suite (branch CAS races, idempotent uploads, missing checks, workspace round trip) run against every implementation. Against a real service: `R3V_TEST_STORAGE=<connection code> go test ./internal/remote -run Live -v`. The branch race test also catches services that accept conditional headers without enforcing them.
3. One object per branch.
4. S3-compatible backend (a minimal signed-request client), tested against a self-hosted S3-compatible server and at least one hosted service.
5. Connection codes in the desktop app and CLI; slower polling for this backend.
6. `r3v migrate`, then `r3v gc`.
