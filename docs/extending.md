# Extending R3V

R3V can be built with extensions that live in another repository: more presets (tools R3V understands), merge handlers, running-tool checks and backends. Extensions use the `github.com/nonlabhq/r3v/ext` package and run R3V's own command line tool and desktop app. Nothing in this repository needs to be forked.

## What an extension can add

| Function | Adds | Used by |
| --- | --- | --- |
| `ext.RegisterPreset(yaml)` | A preset, in the format of [`internal/profile/presets`](../internal/profile/presets) and [profiles.md](profiles.md). | Detection, `presets:` in `.r3v.yaml` |
| `ext.RegisterMerge(name, fn)` | A merge handler: given base, yours and theirs, it returns the merged file, or reports that the changes collide (the file is then chosen whole). | A preset's `handlers: - merge: name` |
| `ext.RegisterRunning(name, fn)` | A check of whether a tool has the project open. R3V doesn't rewrite files while it does. | A preset's `running: name` |
| `ext.RegisterTeamFeature(name)` | A team feature this build understands: something a team turns on that changes what it stores. R3Vs without it stop before working with such a team. | [design/channels.md](design/channels.md) |
| `ext.RegisterBackend(prefix, open)` | A kind of team backend for addresses starting with `prefix` (e.g. `rtdb+https://`). It implements `ext.Backend`, and optionally `ext.Capable` to offer features such as locks. | Team addresses and connection codes |

The app reads a backend's capabilities (`Capabilities()`), so features that need them can appear only for teams whose backend offers them.

`ext` is the stable surface. Everything under `internal/` may change.

## A build with extensions

An extension is a Go module that requires this one and has its own entry points:

```
my-extension/
  go.mod          module my-extension
                  require github.com/nonlabhq/r3v v0.0.0
                  replace github.com/nonlabhq/r3v => ../R3V   (a checkout of this repository)
  unity/unity.go  registers a preset, handlers… in init()
  cmd/cli/main.go      import _ "my-extension/unity"; cli.Main()
  cmd/desktop/main.go  import _ "my-extension/unity"; desktop.Run()
```

```go
// cmd/desktop/main.go
package main

import (
	"github.com/nonlabhq/r3v/desktop"

	_ "my-extension/unity" // registers what it adds
)

func main() { desktop.Run() }
```

The desktop app embeds this repository's built frontend (`desktop/frontend/dist`), so build the frontend here first (`npm run build` in `desktop/frontend`). Then build the extension's entry points as usual. On Windows, `wails3 generate syso` adds the icon and version info.

## Compatibility

Teams may mix builds with and without an extension, so storage stays compatible both ways:

- A backend's extra data (e.g. locks) must not get in the way of builds that don't know about it.
- A preset the other build doesn't know is an error in `.r3v.yaml` (`presets:`). Projects that need an extension's preset should set `requires:` accordingly, or be used only with builds that have it.

[ext_test.go](../ext/ext_test.go) is a complete example: a made-up tool with its own preset and a line merge, used by two computers.
