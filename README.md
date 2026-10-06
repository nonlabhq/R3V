# R3V

**English** | [繁體中文](README-cht.md)

Version control and teamwork for creative projects: Ableton Live sets first, with Unity, Unreal and Godot projects in testing.

> **Work in progress.** R3V is an early preview for Windows, tested with Ableton Live 12. Expect rough edges and changes before 1.0, and keep your own backups of projects that matter.

## Why R3V

**Made for artists, not for programmers.** Press Ctrl+S in Live as usual; R3V shows what changed, track by track. Commit a version with a sentence, go back to any version, undo one, and try ideas on a branch. When teammates work on the same song, their changes are merged track by track, and R3V only asks when two people changed the same track. No Git knowledge needed.

**Samples come along.** Samples from anywhere on your disk are stored with each version and relinked on your teammates' computers. Big files (stems, videos) are stored in pieces, so a small change doesn't upload the whole file again, and big files go up in the background before you commit.

**Open source, with storage you own.** R3V is free and licensed under Apache-2.0. Your team's work lives in your own S3-compatible bucket (Cloudflare R2, Amazon S3, MinIO, …), not on our servers. A small team usually stays within Cloudflare R2's free allowance, and nobody has to keep a computer running.

## Limitations

- **Windows only** for now (macOS is planned). Unity, Unreal and Godot projects are on the Nightly channel (Settings › Updates) while they're tested.
- **Plugins are not synced.** R3V doesn't copy plugins, and it can't collect samples a plugin loads from outside the project (e.g. inside Kontakt or Serum). If your teammates don't have the same plugins, freeze those tracks before you share, or use Live's own devices, which sync completely.
- **Everyone with the connection code has full access.** The code contains the storage key: anyone who has it can read, change and delete all of the team's work. Share it privately, only with people you trust. If it leaks, make a new key and send the new code.

## Getting started

Download `R3V-<version>-setup.exe` from the [Releases](../../releases) page and run it (Windows 10 21H2 or later, or Windows 11; no administrator rights needed). The installer is not code-signed yet: if Windows shows "Windows protected your PC", click **More info → Run anyway**. R3V updates itself after that.

Every project lives in a team's storage, so its versions are safe if a drive fails. **On your own?** Create a team of one.

**Start a team (one person):** choose **Create a team** and follow the steps to create a Cloudflare R2 bucket and key (about 5 minutes), or enter any other S3-compatible storage. R3V checks it and gives you a **connection code** to send to your teammates.

**Join a team:** choose **Join a team** and paste the connection code you were sent. Then download the team's projects or add your own.

The [team setup guide](docs/team-setup.md) has the details and everyday use.

## More

- [Team setup guide](docs/team-setup.md)
- [Command line tool](docs/cli.md)
- [R3V for AI agents](docs/agents.md) (Claude Code, Codex, Cursor…)
- [Building and development](docs/development.md)

Feedback and bug reports are welcome in [Issues](../../issues).

## How it's built

R3V is developed with AI coding assistants (Claude), directed and reviewed by its maintainer: every change is read, tried and decided on by a person. Commits are made by the maintainer and don't carry AI co-author lines; this note says it once for all of them.

Because R3V looks after people's work, it leans on checks rather than trust:

- The merge and diff of Live sets are pinned by golden files; stored formats (content hashes, chunk boundaries, version records) have golden tests and never change in ways older versions can't read.
- End-to-end tests run real teams against real cloud storage (save, update, conflicts, interrupted uploads, restores).
- Nothing rewrites a project file while its program has it open, and nothing deletes a teammate's work: updates keep your uncommitted changes, and storage cleanup and restores only touch what no version uses or what is missing.

See [Building and development](docs/development.md), the design notes in [docs/design](docs/design) and the [UX principles](docs/ux-principles.md).

## License

[Apache-2.0](LICENSE). See [NOTICE](NOTICE).
