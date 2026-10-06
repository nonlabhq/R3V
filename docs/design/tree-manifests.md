# Tree manifests

Status: built. Measured results at the end.

## Why

A version record (manifest) that listed every file of the project would be
fine for a song (a few hundred files) and expensive for a game project:

| 100,000-file project, flat list | cost                        |
|---------------------------------|-----------------------------|
| one version record              | 16.5 MB                     |
| 31 versions on disk             | 512 MB of records           |
| a commit changing 10 files      | writes (and shares) 16.5 MB |
| comparing two versions          | reads both whole records    |

Most of every such record repeats the one before it.

## The idea

Like git: each folder is its own small record (a *tree*) listing its files
and its subfolders' trees, named by the hash of its content. A version
points to the tree of the project folder. A commit writes new trees only for
folders that changed, and the folders above them; everything else is shared
with the previous version.

A commit that changes 10 files in a 100,000-file project writes about 30
trees (each folder on the way down), typically tens of KB in all, instead of
16.5 MB. Comparing two versions skips every folder whose tree hash is the
same on both sides.

## Format

### Version record

```json
{
  "version": 1,
  "parents": ["…"],
  "author": "…", "author_id": "…", "time": "…", "message": "…",
  "tree": "<hash of the project folder's tree>",
  "files": 100031,
  "size": 536870912,
  "external": [ … ], "packs": [ … ], "missing": [ … ]
}
```

- `tree` names the project folder's tree. `files` and `size` (count and
  bytes of all files) are for display without reading the trees.
- `external`, `packs` and `missing` stay as they are (Live sets' samples
  outside the project: few, and not in any folder of it).
- The id is the SHA-256 of the record's bytes.

### Tree

Text, one line per entry, sorted by name (byte order), `\n` after every
line:

```
d <tree hash> <name>
f <content hash> <size> <name>
```

- The name is the rest of the line (it may contain spaces). Names with a
  newline can't be stored; Windows doesn't allow them, and a commit with one
  on macOS fails with a clear message.
- An empty folder isn't stored (only files are tracked).
- The tree's hash is the SHA-256 of its bytes, like file contents.

Text rather than JSON: about half the size, and quick to read and write.

## Where trees live

- **Team storage:** with file contents, under `objects/<ab>/<rest>`. They
  are content-addressed like files, so they go up with the same batched
  "which are missing" check and parallel uploads, after the files they list
  and before the version record that names them.
- **This computer:** `.r3v/trees/<ab>/<rest>`, apart from file contents.
  Trees are never pruned: they are the history. A downloaded version's
  trees are stored before its record, so a version stored here can always
  be read.
- **Memory:** parsed trees are cached by hash (shared by all projects in
  the process). Versions share nearly all their trees, so reading the next
  version of a project reads only the trees that changed.

## Newer formats

`version` is the record's format (`manifest.Format`, now 1). A record of a
newer format than this R3V knows is refused ("update R3V"), never read as
something else. Versions are never rewritten.

## In the code

- `manifest`: records (`Tree`, `FileCount`, `TotalSize`); trees
  (`EncodeTree`, `ParseTree`, `BuildTrees`, `Flatten`). In memory a version
  has the flat list of files, filled from its trees on load, so merge,
  checkout and status work on plain file lists.
- `project`: commits write the trees of changed folders (a parent's trees
  are known stored); sync uploads and downloads trees; history and ancestry
  read records only; cleanup (GC) reads each folder's tree once across all
  versions.
- `.r3v/index.bin` is the local stat cache, a compact binary file (as JSON
  it was 17.7 MB at 100,000 files).

## Results (100,000 files)

|                                | flat list | trees                |
|--------------------------------|-----------|----------------------|
| record per commit (10 changes) | 16.5 MB   | 307 B + 21 trees, 147 KB |
| all trees of the project       | -         | 1,024 trees, 9 MB    |
| commit, 10 files changed       | 0.43 s    | 0.35 s               |
| reading a version              | ~0.2 s (16.5 MB JSON) | 0.09 s (cached trees), 0.14 s cold |

The first commit of a project writes all its trees (about 1.5 s
here); later ones write only what changed.

It also prepares partial download: a folder not downloaded is one tree hash
the version keeps from its parent.

## Limits

- A folder with very many files (say 20,000 textures in one folder) has one
  big tree, rewritten whenever any file in it changes (about 2 MB). If that
  turns out to matter, big trees can be split into fixed-size parts later
  without changing anything above them.
- Trees no version uses any more (after a branch is deleted) stay on disk;
  they are small.
