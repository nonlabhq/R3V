# Agents on your project: a quick start

For people who want AI agents (Claude Code, Codex, Cursor or any other
tool) to work on a project versioned with R3V, several at once.
How it works: [design/agent-workspaces.md](design/agent-workspaces.md);
the words: [glossary.md](glossary.md).

## Once per project

1. **An up-to-date R3V** (Settings › Updates).
2. **A team**: the project must be in one (an agent's work goes on a
   branch, and branches are the team's). Your own bucket
   ([team-setup.md](team-setup.md)), or R3V Cloud.
3. **Add the project** to R3V, and commit a first version.
4. **The guide for agents**: in the project's **Workspaces** tab, *Add the
   guide for agents* (for a Live project also in its Settings ›
   Workflow › **Set up AI agents…**). It writes `AGENTS.md` (what every
   agent should know: commit, conflicts, working beside others) and a line
   in `CLAUDE.md` that imports it. Commit them: every workspace then has
   them.

## For each agent

1. **Workspaces** tab › **New workspace**: a branch name (what the agent
   works on: "renderer", "sounds"), a folder (next to the project by
   default), *For an AI agent (Claude Code)* ticked. R3V makes a copy of the
   project there, from the files on this computer, on a branch of its own.
   Another tool: untick it and run `r3v agent setup --tool NAME` in the
   workspace's folder.
2. **Start the agent in that folder** (`claude` in a terminal there). What
   the rules leave out isn't copied: install or build it there first
   (`npm install`, a first `cargo build`). Projects on a Dev Drive (or
   ReFS) take next to no extra space per workspace.
3. **Let it work.** It commits on its branch (`r3v commit`), and its
   versions show who made them. Code merges line by line; files that
   can't be merged (textures, models, sounds) are locked by the first
   agent to change them: another agent is refused them, and so are the
   ones you've changed and not committed. You always can: your changes
   are never refused, and you can break an agent's lock.

## When an agent is done

1. **Workspaces** tab: its workspace says what it has to merge. **Merge into
   main** (or the branch it came from): the usual merge preview; where
   you both changed the same lines of code, you choose, one place at a
   time.
2. Merged, its branch is offered for archiving (its versions stay), and
   the workspace can be removed (**Remove…**; R3V refuses while it has
   work not merged, unless you say so).

## Good to know

- Agents can also do all this from the command line: `r3v help agents`.
- A workspace is a full copy: big projects take space per workspace (a
  Dev Drive helps), and each builds its own `node_modules/`, `target/`.
- Locks between agents stay on this computer; with teammates on R3V
  Cloud, the team's file locks work as before.
