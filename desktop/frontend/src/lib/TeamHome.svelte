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
  import SectionMenu from "./SectionMenu.svelte";
  import TeamActivity from "$shared/team/TeamActivity.svelte";
  import { appTeamSource } from "./teamSource";
  import { toast } from "./notify.svelte";
  import { errorText } from "./api";
  import { blockSpan, cardsShown, columnsFor, foldSection, groupBySection, orderedSections, sectionFolds, sectionShowAll,
    toggleShowAll } from "./sections.svelte";

  // A team's home (a click on the team in the sidebar): who's in it, what
  // happened lately, and its projects (those on this computer, then those
  // only on the cloud storage; a project's workspaces counted on it, not
  // listed) or its settings.
  let { overview, team, projects, open, onopen, onadd, onnewsection, onsettings, onsection, adding = false, reload, canMove = false, section = $bindable("projects") }: {
    overview: Overview;
    team: TeamSummary;
    projects: TeamProject[];
    open: (p: TeamProject) => boolean; // already in a tab
    onopen: (p: TeamProject) => void;
    onadd: () => void;
    onnewsection?: () => void; // New section… (none: the band's + adds a project)
    onsettings: (p: TeamProject) => void; // a project's settings (from its menu)
    onsection?: (p: TeamProject, section: string) => void; // Move to › in its menu (none: not offered)
    adding?: boolean;
    reload: () => Promise<void>;
    canMove?: boolean;
    section?: "projects" | "settings";
  } = $props();

  // The team, by what doesn't change from one read of the team's list to
  // the next (each gives a new team object): the members and the Activity
  // are asked again only for another team, not blinking out and back.
  let teamKey = $derived(team.id);
  let hosted = $derived(!!team.hosted);
  // The members, asked when the team changes (none while it can't be reached).
  let members = $state<Member[]>([]);
  $effect(() => {
    const id = teamKey;
    members = [];
    api.TeamMembers(id).then((ms) => { if (teamKey === id) members = ms ?? []; }).catch(() => {});
  });

  let activitySource = $derived(appTeamSource(teamKey, hosted));
  let kind = $derived(storageKind(team));
  let mains = $derived(projects.filter((p) => !p.workspace));
  // Archived for the team (docs/design/archive.md): listed apart, folded.
  let active = $derived(mains.filter((p) => !p.archived));
  let archived = $derived(mains.filter((p) => p.archived));
  // Sections (docs/design/sections.md): each section's projects, in this
  // person's order and folded as in the left panel, then the ungrouped ones
  // (here, then on the cloud storage).
  let sections = $derived(overview.sectionsOn ? orderedSections(team.id, overview.sections ?? []) : []);
  let grouped = $derived(groupBySection(active, sections, (p) => p.section ?? ""));
  let loose = $derived(grouped[grouped.length - 1].items);
  // How many columns of cards the blocks' grid has (its width, followed).
  let blocksEl = $state<HTMLElement>();
  let cols = $state(1);
  $effect(() => {
    const el = blocksEl;
    if (!el) return;
    const measure = () => { cols = columnsFor(el.clientWidth); };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  });
  let here = $derived(loose.filter((p) => p.status !== "remote"));
  let away = $derived(loose.filter((p) => p.status === "remote"));
  let archivedOpen = $state(false);
  let unarchiving = $state("");
  async function unarchive(p: TeamProject) {
    unarchiving = p.id;
    try {
      await api.UnarchiveProject(team.id, p.id);
      toast(t("“{name}” is back for the team", { name: p.name }), "ok");
      await reload();
    } catch (e) {
      toast(errorText(e), "error");
    } finally {
      unarchiving = "";
    }
  }
  const same = (a = "", b = "") => !!a && !!b && a.replace(/[\/]+$/, "").toLowerCase() === b.replace(/[\/]+$/, "").toLowerCase();
  const spaces = (p: TeamProject) => projects.filter((w) => w.workspace && same(w.mainRoot, p.root)).length;
  let summary = $derived([members.length ? tn(members.length, "{n} member", "{n} members") : "",
    tn(active.length, "{n} project", "{n} projects"), kind].filter(Boolean).join(" · "));
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
  // The Projects band's +: a project, or a choice of a project or a new
  // section (onnewsection: for those who may change the team's sections).
  let plusMenu = $state<{ at: { left: number; top: number }; opener: HTMLElement } | null>(null);
  function plus(e: MouseEvent) {
    if (!onnewsection) { onadd(); return; }
    const el = e.currentTarget as HTMLElement, r = el.getBoundingClientRect();
    plusMenu = plusMenu ? null : { opener: el, at: { left: Math.max(0, Math.min(r.right - 200, window.innerWidth - 220)), top: Math.min(r.bottom + 4, window.innerHeight - 120) } };
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
          <div class="pane-in"><TeamActivity source={activitySource} latest={5} onePerProject onopen={openVersion} /></div>
        </section>
      {/if}
      <section class="pane projects">
        <h2 class="band">{t("Projects")}<span class="end">{active.length}</span>
          <button class="ghost band-add" onclick={plus} disabled={adding} title={onnewsection ? t("Add a project or a section") : t("Add project")}
            aria-label={onnewsection ? t("Add a project or a section") : t("Add project")} aria-haspopup={onnewsection ? "menu" : undefined}
            aria-expanded={onnewsection ? !!plusMenu : undefined}>+</button></h2>
        <div class="pane-in">
          <!-- the sections, each a block as wide as its cards need, side by
               side while they fit, in order; then the projects in none -->
          <div class="blocks" style:--cols={cols} bind:this={blocksEl}>
            {#each grouped.slice(0, -1) as g (g.id)}
              {@const shut = !!sectionFolds[g.id]}
              {@const all = !!sectionShowAll[g.id]}
              {@const shown = cardsShown(g.items.length, cols, all)}
              <div class="block" data-section={g.id} style:--span={shut ? 1 : blockSpan(g.items.length, cols)}>
                <h3><button class="fold" aria-expanded={!shut} onclick={() => foldSection(g.id)}>
                  <span class="caret" aria-hidden="true">{shut ? "▸" : "▾"}</span>{g.name}<span class="count">{g.items.length}</span></button>
                  {#if !shut && g.items.length > 2 * cols}
                    <button class="fold more-cards" aria-expanded={all} onclick={() => toggleShowAll(g.id)}>
                      {all ? t("Show less") : t("Show all ({n})", { n: g.items.length })}</button>
                  {/if}</h3>
                {#if !shut}
                  {#if g.items.length}
                    <div class="cards" aria-label={g.name}>
                      {#each g.items.slice(0, shown) as p (p.root || p.id)}{@render card(p)}{/each}
                    </div>
                  {:else}
                    <p class="none">{t("No projects in this section yet")}</p>
                  {/if}
                {/if}
              </div>
            {/each}
            <div class="block loose" style:--span={blockSpan(here.length + 1, cols)}>
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
            </div>
          </div>
          {#if away.length}
            <h3>{t("On the cloud storage")}<span class="count">{away.length}</span>
              <span class="hint">{t("Open one to download it to this computer")}</span></h3>
            <div class="cards">
              {#each away as p (p.id)}{@render card(p)}{/each}
            </div>
          {/if}
          {#if archived.length}
            <!-- archived for the team (docs/design/archive.md): folded -->
            <h3><button class="fold" aria-expanded={archivedOpen} onclick={() => (archivedOpen = !archivedOpen)}>
              <span class="caret" aria-hidden="true">{archivedOpen ? "▾" : "▸"}</span>{t("Archived")}<span class="count">{archived.length}</span></button>
              <span class="hint">{t("Hidden for everyone in the team; nothing deleted")}</span></h3>
            {#if archivedOpen}
              <div class="cards archived">
                {#each archived as p (p.id)}
                  <div class="archived-card">
                    {@render card(p)}
                    {#if team.canArchive}
                      <button class="unarchive" onclick={() => unarchive(p)} disabled={unarchiving === p.id}>{t("Unarchive")}</button>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          {/if}
        </div>
      </section>
    </div>
    {#if menu}
      <ProjectMenu p={menu.p} at={menu.at} opener={menu.opener} {onsettings} sections={onsection ? sections : undefined} {onsection}
        onclose={() => (menu = null)} />
    {/if}
    {#if plusMenu && onnewsection}
      <SectionMenu at={plusMenu.at} opener={plusMenu.opener} {onadd} onnew={onnewsection} onclose={() => (plusMenu = null)} />
    {/if}
  {:else}
    <div class="body settings">
      {#key team.id}
        <TeamSettings {team} inline tabs workflowTab {reload} onclose={() => (section = "projects")}
          onopenversion={openVersion}
          teams={overview.teams} {canMove} offline={!!overview.teamError}
          roots={projects.filter((p) => p.root && p.status === "downloaded").map((p) => p.root)} />
      {/key}
    </div>
  {/if}
</div>

<style>
  /* The R3V look: a page on the window, the Activity and Projects cards on
     it, each under a plain heading. Flat: the head a panel, the rest panels
     under it, 2px apart, each under its band. */
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
  .body { flex: 1 0 auto; display: flex; flex-direction: column; gap: calc(var(--pane-gap) + var(--sp-16) * (1 - var(--bleed))); }
  .body.settings > :global(*) { flex: 1; }
  .pane { background: var(--surface-pane); border-radius: var(--pane-radius); box-shadow: var(--shadow-card); }
  .pane.projects { flex: 1 0 auto; }
  .pane-in { padding: var(--sp-12) var(--sp-16) var(--sp-16); }
  /* (Flat draws the band; this look a heading in the card) */
  h2 { display: flex; align-items: baseline; gap: var(--sp-8); margin: calc(var(--sp-16) * (1 - var(--bleed))) 0 0;
    padding: 0 var(--sp-16); font-size: var(--fs-md); font-weight: var(--fw-semibold); color: var(--muted); }
  h2 .end { margin-left: auto; font-size: var(--fs-sm); color: var(--faint); font-weight: normal; }
  .band-add { margin-right: calc(var(--sp-4) * -1); padding: 0 var(--sp-6); font-size: var(--fs-lg); line-height: 18px; color: var(--faint); }
  .band-add:hover:not(:disabled) { color: var(--text); }
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
  h3 .fold { display: inline-flex; align-items: baseline; gap: var(--sp-8); padding: 0; border: none; border-radius: 0; background: transparent;
    font: inherit; color: inherit; }
  h3 .fold:hover:not(:disabled) { background: transparent; color: var(--text); }
  .caret { font-size: var(--fs-xs); color: var(--faint); }
  /* One grid of card columns (--cols, as many as fit 240px); each block
     spans --span of them, its cards on the same columns. */
  .blocks { display: grid; grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); gap: var(--sp-10) var(--sp-12); align-items: start; }
  .block { grid-column: span var(--span); min-width: 0; }
  .block .cards { grid-template-columns: repeat(var(--span), minmax(0, 1fr)); }
  h3 .more-cards { margin-left: auto; font-weight: normal; color: var(--accent-text); }
  .none { margin: 0 0 var(--sp-10); font-size: var(--fs-sm); color: var(--faint); }
  .archived-card { position: relative; min-width: 0; }
  .archived-card .card { padding-right: 110px; }
  .archived-card .card .name { color: var(--muted); }
  .unarchive { position: absolute; right: var(--sp-12); top: 50%; transform: translateY(-50%); padding: var(--sp-4) var(--sp-10);
    font-size: var(--fs-md); }
  .plus { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; font-size: var(--fs-xl); color: var(--muted); }
</style>
