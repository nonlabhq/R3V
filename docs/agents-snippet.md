## Version control: R3V

This project is versioned with R3V (`.r3v/`), not git.

- `r3v status --json`: what changed; `r3v update --preview --json`: what teammates saved.
- `r3v save -m "message" --json` saves a version and shares it with the team.
- Commands never prompt: on `merge_conflict` (exit 3) show `error.conflicts` to the user and rerun with
  `--strategy ours|theirs|both` as they decide; on `set_open_in_live` (exit 4) ask them to close the set in Ableton Live.
- Never use `--force` without asking, never print connection codes (they hold storage keys), don't edit `.r3v/`.
- Full guide: `r3v help agents`.
