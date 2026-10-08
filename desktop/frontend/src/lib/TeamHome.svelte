<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { api, type Member, type Overview, type TeamProject, type TeamSummary } from "./api";
  import ProjectIcon from "./ProjectIcon.svelte";
  import Avatar from "./Avatar.svelte";
  import TeamSettings from "./TeamSettings.svelte";
  import { cssColor, initial, pickFor } from "./palette";
  import { storageKind } from "./teamText";

  // A team's home (a click on the team in the sidebar): who's in it, and its
  // projects or its settings. Not a tab: the tabs show no tab picked.
  let { overview, team, projects, open, onopen, onadd, adding = false, reload, canMove = false, section = $bindable("projects") }: {
    overview: Overview;
    team: TeamSummary;
    projects: TeamProject[];
    open: (p: TeamProject) => boolean; // already in a tab
    onopen: (p: TeamProject) => void;
    onadd: () => void;
    adding?: boolean;
    reload: () => Promise<void>;
    canMove?: boolean;
    section?: "projects" | "settings";
  } = $props();

  // The members, asked when the team changes (none while it can't be reached).
  let members = $state<Member[]>([]);
  $effect(() => {
    const id = team.id;
    members = [];
    api.TeamMembers(id).then((ms) => { if (team.id === id) members = ms ?? []; }).catch(() => {});
  });

  let kind = $derived(storageKind(team));
  let summary = $derived([members.length ? tn(members.length, "{n} member", "{n} members") : "",
    tn(projects.length, "{n} project", "{n} projects"), kind].filter(Boolean).join(" · "));
  const status = (p: TeamProject) => (({ remote: t("On the team, not on this computer yet"), missing: t("Folder not found") } as Record<string, string>)[p.status]
    ?? `⑂ ${p.branchLabel || p.branch}`);
</script>

<div class="home">
  <header>
    <span class="mark" style:--c={cssColor(pickFor(team.id))} aria-hidden="true">{initial(team.name)}</span>
    <div class="title">
      <h1>{team.name}</h1>
      <p class="sub">{summary}</p>
    </div>
  </header>

  {#if members.length}
    <ul class="members" aria-label={t("Members")}>
      {#each members as m (m.id)}
        <li class:me={m.id === team.memberId}>
          <Avatar name={m.name} seed={m.id} color={team.looks ? m.color : ""} size={26} />
          <span class="mname">{m.name}</span>
          {#if m.id === team.memberId}<span class="you">{t("you")}</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}

  <div class="switch" role="group" aria-label={team.name}>
    <button class:on={section === "projects"} aria-pressed={section === "projects"} onclick={() => (section = "projects")}>{t("Projects")}</button>
    <button class:on={section === "settings"} aria-pressed={section === "settings"} onclick={() => (section = "settings")}>{t("Team settings")}</button>
  </div>

  {#if section === "projects"}
    <div class="cards">
      {#each projects as p (p.root || p.id)}
        <button class="card {p.status}" onclick={() => onopen(p)} title={p.root || p.name}>
          <ProjectIcon {p} size={36} />
          <span class="text">
            <span class="name">{p.name}</span>
            <span class="meta">{status(p)}</span>
          </span>
          {#if open(p)}<span class="tag">{t("open")}</span>{:else if p.status === "remote"}<span class="cloud" aria-hidden="true">☁</span>{/if}
        </button>
      {/each}
      <button class="card add" onclick={onadd} disabled={adding}>
        <span class="plus" aria-hidden="true">+</span>
        <span class="text">
          <span class="name">{t("Add project")}</span>
          <span class="meta">{t("Select project folder")}</span>
        </span>
      </button>
    </div>
  {:else}
    {#key team.id}
      <TeamSettings {team} inline author={overview.author} {reload} onclose={() => (section = "projects")}
        teams={overview.teams} {canMove} offline={!!overview.teamError}
        roots={projects.filter((p) => p.root && p.status === "downloaded").map((p) => p.root)} />
    {/key}
  {/if}
</div>

<style>
  .home { height: 100%; overflow: auto; padding: var(--sp-32) var(--sp-40) var(--sp-40); }
  header { display: flex; align-items: center; gap: var(--sp-16); }
  .mark { flex: none; width: 52px; height: 52px; border-radius: var(--radius-lg); display: flex; align-items: center; justify-content: center;
    font-size: var(--fs-xl); font-weight: var(--fw-bold); color: var(--c); background: color-mix(in srgb, var(--c) 20%, var(--panel)); }
  .title { min-width: 0; }
  h1 { margin: 0; font-size: var(--fs-2xl); font-weight: var(--fw-bold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { margin: var(--sp-2) 0 0; font-size: var(--fs-md); color: var(--muted); }
  .members { list-style: none; margin: var(--sp-20) 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: var(--sp-8); }
  .members li { display: flex; align-items: center; gap: var(--sp-8); padding: var(--sp-4) var(--sp-12) var(--sp-4) var(--sp-4);
    border-radius: var(--radius-pill); background: var(--panel); font-size: var(--fs-md); }
  .mname { max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .you { font-size: var(--fs-xs); color: var(--faint); }
  .switch { display: flex; gap: var(--sp-4); margin: var(--sp-28) 0 var(--sp-16); border-bottom: var(--border-width) solid var(--line); }
  .switch button { border: none; border-radius: 0; background: transparent; padding: var(--sp-8) var(--sp-4); margin-right: var(--sp-16);
    font-size: var(--fs-base); color: var(--muted); border-bottom: 2px solid transparent; margin-bottom: calc(var(--border-width) * -1); }
  .switch button:hover:not(:disabled) { background: transparent; color: var(--text); }
  .switch button.on { color: var(--text); font-weight: var(--fw-semibold); border-bottom-color: var(--accent); }
  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--sp-12); }
  .card { position: relative; display: flex; align-items: center; gap: var(--sp-12); min-height: 72px; padding: var(--sp-14) var(--sp-16);
    border-radius: var(--radius-lg); background: var(--panel); text-align: left; }
  .card:hover:not(:disabled) { background: var(--panel-2); border-color: var(--line-strong); }
  .card.remote .name { color: var(--muted); }
  .card.missing .meta { color: var(--warn); }
  .text { min-width: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { font-size: var(--fs-sm); color: var(--faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tag { position: absolute; top: var(--sp-8); right: var(--sp-10); font-size: var(--fs-2xs); padding: 0 var(--sp-6);
    border-radius: var(--radius-pill); background: var(--accent-bg); color: var(--accent); }
  .cloud { position: absolute; top: var(--sp-8); right: var(--sp-10); font-size: var(--fs-sm); color: var(--faint); }
  .card.add { border-style: dashed; background: transparent; }
  .plus { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; font-size: var(--fs-xl); color: var(--muted); }
</style>
