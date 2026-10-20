# Getting started

Setting R3V up for yourself or a team, and using it every day (Windows).
The command line: [cli.md](cli.md). Coming from Git: [git-users.md](git-users.md).
The words: [glossary.md](glossary.md).

## 1. Install

Run `R3V-<version>-setup.exe` (from the [Releases](https://github.com/nonlabhq/R3V/releases)
page) on every computer. No administrator rights are needed; Windows 10
(21H2 or later) or Windows 11. The installer is not code-signed yet: if
Windows shows "Windows protected your PC", click **More info → Run
anyway**. R3V updates itself after that.

Leave **Start with Windows** checked: R3V then waits in the system tray and
tells you about new versions. It never changes your project files on its
own. The installer also puts the command line tool `r3v` on your PATH.

## 2. R3V Cloud or your own bucket

Every project lives in a team's storage, so its versions are safe if a
drive fails. On your own? Make a team of one. The team's storage is one
of two kinds, chosen when the team is made:

| | **R3V Cloud** | **Your own bucket** |
|---|---|---|
| Setting up | Sign in with your browser: nothing else | A bucket and a key on Cloudflare R2 or any S3-compatible storage (about 5 minutes) |
| Teammates join | With an invitation (by email), and sign in | With the team's connection code, which holds the storage key |
| Who may do what | Owners, admins, members; collaborators on chosen projects only | Everyone with the code can do everything |
| File locks | Yes (Team settings › Workflow) | No |
| Backups, storage cleanup | Not R3V's to do: the service looks after its storage | Yours: a backup to a drive or another bucket (below), storage cleanup |
| Cost | Closed beta: one team of your own, by invitation | What your storage charges (a small team usually stays in R2's free allowance) |

A team on its own bucket can move to R3V Cloud later, with its people and
history (Team settings › **Move to R3V Cloud**).

## 3. Join or create the team

The first time R3V opens, it walks you through three steps: **Team**,
**Your name** (shown next to the versions you commit) and **Projects**.
Later, the Team menu at the top of the sidebar joins or creates another
team (**Join/Create a Team…**). The Team step has three tabs: **Join a
team** (a connection code), **Create a team** (your own bucket) and
**R3V-Cloud**.

### On R3V Cloud

1. Choose **R3V-Cloud** › **Sign in with your browser**, and finish signing
   in there. Your account's teams come with it, on every computer you sign
   in on (the sign-in is kept in Windows' Credential Manager; `r3v login`
   on the command line).
2. **Create a team**: give it a name › **Create**. R3V Cloud is in a closed
   beta: making a team needs an invitation to the beta. Without one, R3V
   says so and offers **Join the waitlist**; you can still join teams
   you're invited to. The beta includes one team of your own.
3. **Join someone's team**: paste the invitation link you got under **Join
   with an invitation link** › **Join**. If the team moved to R3V Cloud
   from its own storage, R3V asks **Who were you?**: pick yourself, and
   your versions, name and look stay yours (or **I'm new to this team**).

To invite people: Team settings › **General** › **People** › **Invite
someone**: their email and a role (admin, member, or a collaborator who
works only on the projects you choose) › **Send invitation**.

### On your own bucket (one person sets it up)

The bucket can be on **any S3-compatible storage** that supports
conditional writes: Cloudflare R2, Amazon S3, MinIO and others. R3V checks
the storage before it creates the team. Cloudflare R2 is the easiest
start: the app guides you through it, downloads are free, and a small
team usually stays within its free allowance.

Choose **Create a team**, then **Cloudflare R2** or **Other
S3-compatible**.

**Cloudflare R2:**

1. **Open R2.** Log in to the [Cloudflare dashboard](https://dash.cloudflare.com/)
   (or sign up) and open **Storage & databases → R2 Object Storage**. The
   first time, Cloudflare asks you to activate R2; it may ask for a payment
   method even for the free allowance.
2. **Create a bucket.** **Create bucket** → a name such as `my-team-r3v` →
   keep Location *Automatic* and Storage Class *Standard* → **Create
   bucket**. Use a bucket just for R3V.
3. **Create a key for the bucket.** Back on the R2 page, under *Account
   Details*: **Manage API Tokens** → **Create Account API token**.
   Permissions: *Object Read & Write*. Specify bucket(s): *Apply to
   specific buckets only* → your bucket. Then create the token.
4. **Copy the S3 credentials.** The next page is shown only once. Under
   *Use the following credentials for S3 clients*, copy the **Access Key
   ID**, the **Secret Access Key** and the **Default** endpoint
   (`https://<account id>.r2.cloudflarestorage.com`). The *Token value* at
   the top of that page is not needed.
5. **Name your team** and click **Check & create team**. R3V reads, writes
   and removes a test file to make sure everything works, then shows the
   team's **connection code**.

**Other S3-compatible storage:** create a bucket for R3V and a key that may
read, write and delete objects in it, then enter the bucket, the keys, the
endpoint (e.g. `https://s3.eu-central-1.amazonaws.com` for Amazon S3) and
the region; the folder inside the bucket defaults to `r3v`. Without
conditional writes (`If-None-Match`), which keep two people from
overwriting each other's versions, R3V says so and does not create the
team.

**Send the connection code** to each teammate **privately** (a direct
message, not a public channel): it contains the key, and anyone who has it
can read, change and delete the team's work. Teammates choose **Join a
team** and paste it. If a code leaks, make a new key (in Cloudflare: a new
API token, then delete the old one), enter it in Team settings ›
**Connection**, and send teammates the new code. Keys are kept once per
computer (`%APPDATA%\R3V\teams.json`), never inside project folders, so a
project folder can be copied without leaking them.

### Team settings

The team's home (click its name at the top of the sidebar) › **Team
settings**, in four tabs:

- **General** — the team's name and icon (renamed for everyone, or only on
  this computer); **Your name in this team** (changing it changes it on
  all your versions); members or **Invite teammates**; **Connection** (own
  bucket: change the bucket or key; R3V checks them first); **Backup**;
  **Move to R3V Cloud**; **Storage cleanup**; leaving or removing the team
  on this computer (its projects keep their versions, set aside; joining
  again offers to reconnect them).
- **Workflow** — file locking (R3V Cloud) and the agents' settings: the
  team's defaults, which a project can set its own over.
- **Activity** — what happened in the team, newest first.
- **Usage** — how much the team keeps, by project.

## 4. Add projects

**Share one you have:** in the sidebar, **+ Add project** and choose the
project folder (for Live, the one with the `.als` file and `Ableton Project
Info`; for Unity, Unreal or Godot, the project's folder). R3V tells what
kind of project it is, which says what is left out (caches, builds) and how
files merge ([profiles.md](profiles.md)). It asks whether to commit and
share a first version now, or later, after you've looked through the files
and left out what you don't need (right-click › **Ignore**). Nothing is
uploaded until you say so.

**Get one from the team:** the sidebar lists every project of the team.
Those not on this computer yet are dimmed (☁): pick one and click
**Download**. Samples that lived outside the project on the other computer
are downloaded too, and the set is pointed at them. If you move a
downloaded project folder, R3V marks it with ⚠: **Locate folder…** points
it at the new place.

The **⋯** menu next to a project: **Open in** its program, **Open folder**,
**Move to** a section, **Settings…**. The project's settings (also its
**Settings** tab):

- **General** — its name (for everyone; the folder keeps its name), its
  section, **Check project…** (read its whole history again, looking for
  damage) and, under Danger zone, **Unlink folder** (R3V stops listing the
  folder; nothing is deleted), **Archive project…** (hidden for everyone,
  nothing deleted) and **Delete…** (for everyone; copies on computers are
  kept).
- **Branches** — archived branches, to bring back or delete.
- **Rules** — which files are versioned (`.r3v.yaml`).
- **Workflow** — the team's workflow settings, or the project's own.

## 5. Everyday work

- Work in your program as usual and save (in Live, **Ctrl+S**). Your
  changes appear under **Changes** (a Live Set track by track).
- When you reach a point worth sharing, describe it and click **Commit &
  Share**. If a teammate committed in the meantime, R3V shows what they
  changed and lets you choose: combine your work with theirs (**Combine and
  share**; where you both changed the same track or lines, it asks which to
  keep: yours, theirs, or both), put your work on a new branch, or discard
  it.
- When a teammate shares, R3V shows it. **Preview** shows what changed,
  **Get updates** takes it. Close the set in Live first (the project in
  Unity, Unreal or Godot): R3V changes the files on disk and the program
  would overwrite them.
- To stay out of each other's way, work on your own branch (**⑂ → New
  branch from here…**) and merge it when it's ready (**Merge** on its
  latest version), or use your own `.als` in the same project. Editing the
  same set on the same branch works too: R3V combines your changes track by
  track and only asks when you both changed the same track.
- Going to another branch or an older version (**Go to this version**)
  with changes not committed parks them; they come back when you return.

**Disk space.** A team project's history lives in the team's storage. On
your computer, the project's `.r3v` folder keeps its Live Sets (small) but
not copies of files the team's storage already has: the project folder has
the current ones, and older ones are downloaded when you go to an older
version, listen to one, or restore a file.

## 6. Back up the team

The team's storage holds everything, but on a team's own bucket one member
should keep a second copy on a drive or NAS, in case the bucket or its keys
are ever lost. Creating a team ends with this suggestion; later, it's in
Team settings › **Backup** › **Choose a backup folder…** (an empty folder,
or the team's earlier backup) or **Another bucket…**: any S3-compatible
storage other than the team's own (another Cloudflare account, Backblaze
B2, Wasabi, a NAS running MinIO), with keys of its own, so a lost or leaked
team key can't reach the backup too.

- R3V copies what's new once a day while it is open (the first time:
  everything, in the background). It never deletes anything from the
  backup.
- If the drive isn't connected, it tries again later; after a few days
  without a backup, the sidebar says so.
- Everyone sees who backs up the team. While nobody has in the last week,
  the sidebar suggests it (✕ puts it off for a week).
- From the command line (a scheduled task on a NAS, say): `r3v backup run
  <folder>`.

**Restoring:** Team settings › **Backup** › **Restore…** compares the
backup with the team's storage and lists what would come back: deleted
projects (with their versions), branches, lost files. It only adds what
the storage lacks, so nothing a teammate did since is undone. **As of**
picks an earlier day's backup. If the bucket itself is gone (deleted, or
its keys lost): create a new bucket and a new team on it, then
**Restore…** from the backup into it, and send teammates the new code.

## 7. AI agents on your project

Several AI agents (Claude Code, Codex, Cursor or any other tool) can work
on a project at once, each in a **workspace** of its own: another copy of
the project on this computer, on a branch of its own. How it works:
[design/agent-workspaces.md](design/agent-workspaces.md); for the agents
themselves: [agents.md](agents.md) (`r3v help agents`).

**Once per project:**

1. **A team**: the project must be in one (an agent's work goes on a
   branch, and branches are the team's), and have a first version.
2. **The guide for agents**: the project's **Workspaces** tab › **Add the
   guide for agents** (for a Live project also Settings › Workflow › **Set
   up AI agents…**). It writes `AGENTS.md` (what every agent should know:
   commit, conflicts, working beside others) and a line in `CLAUDE.md` that
   imports it. Commit them: every workspace then has them.

**For each agent:**

1. **Workspaces** › **New workspace**: a branch name (what the agent works
   on: "renderer", "sounds"), a folder (next to the project by default),
   **For an AI agent (Claude Code)** ticked. R3V makes the copy from the
   files on this computer. For another tool, leave it unticked and run
   `r3v agent setup --tool NAME` in the workspace's folder.
2. **Start the agent in that folder** (`claude` in a terminal there). What
   the rules leave out isn't copied: install or build it there first (`npm
   install`, a first `cargo build`). On a Dev Drive (or ReFS) a workspace
   takes next to no extra space until files change.
3. **Let it work.** It commits on its branch (`r3v commit`), and its
   versions show which agent made them. Code merges line by line; files
   that can't be merged (textures, models, sounds) are locked by the first
   agent to change them: another agent is refused them, and so are the
   ones you've changed and not committed. Your own changes are never
   refused, and you can break an agent's lock. Its steps (the files after
   each of its moves) let you go back to any point of its work.

**When it's done:** the Workspaces tab says what each workspace has to
merge. **Merge into {branch}**: the usual merge preview; where you both
changed the same lines, you choose, one place at a time. Merged, its branch
is offered for archiving (its versions stay), and **Remove…** removes the
workspace (refused while it has work not merged, unless you say so).

Locks between agents stay on this computer; with teammates on R3V Cloud,
the team's file locks work as before.

## Uninstall

Settings → Apps → R3V → Uninstall. Your projects and their `.r3v` history
folders are kept, and so is the team's storage. Tick **Remove my settings**
to also forget your teams, storage keys and name on this computer (before
handing the computer on, or to try a fresh install).
