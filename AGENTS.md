# AGENTS.md

For AI agents working on R3V's code. To **use** R3V in a project
(save, update, merge), read [docs/agents.md](docs/agents.md) instead, or run
`r3v help agents`.

## The project

R3V is version control for creative projects (Ableton Live sets, Unity,
Unreal and Godot projects): a Go command line tool (`cmd/r3v`, package `cli`)
and a Windows desktop app (`desktop/`, Wails v3 + Svelte 5). A private build
adds extensions through `ext/`; keep `cli` and `desktop` usable as libraries
(no `os.Exit` outside `cli.Main`).

Two release lines come from `main`: Stable, and Nightly (`-tags nightly`),
which adds the project kinds still in testing from `presets/`. Anything
that changes what a team stores must be a team feature (see
[docs/design/channels.md](docs/design/channels.md)).

[docs/development.md](docs/development.md) has the layout and how to build;
`docs/design/` has the design notes (storage backends, tree manifests,
chunked big files).

## Build and test

```sh
go build ./...
go vet ./...
go test ./...                          # no network needed
go test -tags nightly ./...            # the Nightly build too
cd desktop/frontend && npx svelte-check && npm test   # after any UI change
```

- After changing Go methods the frontend calls: `wails3 generate bindings
  -ts` in `desktop/`, then check `git diff -w` on the bindings before
  committing.
- `testdata/golden` pins the output of diff and merge byte for byte
  (`go test ./internal/merge`); change it only on purpose.

## Conventions

- Code, comments, identifiers, commit messages: English. Plain words in
  messages users see; the app translates its UI (`desktop/frontend/src/lib/
  locales`, English text is the key: `node scripts/i18n-check.mjs` in
  `desktop/frontend` lists what is missing).
- What users see follows [docs/ux-principles.md](docs/ux-principles.md).
- Styles use the design tokens in `desktop/frontend/src/tokens.css`
  (colours by role, type sizes, corners, shadows, layers), not raw values;
  a new colour is a new token there. `npm test` fails on a raw colour or an
  undefined token.
- Match the surrounding code: short doc comments that say why, no
  boilerplate.
- `go vet` and `gofmt` clean.
- The CLI's `--json` output and error codes are an interface (see
  `cli/output.go` and docs/agents.md): add fields freely, don't rename or
  remove them, and keep the codes stable.

## Working in the repository

Several sessions may work at once (in other worktrees or the same checkout),
so:

- **Branches.** Work on a branch (`feat/…`, `fix/…`, `ui/…`) and merge it
  into `main` with `--no-ff`. Merge `main` into a long branch often, so
  conflicts stay small.
- **Stage only your own files**, by name. Never `git add -A` a folder:
  someone else's uncommitted changes may be in it.
- **Commit messages** in plain English, with no AI co-author or "Generated
  with" lines: the README says once how R3V is built.
- **The version** (`internal/version/version.go`) changes only when a
  release is made, on `main`, never on a branch.
- **Translations:** add keys to every locale; don't reorder or rewrite
  others' keys (`node scripts/i18n-check.mjs` must report nothing missing).
- **Bindings:** commit only the generated files with real changes (see
  above).
- Pushing, tagging and releasing are the maintainer's call: do them only
  when asked.

## Things that must not break

- **Stored data is forever.** Content hashes, the blob header
  (`internal/blob`), chunk boundaries (`internal/chunk`, golden test) and
  version formats are read by every R3V after; a change that writes
  something older versions can't read needs a release that forces updating
  (`-MinVersion`).
- **Never lose a user's file.** Anything that rewrites project files checks
  first (Live running, unsaved changes) and fails rather than guesses.
- **Secrets.** Connection codes and storage keys never go into logs, test
  output, commits or docs. The release signing key is never in the repo.
- Storage cleanup (`internal/remote/gc.go`) must never delete something a
  version, a share in progress or a chunk list relies on.
