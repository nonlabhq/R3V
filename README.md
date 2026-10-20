# R3V

**English** | [繁體中文](README-cht.md)

Version control and teamwork for creative projects: Ableton Live sets, Unity, Unreal and Godot projects, design files and code.

> **Work in progress.** R3V is an early preview for Windows, tested with Ableton Live 12. Expect rough edges and changes before 1.0, and keep your own backups of projects that matter.

## Why R3V

**Made for artists, not for programmers.** Press Ctrl+S in Live as usual; R3V shows what changed, track by track. Commit a version with a sentence, go back to any version, undo one, and try ideas on a branch. When teammates work on the same song, their changes are merged track by track, and R3V only asks when two people changed the same track. No Git knowledge needed.

**Samples come along.** Samples from anywhere on your disk are stored with each version and relinked on your teammates' computers. Big files (stems, videos) are stored in pieces, so a small change doesn't upload the whole file again, and big files go up in the background before you commit.

**Open source, your choice of storage.** R3V is free and licensed under Apache-2.0. A team keeps its work on **R3V Cloud** (sign in and start; in a closed beta) or in an S3-compatible bucket of its own (Cloudflare R2, Amazon S3, MinIO, …), with no server of ours in between. A small team on its own bucket usually stays within Cloudflare R2's free allowance, and nobody has to keep a computer running.

## Limitations

- **Windows only** for now (macOS is planned).
- **Plugins are not synced.** R3V doesn't copy plugins, and it can't collect samples a plugin loads from outside the project (e.g. inside Kontakt or Serum). If your teammates don't have the same plugins, freeze those tracks before you share, or use Live's own devices, which sync completely.
- **On a team's own bucket, everyone with the connection code has full access.** The code contains the storage key: anyone who has it can read, change and delete all of the team's work. Share it privately, only with people you trust. If it leaks, make a new key and send the new code. (On R3V Cloud, people join with an invitation and sign in: there is no key to pass around.)

## Getting started

Download `R3V-<version>-setup.exe` from the [Releases](../../releases) page and run it (Windows 10 21H2 or later, or Windows 11; no administrator rights needed). The installer is not code-signed yet: if Windows shows "Windows protected your PC", click **More info → Run anyway**. R3V updates itself after that.

Every project lives in a team's storage, so its versions are safe if a drive fails. **On your own?** Create a team of one.

**On R3V Cloud:** choose **R3V-Cloud**, sign in with your browser, then create a team (the closed beta needs an invitation) or join one with an invitation link.

**On your own bucket (one person sets it up):** choose **Create a team** and follow the steps to create a Cloudflare R2 bucket and key (about 5 minutes), or enter any other S3-compatible storage. R3V checks it and gives you a **connection code** to send to your teammates, who choose **Join a team** and paste it.

Then download the team's projects or add your own. The [team setup guide](docs/team-setup.md) has the details and everyday use.

## More

- [Team setup guide](docs/team-setup.md)
- [AI agents on your project](docs/agents-quickstart.md)
- [Command line tool](docs/cli.md)
- [R3V for Git users](docs/git-users.md) and the [glossary](docs/glossary.md)
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
