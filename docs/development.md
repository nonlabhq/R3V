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
- `internal/project/crash_test.go` cuts a share off after each of its writes in turn and checks a teammate still gets a whole version.
- `internal/remote/backendtest` is a contract test every storage backend must pass. It runs against an in-memory bucket and a fake S3; to run it against a real bucket:

  ```
  set R3V_TEST_STORAGE=<connection code>
  go test ./internal/remote -run Live -v
  ```

  It has been run against Versity S3 Gateway and Cloudflare R2.
- Diff and merge must keep producing `testdata/golden` exactly (`go test ./internal/merge`). The golden data pins their output byte for byte; change it only on purpose.

`go test ./internal/merge` compares Go merge/diff/validate output byte-for-byte against the golden data (skipped when absent).

## Design notes

- [docs/als-format-notes.md](als-format-notes.md) — findings about the `.als` format
- [docs/design/storage-backends.md](design/storage-backends.md) — pluggable storage backends
- [docs/cli.md](cli.md) — the command line tool
