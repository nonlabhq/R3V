# Project rules (`.r3v.yaml`)

A project's rules say which files are left out of versions. They are in one place: the `.r3v.yaml` file in the project folder, committed with the project, so the whole team follows the same rules (and so can an agent reading the project).

R3V writes the file when a project is added (or opened, when it has none): it names the presets it finds, such as **ableton** for a Live project, marked `# found by R3V`. Change the file freely. In the app: the gear next to the team name → **Rules**.

## Example

```yaml
requires: "0.1.0"        # the oldest R3V that understands this file
presets:                 # which preset applies to which folder
  ./: design  # found by R3V
  "Game/": unity  # found by R3V
  "Music/Theme Project/": ableton  # found by R3V
  "Tools/": none         # not a project of a tool: only the rules below
rules:                   # yours, on top of the presets; later rules win
  - ignore: "Exports/"   # leave every "Exports" folder out of versions
  - ignore: "*.tmp"
  - track: "*.asd"       # keep Live's analysis files after all
```

## Rules

Each rule is either `ignore:` (leave matching files out of versions) or `track:` (keep them, overriding an ignore from the preset or an earlier rule). When several rules match a file, the **last** one decides. Rules come before the preset.

Patterns work like `.gitignore` lines and ignore upper/lower case. The one thing to know: **a pattern with a `/` in it (other than at its end) starts at the project folder**; one without matches at any depth. So `target/` leaves out every `target` folder, but `src-tauri/gen/` only the one at the top: write `**/src-tauri/gen/` for any of them.

| Pattern | Matches |
| --- | --- |
| `*.tmp` | any file named `….tmp`, in any folder |
| `Exports/` | any folder named `Exports`, at any depth, and everything in it |
| `/Notes.txt` | `Notes.txt` in the project folder only |
| `Samples/Recorded/*.wav` | `.wav` files in `Samples/Recorded` of the project folder only |
| `apps/web/dist/` | that one folder, from the project folder |
| `**/Bounces/` | a `Bounces` folder at any depth |
| `**/src-tauri/gen/` | `gen` in any `src-tauri` folder, at any depth |

`r3v rules explain <file>` says which rule decided a file.

### The project's `.gitignore` files

With `gitignore: true` in `.r3v.yaml` (or a preset that asks for it), R3V also follows the project's `.gitignore` files, as git reads them: each applies to its own folder, a deeper one wins over the one above, within a file the last matching line wins, and `!` takes a file back (but not out of a folder that is left out). Your `rules:` still come first.

```yaml
gitignore: true
rules:
  - track: "*.psd"       # a .gitignore leaves them out; keep them in versions
```

A few things are never tracked, whatever the rules say: R3V's own `.r3v` folder, `.git` folders, and system files such as `desktop.ini` and `Thumbs.db`. `.r3v.yaml` itself is always tracked.

## What happens when a rule changes

- Files that a new rule leaves out show up as **no longer tracked** (○) in the Changes tab. When you commit, R3V lists them and asks you to confirm. The next version doesn't have them.
- **Nobody's files are deleted.** When your teammates take in that version, the files stay on their computers, just untracked. Take them out of the rule and they are tracked again.

## Presets

`presets:` says which preset applies to which folder (the app calls it the folder's **project type**: "Ableton Live project", "Code and documents", "Files as they are" for `none`). `./` means the project folder. `none` turns a folder's preset off, so only your rules apply there. A folder `presets:` doesn't name gets no preset.

When a project of a tool turns up in a folder `presets:` doesn't name (a Unity project added to a bigger project, say), R3V suggests its preset: in the app as a note above the changes, asked again before committing (or the tool's caches would go up with the version); on the command line in `r3v status`. Taking it, or saying it isn't one, writes the folder into `presets:` (`r3v rules preset <folder> <preset|none>`). R3V looks up to three folders deep, for the projects of tools (presets of priority 0 and up, such as a Live or Unity project); design files or a `.gitignore` in a folder don't make it a project of its own.

A file without `presets:` gets the presets R3V finds added when the project is opened.

The built-in presets:

| Preset | For | Detected by | Leaves out |
| --- | --- | --- | --- |
| `ableton` | Ableton Live projects | an `Ableton Project Info` folder or a `.als` file | `/Backup/`, `*.asd` |
| `code` | Software, and folders of documents | `.git/`, `.gitignore`, `package.json`, `go.mod`, `Cargo.toml` and the like | what the `.gitignore` files leave out, and tool folders and build outputs (`node_modules/`, `.venv/`, `dist/`, `target/`, `.next/`, `.svelte-kit/`, `coverage/` …), `.env`; merges text files line by line |

A preset can ask for the project's `.gitignore` files (`gitignore: true`), and has a `priority` for detection (0 by default): when several presets recognize a folder, the highest wins. So a folder with a `.gitignore` can be a code project (a low priority) unless it is also, say, a Unity project.

A preset also tells R3V which built-in code handles which files, what to check before rewriting files, and how files are grouped and shown in the app. The handlers a preset can name:

| Handler | Kind | Does |
| --- | --- | --- |
| `ableton-set` | `merge:` | Compares and merges Live Sets track by track; with `samples: ableton`, collects and relinks their samples |
| `text` | `merge:` | Merges text files line by line when both sides changed them; if the same lines changed differently, you choose the whole file |
| `ableton-live` | `running:` | Doesn't rewrite a set while Live has it open |

Files without a merge handler are chosen whole (yours, theirs or both) when both sides changed them. A `.json` file that merging line by line would leave broken (each side valid, the result not) is chosen whole too.

### The project's own merges

`handlers:` in `.r3v.yaml` says how the project's files merge, over what the presets say. Patterns are as in rules, relative to the project folder; when several match a file, the last decides. `merge:` is `text` (line by line, conflicts hunk by hunk) or `whole` (chosen whole, no merge):

```yaml
requires: "0.1.39"
handlers:
  - files: ["*.scene", "script/**/*.md"]
    merge: text
  - files: ["data/generated/"]
    merge: whole
```

R3V before 0.1.39 doesn't know `handlers:` and refuses the file, and Stable refuses it for now: set `requires: "0.1.39"` when you add it.

## Workspaces (Nightly)

An agent's workspace (`r3v workspace add`) gets the project's tracked files. To spare it building from nothing, `.r3v.yaml` can name build folders the rules leave out that a new workspace gets a copy of:

```yaml
requires: "0.1.45"
workspaces:
  copy: ["target/", "**/node_modules/"]   # that folder; any folder of that name
```

They are copied file by file with the system's copy, a block clone on ReFS and Dev Drive volumes (no room taken until a file changes). Links aren't copied (one would point into the main copy): a `node_modules` made by pnpm is mostly links, so install there instead (`pnpm install --offline` is quick from its store). R3V before 0.1.45 doesn't know `workspaces:` and refuses the file: set `requires: "0.1.45"` when you add it.

## File locks

File locking (R3V Cloud teams; see [design/locks.md](design/locks.md)) is set in the team's settings and each project's (Workflow), not in `.r3v.yaml`: a project follows the team's unless an admin sets its own there (on or off, other kinds, paths of its own). See [design/team-settings.md](design/team-settings.md).

R3V before 0.1.75 read a `file_locks:` section here, which could only narrow the team's settings. R3V no longer reads it: a project whose `.r3v.yaml` still has one shows a note, and an admin can move it into the project's settings (which takes the section out of the file: commit that).

## Checking the rules

- The **Rules** dialog in the app shows the preset in use and any problem with `.r3v.yaml`. While the file has a mistake (e.g. a misspelt field), R3V shows the problem and won't commit until it's fixed. Looking at changes still works.
- From the command line, inside the project folder:

  ```
  r3v rules                       # the rules in use, and whether they work
  r3v rules explain Exports/mix.wav Backup
  ```

  `explain` says whether each file is tracked and which rule or preset decided it.

## Older R3V versions

`requires:` names the oldest R3V that understands the file (R3V fills it in when it creates the file). A R3V older than that refuses to commit to the project or take in its versions, and asks to be updated. This way two people never track different files.
