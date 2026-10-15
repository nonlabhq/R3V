# Team and project settings

Status: agreed (2026-10-15), being built. The App first; R3V Cloud's web
app follows with the same pages.

## What changes

Team Settings becomes a page with tabs down its left side:

| Tab | What it holds |
|---|---|
| **General** | Icon and name; Members |
| **Workflow** | File locking; Agents. Later: merging, reviews (docs/design/reviews.md) |
| **Activity** | What happened in the team, newest first |
| **Usage** | How much the team keeps, by project |

What's there today moves in without changing:
- your name in this team, inviting and your setup go to General;
- Connection, the move to R3V Cloud, storage cleanup and leaving go to General, under the members.

A project's Settings tab gets a **Workflow** section with the same settings
as the team's Workflow tab. Each one follows the team's unless the project
overrides it.

"Workflow" is the name because what it holds is how the team works together
(locking, agents, merging, reviews). "Rules" is already the project's
`.r3v.yaml`, and "Policy" sounds like an enterprise's.

## Inheriting and overriding

The rule, for every Workflow setting now and later:

    what applies = the project's override, else the team's setting, else R3V's default

In the project's settings, each one is a choice whose first option says what
the team has:

    File locking   [ As the team: on ▾ ]      ← follows the team
                     As the team: on
                     On for this project
                     Off for this project

When it's overridden, the row says so ("Overridden for this project"), with
**Use the team's** to go back.

In the team's Workflow tab, a setting some projects override says how many:
"2 projects set their own". A click lists them, each a link to its settings.

### Who can change them

- **R3V Cloud teams:** admins only, for the team's settings and the
  projects' overrides. The service refuses the write from anyone else, and
  the app shows the setting read-only, with who can change it.
- **Teams on their own storage:** anyone in the team. There are no roles:
  whoever has the connection code can write anything. That's said once, in
  the Workflow tab.

### Where they're kept

In the team's storage, beside what's there already, so everyone sees the same:
- the team's settings are in its info (`team.json`), as `locks` is today;
- a project's overrides are a new field of its record (`Project`, in
  `projects.json`): `"workflow": { "locks": {...}, "steps": {...} }`. Only
  what's overridden is there; a missing key means "as the team".

Both records keep the fields they don't know (jsonx.Extra), so an older R3V
rewriting one never drops them.

### Older R3Vs (AGENTS.md: team features)

An older R3V ignores a project's overrides. For each setting:
- **File locking.** Ignoring "on for this project" would share without
  locks. So a project turning locking on turns the team feature `locks` on,
  as the team's switch does today: R3Vs without it must update before
  sharing. "Off for this project" ignored is only stricter, so it's safe.
- **Agents' steps.** Steps are made on the agent's computer, by the R3V
  there. An older R3V would keep making steps, or keeping prompts, where the
  project says not to. That's a privacy setting the team chose, so the
  first override turns on a new team feature `workflow` (older R3Vs update
  before sharing).

## Workflow: what's in it

### File locking

R3V Cloud teams only: the service keeps who holds what. On a team's own
storage the section says it needs R3V Cloud.

- **The team's:** on or off for projects, and the kinds of file that lock
  by themselves (`team.json` `locks`: `on`, `kinds`, and `default`).
- **A project's own** (its record, `workflow.locks`): on or off, other
  kinds, manual locks only, and paths of its own that lock by themselves or
  never do ("Content/Maps/", "*.uasset"). Anything it doesn't set follows
  the team.
- **Team off, a project on:** the service has to keep locks for that
  project, so `on` stays true for the team's service and `default: false`
  says projects don't lock unless they say so. In a project that doesn't,
  nothing is ever locked, so the service's check never stops anyone.
  `default` is new: an R3V that knows locks but not it would lock in every
  project, so writing it turns the `workflow` feature on.
- **"Apply the team's to every project"** clears what projects set for
  locking (or for agents), in one go.
- **`.r3v.yaml`'s `file_locks:`** (where a project narrowed locks before)
  is no longer read: the settings are the team's, shared at once, changed
  by admins, never a file a member commits. A project whose file still has
  it shows a note, and an admin can move it into the project's settings,
  which also takes the section out of the file (a change to commit). Older
  R3Vs still read the file, and it only ever narrows, so that's safe.

### Agents

| Setting | Team default | Project | Person |
|---|---|---|---|
| Record agents' steps | on | can override | can say "none here" |
| Keep prompts with the steps | on | can override | can say "not mine" |
| Agents take turns on files that can't be merged | on | can override | — |

- **Record steps** off: agents' work still makes versions, but no steps
  between them (today's `StepSettings.Off`).
- **Keep prompts** off: steps without what was asked (today's `NoPrompts`).
  A person can also keep their own prompts out, in Preferences, whatever
  the project says. That choice is per person on this computer: it can only
  keep more out, never less.

- **Take turns** (agent locks, docs/design/agent-workspaces.md): between a
  project's workspaces on one computer, a file that can't be merged is
  changed by one at a time. Every team has it, a team's own storage too:
  it needs no service. Its kinds are the project's own, else the team's
  file-locking kinds, else every kind that can't be merged.

This computer's choices (`StepSettings` in the copy's config, as before)
stay, as the person's own: they only ever keep more out. A project of no
team keeps its settings on this computer.

The agents' hooks run on every tool call and can't ask the team: the
team's and the project's settings are kept beside the copy's agent lock
list (`workflow.json`), read again when an agent's session starts and
whenever the app shows or changes them.

## Activity

The team's recent events, newest first: mostly from what the storage keeps
already, plus a small team log for what it doesn't.

- **Shares and merges:** the branch log (`projects/<pid>/branchlog`): who
  moved which branch, when, from which version to which. A move to a version
  with two parents is a merge.
- **Agents:** a version made by an agent has its actor (tool, model), so its
  share reads "Claude Code (for Robin) shared 3 versions on agent/fix".
- **Branches made and deleted:** the branch log too.
- **Projects added and removed, members joining:** a new team log, kept
  the branch log's way: one record per event, never changed
  (`teamlog/<time>-<random>.json`: `{kind, project?, member?, by, time}`;
  kinds `project-added`, `project-removed`, `member-joined`). Written by
  this R3V after the step succeeded, as well as it can. An older R3V
  doesn't write them; that's data it never reads, so no team feature is
  needed. What happened before the log existed isn't shown.

Each line names the project and links to the version in its graph. Filters:
a project, each member, each agent (by tool), agents or people. The page reads the last 30 days, and
more on request; the branch log is read per project, so a team of many
projects reads them a few at a time.

## Usage

Both kinds of team:
- **What R3V keeps:** the size of the team's stored files, by project (each
  project's versions' files, counted once per content even if shared).
  Shown as a bar per project and a total. Counting walks the stored objects
  (as storage cleanup does), so it's done on request ("Count now") and the
  last count is kept, with its time.
- **R3V Cloud teams:** the plan's quota and what the service says is used,
  from the service. That is the number that counts for the plan; R3V's own
  count is shown beside it.
- **Teams on their own storage:** R3V's count only. The storage company's
  bill can be more (old versions kept by the bucket, other things in it),
  and the page says so.

## Members (in General)

- **R3V Cloud teams:** as today: people, roles, inviting, removing (admins).
- **Teams on their own storage:** the people who have shared versions, from
  the members list storage keeps (names and ids), with their looks. There's
  nothing to manage: the page says that anyone with the connection code is
  in.

## The pages

    ┌ Team settings ─────────────────────────────────────────────┐
    │ General      │  Icon and name                               │
    │ Workflow     │  [mark]  Demo Band   [Rename]                │
    │ Activity     │                                              │
    │ Usage        │  Members                                     │
    │              │  ● Robin (you) · admin                        │
    │              │  ● Mia · member                               │
    │              │  …                                           │
    │              │  Connection · Storage cleanup · Leave         │
    └──────────────┴──────────────────────────────────────────────┘

The tab picked is remembered. The project's Settings tab gets a Workflow
section after Rules, its rows as above.

## Steps (App first)

1. The team page with its four tabs: what's there today, moved into General
   and Workflow (file locking). Activity and Usage come with steps 3 and 4.
2. Overrides: the `Project.workflow` field, the inheriting rule and its
   row component, file locking per project (the `locks` feature when one
   turns it on). Agents' steps move to the team (the `workflow` feature), with
   the person's "not mine".
3. Activity from the branch log, and the team log.
4. Usage: R3V's count, and R3V Cloud's from the service (the Cloud lane
   adds the call).
5. R3V Cloud's web app: the same tabs with the shared components, once the
   App's are settled.

All behind Nightly first, with a Stable test that Stable shows today's
Team Settings.

## Open

- Which settings move into Workflow next: reviews (docs/design/reviews.md)
  and automatic merging for agents are the first candidates.
- Whether Activity should also show what happens live (presence) on R3V
  Cloud teams.
