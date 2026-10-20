# R3V for Git users

R3V is version control for creative projects (Ableton Live sets, Unity,
Unreal, Godot), made for people who have never used Git. If you have, most
of it will feel familiar; this page is about where it doesn't. The words
side by side: [glossary.md](glossary.md).

## What's the same

Versions with parents, branches, merges, a history you can go back to.
Content is stored by its hash, versions are never rewritten, and a version
id is a hash (any unique prefix works on the command line).

## What's different, and why

**Commit is commit and push.** `r3v commit -m "…"` (the app's **Commit &
Share**) makes a version and shares it with the team in one step. If
teammates shared on your branch meanwhile, their versions are merged in
first; when you both changed the same track or file it stops, changing
nothing, and asks. There is no local branch that drifts from the team's:
your branch is the team's branch. (`commit --local` commits on this
computer only; the next commit shares it.)

**No staging area.** A commit takes your changes; to leave some out,
untick them in the app's list. On the command line a commit takes them all,
or only those in the paths you give (`r3v commit -m "…" src/menu`).

**Merges understand Live sets.** A set is merged track by track (devices,
clips, automation), not line by line. A conflict is a track, or a whole
file that can't be merged (a .uasset, a .psd); you choose yours, theirs or
both. There are never conflict markers in your files. Files that can't be
merged can be **locked** instead (on R3V Cloud).

**Get updates is never automatic.** Teammates' versions come into your
files only when you ask (`r3v update`, the app's **Get updates**), and
your uncommitted changes stay as they are, merged with theirs. `r3v update
--preview` is `git fetch` plus a look at what would change.

**A version keeps its branch.** Every version records the branch it was
made on, for good, and every branch the branch it was made from. So the
history is drawn as a tree that doesn't move as you switch, and "which
branch was this made on" always has one answer. A commit on the wrong
branch stays there (merge it elsewhere); before committing, the app can
bring changes to another branch.

**Merging another branch always makes a version.** As `--no-ff` would.
Updating your own branch with teammates' versions still fast-forwards.

**No detached HEAD.** Going to an older version (`r3v goto <id>`, the
app's **Go to this version**) leaves you "on an older version" of your
branch: you can look, start a branch from there, or **make it the latest
version** (a new version with its files). `r3v goto latest` goes back.

**No stash: changes are parked.** Switching branch or going to a version with uncommitted changes parks them with the place
they were made (a branch's latest, or an older version) and brings them
back when you return, merged with what came since. `r3v parked` lists
them; `parked bring` brings a set into the branch you're on.

**Branches are archived, not deleted.** Archiving puts a branch away (with
the branches made from it) and it comes back whole; only an archived
branch can be deleted for good, and its versions stay either way. Branch
names are any text, in any language.

**No rebase, no force push, no history rewriting.** The one exception:
your own latest version, when no one has it yet, can be taken back (the
app's **Undo this commit**); otherwise undoing makes a new version.

**Milestones, not tags.** A milestone names a version for the team ("Sent
to the label"): only a shared version takes one, everyone sees it at once
(nothing to push), and it can be renamed, given a note or taken away
without touching the version. `r3v tag` and `r3v tags` work too.

**Undoing a merge makes a version.** `r3v undo-merge <merge>` is `git
revert -m 1`: a new version without what the merge brought in, the
history kept. What changed since stays; where the same thing changed again
you choose `keep` (as it is now) or `undo` (as before the merge), as in a
merge.

**Discard is restore.** `r3v discard` (the app's **Discard changes**) puts
the files back as in the version you're on, as `git restore .` or `git
reset --hard` would: changed files back, deleted ones back, files added
since gone; what the rules leave out stays, as `git clean` isn't run. With
paths, only those (`git restore <path>`); `--preview` lists what would go.
The app asks first; an agent asks the user.

**Workspaces, not worktrees.** `r3v workspace add --branch NAME` is `git
worktree add -b`: another folder with the project, on a branch of its own,
sharing the main copy's store. Unlike a worktree, its branch is the team's
(its commits share), it is made from the files on this computer (nothing
downloaded; block-cloned on a Dev Drive), and it can be an AI agent's
(`--agent`), with agent locks on files that can't be merged. `workspace
remove` refuses while its work isn't merged.

**Steps have no Git counterpart.** While an AI agent works, R3V keeps the
files after each of its moves, under the prompt that asked for it (as
Claude Code's checkpoints, or `jj`'s operation log): `r3v steps`, `r3v step
back <n>`. They stay on this computer, never in the history.

**Storage is a bucket.** A team's storage is an S3-compatible bucket you
own (R2, S3, B2, MinIO…) or R3V Cloud; a **connection code** holds its
address and keys. There is no server to run.

**Big files are normal.** No LFS: a 4 GB recording is a file like any
other, stored in pieces, and big files go up in the background before you
commit.

## R3V and git in one folder

A code project can be a git repository and an R3V project at once:
`.git/` and `.r3v/` side by side, each keeping its own history.

- R3V never tracks `.git` folders. Add `.r3v/` to the `.gitignore` (R3V's
  own data, not the project's); `.r3v.yaml`, the project's rules, can be
  committed to both.
- A folder with `.git/` or a `.gitignore` is a code project for R3V
  (unless it is also, say, a Unity project): its
  rules have `gitignore: true`, so R3V tracks what git tracks (each
  `.gitignore` read as git reads it), and leaves out build outputs and
  tool folders git might keep (`node_modules/`, `target/`, `dist/`…).
  Your own `rules:` in `.r3v.yaml` come first ([profiles.md](profiles.md)).
- The two don't know each other: a `git commit` isn't an R3V version, nor
  the other way round, and switching branches in one doesn't switch the
  other. Pick one to switch with, and commit in both when both should keep
  the point.

## Commands, side by side

| Git | R3V |
|---|---|
| `git init` | `r3v init` |
| `git clone <url>` | `r3v clone <connection-code> <project>` |
| `git status` | `r3v status` |
| `git add -A && git commit -m … && git push` | `r3v commit -m …` |
| `git commit` (no push) | `r3v commit --local -m …` |
| `git fetch && git log ..@{u}` | `r3v update --preview` |
| `git pull` | `r3v update` |
| `git log` | `r3v log` |
| `git switch <branch>` | `r3v switch <branch>` |
| `git switch -c <branch>` | `r3v branch new <name>` |
| `git merge --no-ff <branch>` | `r3v merge <branch>` |
| `git checkout <commit>` | `r3v goto <version>` |
| `git stash` / `git stash pop` | (parked by themselves) `r3v parked` |
| `git branch -d` | `r3v branch archive` (then `branch delete`) |
| `git restore .`, `git reset --hard` | `r3v discard` |
| `git revert -m 1 <merge>` | `r3v undo-merge <merge>` |
| `git tag -a <name>` | `r3v milestone add <name>` |
| `git worktree add -b <branch> <folder>` | `r3v workspace add <folder> --branch <branch>` |
| `git archive` | `r3v export <version> <folder>` |
| `git fsck` | `r3v verify` |
| `git diff` (a .als) | `r3v diff a.als b.als` |
| `.gitignore` | `.r3v.yaml` (`r3v rules`) |

For agents: [agents.md](agents.md) (`r3v help agents`).
