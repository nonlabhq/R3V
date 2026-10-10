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
untick them in the app's list. On the command line a commit takes them all.

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

**No detached HEAD.** Going to an older version (`r3v checkout <id>`, the
app's **Go to this version**) leaves you "on an older version" of your
branch: you can look, start a branch from there, or **make it the latest
version** (a new version with its files). `r3v checkout latest` goes back.

**No stash: changes are parked.** On the Nightly channel, switching branch
or going to a version with uncommitted changes parks them with the place
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
to the label"); it can be renamed or taken away.

**Storage is a bucket.** A team's storage is an S3-compatible bucket you
own (R2, S3, B2, MinIO…) or R3V Cloud; a **connection code** holds its
address and keys. There is no server to run.

**Big files are normal.** No LFS: a 4 GB recording is a file like any
other, stored in pieces, and big files go up in the background before you
commit.

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
| `git checkout <commit>` | `r3v checkout <version>` |
| `git stash` / `git stash pop` | (parked by themselves) `r3v parked` |
| `git branch -d` | `r3v branch archive` (then `branch delete`) |
| `git archive` | `r3v export <version> <folder>` |
| `git fsck` | `r3v verify` |
| `git diff` (a .als) | `r3v diff a.als b.als` |
| `.gitignore` | `.r3v.yaml` (`r3v profile check`) |

For agents: [agents.md](agents.md) (`r3v help agents`).
