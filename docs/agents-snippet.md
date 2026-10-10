## Version control: R3V

This project is versioned with R3V (`.r3v/`), not git. Full guide: `r3v help agents`.

- `r3v status --json`: what changed; `r3v update --preview --json`: what teammates shared.
- `r3v commit -m "Title" -m "Details" --json` commits a version and shares it with the team
  (no separate push). One commit per piece of work; the title says what, the details why.
- Commands never prompt: on `merge_conflict` (exit 3) read `error.conflicts`; code conflicts come
  hunk by hunk (`text:<path>#<n>`): decide each with `--resolve <key>=ours|theirs|both`, or give the
  file as it should be with `--resolved <path>=<file>`; ask the user when unsure. A lockfile that
  won't merge: take one side, then regenerate it (`npm install`, `cargo update -w`).
- On `set_open_in_live` (exit 4) ask the user to close the set in Ableton Live.
- Working beside other agents, each in a workspace of its own (`r3v workspace list --json`): stay in
  yours. A change refused by the hook, or `files_locked_by_agent`, means another agent or the user is
  on that file (textures, models, sounds): don't wait or retry, do other work and tell the user. The
  user merges your branch; don't merge into it yourself unless asked.
- What is tracked: `.r3v.yaml` (syntax: `r3v help rules`). Leave out caches, renders and build
  outputs (`rules: - ignore: "cache/"`); `r3v rules explain <file>` says why a file is tracked.
  `target/` matches at any depth; a pattern with a `/` inside starts at the project folder
  (`**/src-tauri/gen/` for any depth).
- Never use `--force` without asking, never print connection codes (they hold storage keys), don't
  edit `.r3v/`, and keep it out of formatters and linters (Biome `"!**/.r3v"`, `.prettierignore`).
