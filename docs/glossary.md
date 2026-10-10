# Glossary

One word for one thing, in the app (for people), on the command line (for
agents and scripts) and in these documents; and how each compares with
Git's. New text in the app or the CLI uses these words. Git users: see
[git-users.md](git-users.md) for the ideas behind the differences.

The app's words are its English texts (the keys of every translation);
the Traditional Chinese column is what `zh-TW.json` says.

## The model in a paragraph

A **team** keeps **projects** in its storage (its own bucket, or R3V
Cloud). A project is one folder: a Live set and its samples, a Unity,
Unreal or Godot project. Each **commit** makes a **version** of the whole
folder and **shares** it with the team in the same step. A version keeps
the **branch** it was made on, for good; a branch keeps the branch it was
made from. Teammates' versions come into your files only when you **get
updates**, and Live sets are merged track by track. Changes you haven't
committed stay with their place: going to another branch or version
**parks** them, coming back brings them back.

## Words

| Thing | App (English) | App (zh-TW) | CLI | Git | How it differs |
|---|---|---|---|---|---|
| Team | Team | 團隊 | `teams`, `remote`, `connection-code` | a remote (and its host) | A team holds many projects; its storage is a bucket you own, or R3V Cloud |
| Project | Project, Add project | 專案、新增專案 | `init`, `clone` | repository | One folder per project; the rules (`.r3v.yaml`) say what is tracked |
| Connection code | Connection code | 連線代碼 | `connection-code`, `remote <code>` | remote URL + credentials | Holds the storage keys: never shown, logged or committed |
| Download a project | Download | 下載 | `clone` | `git clone` | |
| Version | Version, Versions (tab) | 版本 | `log` | commit | Keeps its branch (`made_on`); the whole folder, never a patch |
| Your changes | Your changes | 你的變更 | `status` | working tree + index | No staging area: untick files in the list to leave them for later (`Repo.Only`) |
| Commit | Commit & Share | 提交並分享 | `commit -m` | `git commit` + `git push` | One step. Teammates' versions are merged in first; a conflict stops it with nothing changed |
| Commit here only | (no team: Commit) | 提交 | `commit --local` | `git commit` | Shared by the next commit |
| Discard | Discard changes | 捨棄變更 | `goto HEAD --force` | `git restore`, `git reset --hard` | Asked first in the app |
| Get updates | Get updates | 取得更新 | `update` (`--preview`) | `git pull` | Never by itself. Your uncommitted changes stay, merged with theirs |
| What teammates shared | "Mia shared 1 new version" | 「Mia 上傳了 1 個新版本」 | `update --preview`, `status` (`team.incoming`) | `git fetch` + `git log ..origin` | |
| Combine | Combine and share | 合併並分享 | (`commit` does it) | pull, then push | Teammates committed on your branch meanwhile |
| Branch | Branch, New branch | 分支、新分支 | `branch`, `branch new` | branch | Names are any text; keeps its parent and the version it started at; its versions are its own for good |
| Switch | Switch to | 切換到 | `switch` | `git switch` | Changes don't block it: they are parked |
| Parked changes | Parked, Bring changes here | 暫放、把變更帶到這裡 | `parked`, `parked bring`, `parked discard` | `git stash` | One set per place, kept and brought back by themselves; on this computer only |
| Go to a version | Go to this version, Back to latest | 前往這個版本、回到最新版本 | `goto <version>`, `goto latest` | `git checkout <commit>` (detached HEAD) | Not detached: "on an older version" of your branch, with a way back |
| Keep an older version | Make this the latest version | 把這個版本設為最新 | — | `git checkout <old> -- .` + commit | A new version with the older one's files |
| Merge | Merge into current branch, Merge back to main | 合併到目前的分支、合併回 main | `merge <branch>`, `merge-sets` | `git merge --no-ff` | Always makes a version. Live sets merge per track; a conflict is a track or a file |
| Decisions | Keep yours / Keep {who}'s / Keep both | 保留你的／保留 {who} 的／兩者都保留 | `--strategy ours\|theirs\|both` | conflict markers, `-X ours/theirs` | No markers in files: you choose per track or file |
| Undo a commit | Undo this commit | 復原這次提交 | — | `git reset` + force push, or `git revert` | Taken back when no one has it yet, else a new version that undoes it |
| Restore a file | Restore… | 還原… | — | `git restore --source` | |
| Recover samples | Recover from R3V | 從 R3V 救回 | — | — | Samples a set uses that are missing here, from R3V's copy |
| Milestone | Milestone | 里程碑 | — | annotated tag | Named by the team, renamed or taken away any time |
| Export | Export… | 匯出… | `export` | `git archive`, a worktree | A separate project folder that opens on its own (samples copied) |
| Archive a branch | Archive branch… | 封存分支… | `branch archive`, `branch archived`, `branch unarchive` | `git branch -d` (reflog) | With the branches made from it; comes back whole |
| Delete a branch | Delete… (archived ones) | 刪除… | `branch delete` | `git branch -D` | Only an archived branch; its versions stay |
| File lock | Lock, Unlock | 鎖定、解除鎖定 | (R3V Cloud) | `git lfs lock` | Off unless the team turns it on; auto-lock by kind |
| Rules | Rules… | 規則… | `profile check`, `profile explain` | `.gitignore`, `.gitattributes` | Presets per tool (Live, Unity, Unreal, Godot…) |
| Backup | Backup | 備份 | `backup run`, `backup status`, `backup restore` | a mirror clone | The whole team, only adding |
| Health check | Check project… | 檢查專案… | `verify [--repair]` | `git fsck` | |
| Free space | — | — | `gc`, `storage-cleanup` | `git gc`, `git prune` | |
| Notices | (the app's notices) | | `watch` | — | Says what's new; never changes files |

## Words we don't use

- **push, pull, fetch, stage, stash, HEAD, detached, rebase, cherry-pick,
  tag, remote** in the app: say what happens instead (share, get updates,
  parked, the version you're on).
- **save** for making a version: in Live, saving is Ctrl+S. R3V commits.
  (`r3v save` and `r3v snapshot`, the CLI's names before 0.1.33, still
  work: they are `commit` and `commit --local`.)
- **delete** for a branch put away: it is archived.
- **upload** for sharing a version: uploading is the bytes going up (big
  files go up early, in the background).

## Settled

0.1.33: samples R3V brings back are **recovered** ("Recover from R3V";
"Restore" is for files from a version, and backups); "Your changes"
everywhere (not "pending changes"); "Versions you commit"; zh-TW says 分支
for branch and 復原 for Undo (撤回 for taking a version back).

0.1.34: the CLI goes to a version with `goto`, as the app's "Go to"
(`checkout`, its name before, still works).
