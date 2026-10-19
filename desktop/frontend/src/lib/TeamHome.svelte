<script lang="ts">
  import TeamMark from "$shared/TeamMark.svelte";
  import { t, tn } from "./i18n.svelte";
  import { api, type Member, type Overview, type TeamProject, type TeamSummary } from "./api";
  import ProjectIcon from "$shared/ProjectIcon.svelte";
  import Avatar from "$shared/Avatar.svelte";
  import TeamSettings from "./TeamSettings.svelte";
  import { storageKind } from "./teamText";
  import { linkPick } from "./links.svelte";
  import ProjectMenu from "./ProjectMenu.svelte";
  import TeamActivity from "$shared/team/TeamActivity.svelte";
  import { appTeamSource } from "./teamSource";

  // A team's home (a click on the team in the sidebar): who's in it, what
  // happened lately, and its projects (those on this computer, then those
  // only on the cloud storage; a project's workspaces counted on it, not
  // listed) or its settings.
  let { overview, team, projects, open, onopen, onadd, onsettings, adding = false, reload, canMove = false, agents = false, section = $bindable("projects") }: {
    overview: Overview;
    team: TeamSummary;
    projects: TeamProject[];
    open: (p: TeamProject) => boolean; // already in a tab
    onopen: (p: TeamProject) => void;
    onadd: () => void;
    onsettings: (p: TeamProject) => void; // a project's settings (from its menu)
    adding?: boolean;
    reload: () => Promise<void>;
    canMove?: boolean;
    agents?: boolean; // this build has agents (Nightly: their Workflow settings)
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
  let mains = $derived(projects.filter((p) => !p.workspace));
  let here = $derived(mains.filter((p) => p.status !== "remote"));
  let away = $derived(mains.filter((p) => p.status === "remote"));
  const same = (a = "", b = "") => !!a && !!b && a.replace(/[\/]+$/, "").toLowerCase() === b.replace(/[\/]+$/, "").toLowerCase();
  const spaces = (p: TeamProject) => projects.filter((w) => w.workspace && same(w.mainRoot, p.root)).length;
  let summary = $derived([members.length ? tn(members.length, "{n} member", "{n} members") : "",
    tn(mains.length, "{n} project", "{n} projects"), kind].filter(Boolean).join(" · "));
  // A version from the activity, in its project's graph.
  function openVersion(id: string, version: string) {
    const p = mains.find((x) => x.id === id);
    if (p) { linkPick.project = id; linkPick.version = version; onopen(p); }
  }
  const status = (p: TeamProject) => (({ remote: t("On the cloud storage, not on this computer yet"), missing: t("Folder not found") } as Record<string, string>)[p.status]
    ?? `⑂ ${p.branchLabel || p.branch}`);

  // A card's menu (its ⋯, or a right click): the same as the project's row
  // in the sidebar. opener: the ⋯ (Esc gives it the focus back).
  let menu = $state<{ p: TeamProject; at: { left: number; top: number }; opener: HTMLElement | null } | null>(null);
  const keyOf = (p: TeamProject) => p.root || p.id;
  function moreMenu(p: TeamProject, e: MouseEvent) {
    const el = e.currentTarget as HTMLElement;
    if (menu && keyOf(menu.p) === keyOf(p)) { menu = null; return; }
    const r = el.getBoundingClientRect();
    menu = { p, opener: el, at: { left: Math.max(0, Math.min(r.right - 220, window.innerWidth - 240)), top: Math.min(r.bottom + 4, window.innerHeight - 160) } };
  }
  function rightMenu(p: TeamProject, e: MouseEvent) {
    e.preventDefault();
    menu = { p, opener: null, at: { left: Math.min(e.clientX, window.innerWidth - 240), top: Math.min(e.clientY, window.innerHeight - 160) } };
  }
</script>

<div class="home">
  <!-- the team: who's in it, inviting someone, adding a project; then its
       projects or its settings -->
  <div class="head">
    <header>
      <TeamMark {team} size={44} />
      <div class="title">
        <h1>{team.name}</h1>
        <p class="sub">{summary}</p>
      </div>
      <span class="spacer"></span>
      {#if members.length}
        <ul class="members" aria-label={t("Members")}>
          {#each members.slice(0, 8) as m (m.id)}
            <li title={m.id === team.memberId ? `${m.name} (${t("you")})` : m.name}>
              <Avatar name={m.name} seed={m.id} color={team.looks ? m.color : ""} size={26} />
            </li>
          {/each}
          {#if members.length > 8}<li class="more">+{members.length - 8}</li>{/if}
        </ul>
      {/if}
      <button onclick={() => (section = "settings")} title={t("Teammates join with the team's invitation, in its settings")}>{t("Invite")}</button>
      <button class="primary" onclick={onadd} disabled={adding}>+ {t("Add project")}</button>
    </header>

    <div class="switch" role="group" aria-label={team.name}>
      <button class:on={section === "projects"} aria-pressed={section === "projects"} onclick={() => (section = "projects")}>{t("Projects")}</button>
      <button class:on={section === "settings"} aria-pressed={section === "settings"} onclick={() => (section = "settings")}>{t("Team settings")}</button>
    </div>
  </div>

  {#if section === "projects"}
    {#snippet card(p: TeamProject)}
      {@const n = spaces(p)}
      {@const on = !!menu && keyOf(menu.p) === keyOf(p)}
      <div class="slot" class:on oncontextmenu={(e) => rightMenu(p, e)} role="none">
      <button class="card {p.status}" onclick={() => onopen(p)} title={p.root || p.name}>
        <ProjectIcon {p} size={36} />
        <span class="text">
          <span class="name">{p.name}</span>
          <span class="meta">{status(p)}{n ? ` · ${tn(n, "{n} workspace", "{n} workspaces")}` : ""}</span>
        </span>
        {#if open(p)}<span class="tag">{t("open")}</span>{:else if p.status === "remote"}<span class="cloud" aria-hidden="true">☁</span>{/if}
      </button>
      <button class="ghost more" title={t("More")} aria-label={t("More")} aria-haspopup="menu" aria-expanded={on}
        onclick={(e) => moreMenu(p, e)}>⋯</button>
      </div>
    {/snippet}
    <div class="body">
      {#if !overview.teamError}
        <section class="pane activity">
          <h2 class="band">{t("Team activity")}</h2>
          <div class="pane-in"><TeamActivity source={appTeamSource(team.id, !!team.hosted)} latest={5} onopen={openVersion} /></div>
        </section>
      {/if}
      <section class="pane projects">
        <h2 class="band flat-only">{t("Projects")}<span class="end">{mains.length}</span></h2>
        <div class="pane-in">
          <h3>{t("On this computer")}<span class="count">{here.length}</span></h3>
          <div class="cards">
            {#each here as p (p.root || p.id)}{@render card(p)}{/each}
            <button class="card add" onclick={onadd} disabled={adding}>
              <span class="plus" aria-hidden="true">+</span>
              <span class="text">
                <span class="name">{t("Add project")}</span>
                <span class="meta">{t("Select project folder")}</span>
              </span>
            </button>
          </div>
          {#if away.length}
            <h3>{t("On the cloud storage")}<span class="count">{away.length}</span>
              <span class="hint">{t("Open one to download it to this computer")}</span></h3>
            <div class="cards">
              {#each away as p (p.id)}{@render card(p)}{/each}
            </div>
          {/if}
        </div>
      </section>
    </div>
    {#if menu}
      <ProjectMenu p={menu.p} at={menu.at} opener={menu.opener} {onsettings} onclose={() => (menu = null)} />
    {/if}
  {:else}
    <div class="body settings">
      {#key team.id}
        <TeamSettings {team} inline tabs workflowTab={agents || !!team.hosted} {reload} onclose={() => (section = "projects")}
          onopenversion={openVersion}
          teams={overview.teams} {canMove} offline={!!overview.teamError}
          roots={projects.filter((p) => p.root && p.status === "downloaded").map((p) => p.root)} />
      {/key}
    </div>
  {/if}
</div>

<style>
  /* The R3V look: a page on the window. Flat: the head a panel, the rest
     panels under it, 2px apart. */
  .home { height: 100%; overflow: auto; display: flex; flex-direction: column; gap: var(--pane-gap); background: var(--surface-gutter);
    padding: calc(var(--sp-32) * (1 - var(--bleed))) calc(var(--sp-40) * (1 - var(--bleed))) calc(var(--sp-40) * (1 - var(--bleed))); }
  .head { flex: none; background: var(--surface-head); padding: calc(var(--sp-16) * var(--bleed)) calc(var(--sp-16) * var(--bleed)) 0; }
  header { display: flex; align-items: center; gap: var(--sp-12); flex-wrap: wrap; }
  .title { min-width: 0; }
  h1 { margin: 0; font-size: var(--fs-2xl); font-weight: var(--fw-bold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { margin: var(--sp-2) 0 0; font-size: var(--fs-md); color: var(--muted); }
  .spacer { flex: 1; }
  .members { list-style: none; margin: 0 var(--sp-4) 0 0; padding: 0; display: flex; }
  .members li { display: flex; margin-left: -6px; border-radius: 50%; box-shadow: 0 0 0 2px var(--surface-gutter); }
  .members li.more { align-items: center; justify-content: center; width: 26px; height: 26px; font-size: var(--fs-2xs);
    background: var(--panel-2); color: var(--muted); }
  .switch { display: flex; gap: var(--sp-4); margin: var(--sp-20) 0 calc(var(--sp-16) * (1 - var(--bleed))); border-bottom: var(--border-width) solid var(--pane-edge); }
  .switch button { border: none; border-radius: 0; background: transparent; padding: var(--sp-8) var(--sp-4); margin-right: var(--sp-16);
    font-size: var(--tab-fs); text-transform: var(--tab-case); letter-spacing: var(--tab-spacing);
    color: var(--muted); border-bottom: 2px solid transparent; margin-bottom: calc(var(--border-width) * -1); }
  .switch button:hover:not(:disabled) { background: transparent; color: var(--text); }
  .switch button.on { color: var(--text); font-weight: var(--fw-semibold); border-bottom-color: var(--accent); }
  .body { flex: 1 0 auto; display: flex; flex-direction: column; gap: var(--pane-gap); }
  .body.settings > :global(*) { flex: 1; }
  .pane { background: var(--surface-head); }
  .pane.projects { flex: 1 0 auto; }
  .pane-in { padding: calc(var(--sp-12) * var(--bleed)) calc(var(--sp-16) * var(--bleed)) calc(var(--sp-16) * var(--bleed)); }
  h2 { display: flex; align-items: baseline; gap: var(--sp-8); margin: var(--sp-20) 0 var(--sp-10); font-size: var(--fs-md);
    font-weight: var(--fw-semibold); color: var(--muted); }
  .activity h2 { margin-top: var(--sp-4); }
  h2 .end { font-size: var(--fs-sm); color: var(--faint); font-weight: normal; }
  h3 { display: flex; align-items: baseline; gap: var(--sp-8); margin: var(--sp-10) 0 var(--sp-10); font-size: var(--fs-sm);
    font-weight: var(--fw-semibold); color: var(--muted); }
  h3 .count { font-size: var(--fs-sm); color: var(--faint); font-variant-numeric: tabular-nums; }
  h3 .hint { font-size: var(--fs-sm); font-weight: normal; color: var(--faint); }
  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--sp-12); }
  .card { position: relative; display: flex; align-items: center; gap: var(--sp-12); min-height: 72px; padding: var(--sp-14) var(--sp-16);
    border-radius: var(--radius-lg); background: var(--surface-tile); text-align: left; }
  .card:hover:not(:disabled) { background: var(--panel-2); border-color: var(--line-strong); }
  .card.remote { opacity: .75; }
  .card.remote .name { color: var(--muted); }
  .card.missing .meta { color: var(--warn); }
  .text { min-width: 0; display: flex; flex-direction: column; gap: var(--sp-2); }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { font-size: var(--fs-sm); color: var(--faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tag { position: absolute; top: var(--sp-8); right: var(--sp-10); font-size: var(--fs-2xs); padding: 0 var(--sp-6);
    border-radius: var(--radius-pill); background: var(--accent-bg); color: var(--accent); }
  .cloud { position: absolute; top: var(--sp-8); right: var(--sp-10); font-size: var(--fs-sm); color: var(--faint); }
  /* ⋯ at the top right, on hover or focus (a button beside the card's, not
     in it), over the "open" tag or cloud while it shows */
  .slot { position: relative; display: flex; min-width: 0; }
  .slot > .card { flex: 1; min-width: 0; }
  .more { position: absolute; top: var(--sp-4); right: var(--sp-4); padding: 0 var(--sp-6); line-height: 1.4; color: var(--muted); opacity: 0; }
  .slot:hover .more, .slot.on .more, .more:focus-visible { opacity: 1; }
  .slot:hover .card :is(.tag, .cloud), .slot.on .card :is(.tag, .cloud), .slot:has(.more:focus-visible) .card :is(.tag, .cloud) { visibility: hidden; }
  .card.add { border-style: dashed; background: transparent; }
  .plus { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; font-size: var(--fs-xl); color: var(--muted); }
</style>
