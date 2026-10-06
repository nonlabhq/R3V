<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type Overview, type TeamProject } from "./api";
  import { toast } from "./notify.svelte";

  // A new tab (Ctrl+T, or + by the tabs): pick a team, then a project to
  // open here, or add one.
  let { overview, projects, open, reload, onopen, onadd, adding = false }: {
    overview: Overview;
    projects: TeamProject[]; // the current team's
    open: (p: TeamProject) => boolean; // already in a tab
    reload: () => Promise<void>;
    onopen: (p: TeamProject) => void;
    onadd: () => void;
    adding?: boolean;
  } = $props();

  async function team(id: string) {
    if (id === overview.currentTeam) return;
    try {
      await api.SelectTeam(id);
      await reload();
    } catch (e) {
      toast(errorText(e), "error");
    }
  }
  const status = (p: TeamProject) => (({ remote: t("On the team, not on this computer yet"), missing: t("Folder not found") } as Record<string, string>)[p.status] ?? `⑂ ${p.branch}`);
  const icon: Record<string, string> = { remote: "☁", missing: "⚠", downloaded: "♪" };
</script>

<div class="new-tab">
  <h1>{t("Open a project")}</h1>
  {#if overview.teams.length > 1}
    <div class="teams" role="tablist" aria-label={t("Team")}>
      {#each overview.teams as tm (tm.id)}
        <button class="team" class:on={tm.id === overview.currentTeam} role="tab" aria-selected={tm.id === overview.currentTeam}
          onclick={() => team(tm.id)}>{tm.name}</button>
      {/each}
    </div>
  {:else if overview.teams.length === 1}
    <div class="teams"><span class="team on">{overview.teams[0].name}</span></div>
  {/if}

  <div class="cards">
    {#each projects as p (p.root || p.id)}
      <button class="card {p.status}" onclick={() => onopen(p)} title={p.root || p.name}>
        <span class="icon" aria-hidden="true">{icon[p.status]}</span>
        <span class="name">{p.name}</span>
        <span class="meta">{status(p)}</span>
        {#if open(p)}<span class="open-tag">{t("open")}</span>{/if}
      </button>
    {/each}
    <button class="card add" onclick={onadd} disabled={adding}>
      <span class="icon" aria-hidden="true">+</span>
      <span class="name">{t("Add project")}</span>
      <span class="meta">{t("Select project folder")}</span>
    </button>
  </div>
</div>

<style>
  .new-tab { height: 100%; overflow: auto; padding: var(--sp-32) var(--sp-40); }
  h1 { margin: 0 0 var(--sp-16); font-size: var(--fs-2xl); font-weight: var(--fw-semibold); }
  .teams { display: flex; flex-wrap: wrap; gap: var(--sp-6); margin-bottom: var(--sp-24); }
  .team { padding: var(--sp-4) var(--sp-14); border-radius: var(--radius-pill); font-size: var(--fs-md); color: var(--muted); }
  .team.on { color: var(--accent-ink); background: var(--accent); border-color: var(--accent); font-weight: var(--fw-semibold); }
  span.team { display: inline-block; border: var(--border-width) solid var(--accent); }
  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: var(--sp-12); }
  .card { position: relative; display: flex; flex-direction: column; align-items: flex-start; gap: var(--sp-4); min-height: 112px;
    padding: var(--sp-16); border-radius: var(--radius-lg); background: var(--panel); text-align: left; }
  .card:hover:not(:disabled) { background: var(--panel-2); border-color: var(--line-strong); }
  .icon { font-size: var(--fs-xl); color: var(--accent); margin-bottom: var(--sp-6); }
  .card.remote .icon { color: var(--faint); }
  .card.missing .icon { color: var(--warn); }
  .name { font-weight: var(--fw-semibold); max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { font-size: var(--fs-sm); color: var(--faint); max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .open-tag { position: absolute; top: var(--sp-10); right: var(--sp-10); font-size: var(--fs-2xs); padding: 0 var(--sp-6);
    border-radius: var(--radius-pill); background: var(--accent-bg); color: var(--accent); }
  .card.add { border-style: dashed; background: transparent; align-items: center; justify-content: center; text-align: center; }
  .card.add .icon { margin: 0; color: var(--muted); }
</style>
