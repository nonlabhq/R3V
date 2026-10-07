# Development

How R3V is built, how to build it, and how it is tested. For using R3V, see the [README](../README.md).

## Build

Requirements: Go 1.27+, Node.js, the Wails v3 CLI (`wails3`) and, for the installer, NSIS.

```
go build -o r3v.exe ./cmd/r3v                       # command line tool
go test ./...                                             # all Go tests
powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1   # -> dist\R3V-<version>-setup.exe
```

The release number lives in `internal/version/version.go`; the build script reads it for the installer name and the file properties of `R3V.exe`.

Two release lines come from `main`: Stable (the default build) and Nightly (`-tags nightly`; the build script's `-Channel nightly`), which adds the project kinds still in testing from `presets/`. Run their tests with `go test -tags nightly ./...` too. See [design/channels.md](design/channels.md).

The desktop app is in `desktop/` (Go + Svelte 5 frontend in `desktop/frontend/`). After changing Go methods the frontend calls, regenerate the TypeScript bindings:

```
cd desktop
wails3 generate bindings -ts
cd frontend && npx svelte-check && npm run build
```

To try the desktop UI in a browser (no native dialogs), build it in server mode:

```
cd desktop
go build -tags server -o bin/R3V-server.exe ./cmd/r3v-desktop
set WAILS_SERVER_PORT=8765
set R3V_CONFIG_DIR=%TEMP%\r3v-dev      # a fresh settings folder (shows onboarding)
set R3V_DEV_PICK_DIR=C:\path\to\a\folder   # what the folder picker returns
bin\R3V-server.exe                         # then open http://localhost:8765/
```

For the app's look, `go run ./desktop/cmd/r3v-demo` (from the repository
root, after building `bin/R3V-server.exe`) runs that build on a made-up team
instead: teammates, a branch, a version to get and unsaved changes, made
afresh each time in a temporary folder, with the team's storage in memory.
Build it with `-tags server,nightly` to get the Style lab (Ctrl+Alt+L, in
every Nightly build): looks to try (`src/lib/themes.ts`) and knobs for
spacing, corners, borders, shadows, see-through surfaces, accent and font.
"Copy as CSS" gives the tokens of what is on screen, to make it the default
in `src/tokens.css`.

## Layout

- `cmd/r3v` — command line tool (package `cli`; `--json` results and error codes in `cli/output.go`)
- `docs/` — also a Go package: embeds `agents.md` for `r3v help agents`
- `desktop/` — Windows desktop app (Wails v3, Svelte 5); installer in `desktop/build/windows/installer.nsi`
- `internal/xmltree` — ordered XML tree with byte-exact round-trip of Live's output
- `internal/als` — Live Set model, content fingerprints (noise-aware), structural validator
- `internal/diff` — track-level semantic diff
- `internal/merge` — track-level 3-way merge (tracks, placement, order, sends, globals) with id repair
- `internal/store` — content-addressed blob store (SHA-256)
- `internal/project` — versions, status, history, restore with sample relinking; `sync.go` does save/update/clone and merges
- `internal/teamwatch` — background watcher: backs up unsaved work, teammates' edits (soft locks), new versions
- `internal/livecheck` — detects a running Live before rewriting sets
- `internal/manifest` — version records and folder trees
- `internal/remote` — the `Backend` interface and its implementation on S3-compatible object storage (extensions can register more)
- `internal/teams` — per-user team store (`%APPDATA%\R3V\teams.json`): team addresses, credentials, project locations
- `internal/version` — release number, channel (Stable or Nightly) and Nightly build stamp
- `presets/` — project kinds still in testing (Unity, Unreal, Godot, code, design files); only Nightly builds import them
- `cmd/publish` — publishes a Nightly installer and its signed feed to the releases bucket
- `testdata/golden` — the expected output of diff and merge: the specification
- `testdata/live/` — sets saved by Ableton Live 12.3.1 (a project, its Backup folder, two hand-made branches): the fixtures, read byte for byte

## Tests

- `go test ./...` runs everything that needs no network; `go test -tags nightly ./...` the Nightly build too.
- `npm test` (in `desktop/frontend`) runs the frontend's tests: logic in `.ts` files and components rendered in jsdom, with the Go bindings mocked (`vi.mock("./api")`; see `KeptSamples.test.ts`).
- Interruptions (see below): `internal/project/crash_test.go` cuts a share off after each of its writes in turn; `resume_test.go` cuts updates and clones off at each of their reads, and resumes cut uploads; `killed_test.go` kills R3V (a child process: no deferred cleanup runs) in the middle of a commit, an update, a clone and a background upload. The fake S3 (`internal/remote/s3test`) has `CutAfter` / `CutReadsAfter` for a lost connection and `OnWrite` / `OnRead` to act at a given request.
- The end-to-end run (real storage, two computers simulated; kept outside this repository) runs in full before every release.
- `internal/remote/backendtest` is a contract test every storage backend must pass. It runs against an in-memory bucket and a fake S3; to run it against a real bucket:

  ```
  set R3V_TEST_STORAGE=<connection code>
  go test ./internal/remote -run Live -v
  ```

  It has been run against Versity S3 Gateway and Cloudflare R2.
- Diff and merge must keep producing `testdata/golden` exactly (`go test ./internal/merge`). The golden data pins their output byte for byte; change it only on purpose.

`go test ./internal/merge` compares Go merge/diff/validate output byte-for-byte against the golden data (skipped when absent).

### Interruptions

The connection can drop and R3V can be closed or crash at any moment. Every step that writes to the team's storage or to the project (commit, share, update, clone, switch, background upload) is tested both ways, failing (a lost connection: every request fails from some point) and killed (the process ends: nothing after runs), at the start, the end and points between. After each, a test checks that:

1. the team's data is whole: a teammate gets a whole version, the one before or the new one;
2. the project's files are as they were, or complete (a switch that stopped half-way says so and can be put back);
3. what the app offers next is safe and finishes the job (no commit offered on a download that didn't finish, say), and doing it again works;
4. doing it again doesn't send again what already went up (a few small records aside);
5. nothing is left behind for good: temporary copies, registrations, locks.

A new step lists its states in between (in its design note or pull request): where it can stop, what is on disk and in storage then, and what the app shows and allows. Reviews ask of each write: what if it stops here?

## Lanes

Work goes in lanes, each a session of its own; only Core merges into `main`.

| Lane | What | Owns | Where in this repository |
|---|---|---|---|
| Core | this repository: the command line, the app, what is stored, releases | `main`, the version, tags, releases, the end-to-end run | the main checkout, short branches `feat/…`, `fix/…`, `ui/…`, `docs/…` |
| Cloud | the private R3V-Cloud service, and its client here (`internal/cloud`, the broker in `internal/remote`) | the R3V-Cloud repository (its `main`, its deployments) | `feat/cloud` only, in `../R3V.wt/cloud` |
| Experiments | a direction being tried | its branch | `exp/<name>`, in `../R3V.wt/<name>`, one worktree each |

### Rules

1. **Only Core merges into `main`**, changes the version, tags and releases
   (after the full end-to-end run). Other lanes hand over; they don't merge,
   not even a small fix in their own package: a change that looks like one
   lane's can reach Stable or the settings both channels share.
2. **New things arrive behind Nightly**: a preset in `presets/`, a build
   tag, or a switch set only in a `nightly` file (as `remote.HostedTeams`
   is), with a Stable test that shows Stable doesn't reach it. Stable and
   Nightly share the settings folder (teams, sessions): what Nightly writes
   there, Stable must be able to ignore safely.
3. **What must not break** (AGENTS.md) is Core's: stored formats, content
   hashes, chunk boundaries, what a team stores, the CLI's `--json` and
   error codes. A lane needing to change one says so in its hand-over;
   Core decides (and whether a release must force updating).
4. **A lane's worktree is its own.** Don't edit another lane's worktree or
   its uncommitted files (generated noise included): tell its session.
5. **Every report names the repository** (R3V or R3V-Cloud), the branch
   and the commit.

### Handing over to Core

1. The lane merges `main` into its branch, then runs `go vet` and
   `go test` both plain and with `-tags nightly`, and, for UI changes,
   `npx svelte-check`, `npm test` and `node scripts/i18n-check.mjs`.
2. It sends Core a message (between sessions) saying:
   - what changed, and why;
   - where the risks are (Stable, shared settings, storage, interruptions);
   - whether it touches anything on the must-not-break list;
   - the tests run, and their results;
   - the R3V-Cloud commit or deployment it goes with, if any.
3. Core reviews it: correctness, the must-not-break list, the
   interruption checks (see Interruptions), and that Stable stays as it
   was.
4. Core fixes what it finds on `fix/<lane>-review` on top of the lane's
   branch and merges both with `--no-ff`; something that needs rethinking
   goes back to the lane instead.
5. Core tells the lane, which merges `main` back and reviews Core's fixes
   (it knows its code best).
6. Releasing stays Core's, after the full end-to-end run.

### Between this repository and R3V-Cloud

- The service's API is R3V-Cloud's `docs/api.md`. A change to it is said
  in the hand-over, and keeps released Nightlies working (or ships with
  them).
- Tests here use fake services; a test against the test service runs only
  when asked (a build tag or an environment variable).
- This repository is public: the client only. The service's code, accounts
  and keys stay in R3V-Cloud. Secrets (sessions, presigned addresses,
  invitation tokens, storage keys) never go into logs, errors or test
  output.

### Experiments

- A branch `exp/<name>` from `main`, in `../R3V.wt/<name>`. It can build
  Nightly installers to try; it doesn't publish them.
- To keep it: rename it `feat/<name>`, put what it adds behind Nightly,
  and hand it over.
- To stop it: say why in its design note (parked, and what would bring it
  back), then remove its worktree; the branch can stay.

### Files every lane touches

- Translations: add keys at the end of every locale; never reorder or
  rewrite others'. Conflicts are Core's to settle at the merge.
- Bindings: commit only the files with real changes (`git diff -w`).
- Notes for one moment (a review's notes): delete them, don't commit them.
  Decisions go in `docs/design/`.

## Design notes

- [docs/als-format-notes.md](als-format-notes.md) — findings about the `.als` format
- [docs/design/storage-backends.md](design/storage-backends.md) — pluggable storage backends
- [docs/design/resolve.md](design/resolve.md) — DaVinci Resolve projects (parked)
- [docs/cli.md](cli.md) — the command line tool
