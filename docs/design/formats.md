# Stored formats

What R3V writes in a team's storage, for readers outside R3V (R3V Cloud's
web app) and as the reference besides the code. Every R3V reads every
format here written by an older one; a change older R3Vs can't read needs
a release that forces updating (AGENTS.md, "Stored data is forever").

The corpus in `testdata/formats` has a file of each kind, good and bad, and
`cases.json` says what R3V reads from each (`internal/formats` checks it;
`go test ./internal/formats -run Corpus -update` writes it again). A reader
outside R3V tests against it and must agree byte for byte.

## Where things are

Keys relative to the team's folder in its storage:

```
team.json                               the team: name, features, look
members/<member id>.json                a member: name, look
projects/<pid>/project.json             a project: id, name, look
projects/<pid>/snapshots/<id>.json      a version's record
projects/<pid>/branches/<key>           a branch: its latest version's id (the body)
objects/<ab>/<rest>                     contents and trees, by hash (<ab><rest>)
chunked/<file hash>                     a file kept as pieces (a marker for cleanup)
pictures/…                              looks (docs/design/looks.md)
```

A team whose storage keeps contents per project (R3V Cloud's hosted
teams) has `objects/` and `chunked/` under `projects/<pid>/` instead.

Hashes are SHA-256, as 64 lowercase hex digits. Every record, tree,
file and piece is named by the hash of its own bytes (a file's contents:
of the file itself, whatever the stored bytes); check each when reading.

## A version: `projects/<pid>/snapshots/<id>.json`

JSON, written indented with two spaces and a final newline; `<id>` is the
SHA-256 of these exact bytes (keep them as read: re-encoding changes the
id).

| Field | |
|---|---|
| `version` | the record's format: 1 (before branches), 2 now. Higher: written by a newer R3V; don't read it |
| `parents` | version ids: none for a project's first version, one, or two (or more) for a merge (the first: the branch merged into) |
| `author`, `author_id` | the member's name at the time, and id (32 hex; absent in old records) |
| `time` | RFC 3339, UTC |
| `message` | its first line is the title; the details follow a blank line |
| `branch` | the key of the branch it was made on (format 2; absent in format 1). The names people see are the branch records' (docs/design/branches.md) |
| `actor` | an agent made it for the member: `kind` (`agent`), `tool` (`claude-code`), `model`, `session`; absent: the person |
| `files`, `size` | how many files, how many bytes: for display without reading the trees |
| `tree` | the project folder's tree (below); a record without one isn't a version |
| `external` | samples a Live Set uses from outside the project: `[{path, hash, size}]`, `path` as on the computer that saved it |
| `packs`, `missing` | Live packs referenced; samples that were missing when it was saved |

Fields may be added (keep what you don't know when rewriting a record:
R3V never rewrites version records).

## A branch: `projects/<pid>/branches/<key>`

The body is the id of the branch's latest version (64 hex). R3V moves it
with conditional writes; `main` is the default branch.

## A tree: `objects/<ab>/<rest>`

A folder's list, one line per entry, sorted by name (bytes), each line
ending in `\n` (an empty folder: no bytes; empty folders aren't stored
except the project folder):

```
d <tree hash> <name>
f <content hash> <size> <name>
```

`d` is a subfolder (its own tree), `f` a file (its contents' hash and its
size in bytes). Names hold no `/` and may hold spaces and any UTF-8. A
file's path is its folders' names and its own, joined by `/`. Trees are
stored as they are (no header), but read them through the blob layer
(below) like contents.

## A file's contents: `objects/<ab>/<rest>`

The stored bytes are either the file as it is (all contents from before
compression), or start with the five bytes `FF 52 33 56 01` (`"\xffR3V\x01"`)
and a mode byte:

| Mode | Then |
|---|---|
| `z` | a zstd stream of the file |
| `s` | the file as it is (a file that itself starts with those five bytes) |
| `c` | a zstd stream of a **chunk list**: the file is kept as pieces |
| other | written by a newer R3V: say so, don't guess |

A chunk list is text: a first line `r3v-chunks 1 fastcdc-gear <min> <avg>
<max>` (the cutting's sizes), then a line `<piece hash> <size>` per piece,
in order. Each piece is stored at `objects/<ab>/<rest>` like a small file
(as it is, `z` or `s`; never `c`), and the file is the pieces joined.
Files of 16 MB and more are kept this way (docs/design/chunked-files.md);
the cutting itself matters only to writers.

## Records beside them

`team.json`, `members/<id>.json` and `project.json` are JSON R3V rewrites
whole, keeping fields it doesn't know (`internal/jsonx`); their fields are
in `internal/remote/backend.go` (`TeamInfo`, `Member`, `Project`).
