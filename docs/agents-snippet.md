## Version control: R3V

This project is versioned with R3V (`.r3v/`), not git. Full guide: `r3v help agents`.

- `r3v status --json`: what changed; `r3v update --preview --json`: what teammates shared on your
  branch. `r3v update` only gets your branch's versions: in a workspace, `base` in `status --json`
  says the branch you came from has new ones; `r3v merge <that branch>` brings them in.
- `r3v commit -m "Title" -m "Details" --json` commits a version and shares it with the team
  (no separate push). One commit per piece of work; the title says what, the details why. Other
  changes in the folder that aren't this work: list the work's files and folders after the flags
  (`r3v commit -m "…" src/menu`), the rest stays uncommitted. Unfinished work isn't committed to
  be merged later ("WIP" versions): it stays uncommitted (switching parks it) until it's done.
- Steps let the user go back to any point of your work. Claude Code set up with
  `r3v agent setup` makes them with its hooks: nothing to do. Any other tool: begin each task with
  `r3v step start -m "what you're about to do" --json`, end it with
  `r3v step end -m "what you did" --json`, then commit if the work is finished (a commit also
  ends the task left open). Never run `r3v step back` yourself.
- Commands never prompt: on `merge_conflict` (exit 3) read `error.conflicts`; code conflicts come
  hunk by hunk (`text:<path>#<n>`): decide each with `--resolve <key>=ours|theirs|both`, or give the
  file as it should be with `--resolved <path>=<file>`; ask the user when unsure. A lockfile that
  won't merge: take one side, then regenerate it (`npm install`, `cargo update -w`).
- On `set_open_in_live` (exit 4) ask the user to close the set in Ableton Live.
- Working in parallel (a second task at once, a subagent): `r3v workspace add --branch NAME
  --agent <your tool> --json` makes the folder, its branch and the agent setup in one go; work
  there. Never `git worktree`: R3V doesn't see it (its work isn't committed, shared or merged).
- Working beside other agents, each in a workspace of its own (`r3v workspace list --json`): stay in
  yours. A change refused by the hook, or `files_locked_by_agent`, means another agent or the user is
  on that file (textures, models, sounds): don't wait or retry, do other work and tell the user. The
  user merges your branch; don't merge into it yourself unless asked. After running the tests or a
  build, record what they said: `r3v result tests pass -m "cargo test: 214 passed"` (or `fail`).
- What is tracked: `.r3v.yaml` (syntax: `r3v help rules`). Leave out caches, renders and build
  outputs (`rules: - ignore: "cache/"`); `r3v rules explain <file>` says why a file is tracked.
  `target/` matches at any depth; a pattern with a `/` inside starts at the project folder
  (`**/src-tauri/gen/` for any depth).
- A new project: `r3v teams`, then `r3v init --team NAME`: never ask for a connection code.
  `connected: false` in `r3v status --json`: the versions stay on this computer; `next` says how to
  connect it (ask the user which team).
- Once per folder: `r3v agent setup --tool <your tool> [--model <model>]` (`claude-code`, `codex`,
  `opencode`, `gemini-cli`, `cursor`, `pi`...), so the versions you commit say an agent made them.
- Never use `--force` without asking, never print connection codes (they hold storage keys), don't
  edit `.r3v/`, and keep it out of formatters and linters (Biome `"!**/.r3v"`, `.prettierignore`).
