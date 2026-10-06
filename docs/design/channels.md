# Stable and Nightly

Status: built.

One repository, one `main`, two release lines:

- **Nightly** is `main` as it is, built with the `nightly` build tag: the
  project kinds still in testing (Unity, Unreal, Godot, code and design files, in
  `presets/`) and whatever else is behind the tag.
- **Stable** is cut from `main` when it is ready, built without the tag:
  Ableton Live projects on the team's own storage. A fix for a Stable
  release that can't wait goes on a `release/x.y` branch and back to `main`.

A feature graduates by moving out from behind the tag, not by moving code
between branches.

## Builds and versions

```
scripts\build-windows.ps1 -MinVersion 0.1.0                    # Stable  0.1.0
scripts\build-windows.ps1 -MinVersion 0.1.0 -Channel nightly   # Nightly 0.1.0-nightly.202610061200
```

`internal/version.Version` is the release a Nightly leads up to; the build
script adds `-nightly.<UTC time>` (`version.Build`, set with `-ldflags -X`),
and `version.Channel` comes from the build tag. Versions sort as semver:
`0.1.0-nightly.x` < `0.1.0-nightly.y` < `0.1.0` < `0.2.0-nightly.x`. So
right after cutting a Stable release, `main` moves to the next version.

Windows file properties only take numbers: the installer and `R3V.exe`
carry `0.1.0.0` (`NUMVER`), the installer's name the full version.

## Updates: one app, two feeds

The channel is a setting (Settings → Updates → Channel, kept in teams.json
as `channel`; empty means the build's own). Stable and Nightly are the same
app: same install folder, same settings, so both must read what either
writes.

| Chosen | Feed | Notes |
|---|---|---|
| Stable | GitHub releases (`update.json`) | From a Nightly: it stays until a Stable release sorts after it (0.1.0 after 0.1.0-nightly.x). Never back to an older version. |
| Nightly | `nightly.json` in the releases bucket (`update.NightlyFeed`), published with `go run ./cmd/publish <installer> <version> -min 0.1.0` | From Stable, the latest Nightly is offered even when it sorts before this version (`update.SwitchTo`). |

Both feeds are signed with the release key.

## Teams with Stable and Nightly members

Stored data is forever, and a team can mix channels. So a Nightly feature
that changes what a team stores is a **team feature**: the build registers
it (`ext.RegisterTeamFeature`), and it is only used once the team turns it
on (`remote.EnableFeature`, listed in team.json as `features`). From then
on, a R3V that doesn't know it stops before working with the team
(`remote.CheckFeatures`, from `Repo.Client` and the team overview) and says
to update or switch to Nightly (CLI: `team_needs_features`, exit 6).

Project kinds a Stable build doesn't know say so too: initializing a Unity,
Unreal or Godot folder, or a `.r3v.yaml` naming a Nightly preset, fails with
"needs R3V's Nightly channel" (CLI: `needs_nightly`, exit 6).
