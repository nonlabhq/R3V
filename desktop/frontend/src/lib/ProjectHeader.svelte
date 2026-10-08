<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type State, type TeamProject, type TeamSummary } from "./api";
  import ProjectIcon from "./ProjectIcon.svelte";
  import { toast } from "./notify.svelte";
  import PreuploadIcon from "./PreuploadIcon.svelte";
  import { preuploads } from "./preupload.svelte";
  import { spinner } from "./spin.svelte";

  // The top of a project's page: its team (and its settings), its icon and
  // name, and opening it in its tool. (Branches are above the Overview's
  // graph; settings are a tab.)
  let { st, refreshing, team, entry, onteamsettings, oncheck, onrefresh }: {
    st: State;
    refreshing: boolean;
    team?: TeamSummary; // none: on this computer only
    entry?: Pick<TeamProject, "id" | "icon" | "color">; // the team's look for it
    onteamsettings?: (team: TeamSummary) => void;
    oncheck: () => void;
    onrefresh: () => void;
  } = $props();

  let setMenu = $state(false);
  const turning = spinner(() => refreshing);
  let isLive = $derived(st.tool === "Ableton Live");
  const label = (rel: string) => (rel === "." ? st.name ?? t("the project") : rel);
  const open = (rel: string) => api.OpenInTool(st.root, rel).catch((e) => toast(errorText(e), "error"));
  let toolName = $derived(isLive ? "Live" : st.tool ? t(st.tool) : t("app"));
</script>

<svelte:window onclick={(e) => {
  const el = e.target as HTMLElement;
  if (setMenu && !el.closest(".open-wrap")) setMenu = false;
}} onkeydown={(e) => {
  // F12: the project in its program (several sets: their list)
  if (e.key !== "F12" || e.repeat || e.ctrlKey || e.altKey || e.shiftKey || document.querySelector("[aria-modal='true']")) return;
  if (!st.openable.length) return;
  e.preventDefault();
  if (st.openable.length === 1) open(st.openable[0]);
  else setMenu = !setMenu;
}} />

<header>
  <div class="title">
    {#if team}
      <div class="team">
        <span class="team-name" title={team.name}>{team.name}</span>
        {#if onteamsettings}
          <button class="ghost gear" title={t("Team settings: names, connection code, keys")} aria-label={t("Team settings: names, connection code, keys")}
            onclick={() => onteamsettings(team)}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
              <circle cx="12" cy="12" r="3"/>
            </svg>
          </button>
        {/if}
      </div>
    {/if}
    <div class="name">
      <ProjectIcon p={{ id: entry?.id, name: st.name, status: "downloaded", icon: entry?.icon, color: entry?.color }} size={30} />
      <h1>{st.name}{#if preuploads[st.root]} <PreuploadIcon p={preuploads[st.root]} />{/if}</h1>
    </div>
  </div>
  <div class="actions">
    <button class="ghost icon" onclick={oncheck} title={t("Check the project: Live version, samples, plugins")} aria-label={t("Project check")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>
    </button>
    <button class="ghost icon" class:spin={turning.on} onclick={onrefresh} title={t("Refresh") + " (F5)"} aria-label={t("Refresh")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/></svg>
    </button>
    <button class="ghost icon" onclick={() => api.ShowFolder(st.root)} title={t("Show folder")} aria-label={t("Show folder")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>
    </button>
    <!-- the project in its program: at the far right (F12, the key at the far right too) -->
    {#if st.openable.length === 1}
      <button class="primary" onclick={() => open(st.openable[0])}
        title={t("Open {file} in {tool}", { file: label(st.openable[0]), tool: (st.tool ? t(st.tool) : t("its program")) }) + " (F12)"}>▶ {t("Open in {tool}", { tool: toolName })}</button>
    {:else if st.openable.length > 1}
      <div class="open-wrap">
        <button class="primary" onclick={() => (setMenu = !setMenu)} title={t("Open in {tool}", { tool: (st.tool ? t(st.tool) : t("its program")) }) + " (F12)"}>▶ {t("Open in {tool}", { tool: toolName })} ▾</button>
        {#if setMenu}
          <div class="menu right surface-menu" role="menu">
            {#each st.openable as s}
              <button class="item" onclick={() => { setMenu = false; open(s); }}>{label(s)}</button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  header { display: flex; align-items: flex-start; padding: var(--sp-18) var(--sp-24) var(--sp-10); gap: var(--sp-16); }
  .title { flex: 1; min-width: 0; }
  .team { display: flex; align-items: center; gap: var(--sp-4); min-width: 0; margin-bottom: var(--sp-2); }
  .team-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--fs-sm); color: var(--muted); }
  .gear { flex: none; padding: var(--sp-2); line-height: 0; color: var(--faint); }
  .gear:hover:not(:disabled) { color: var(--text); }
  .gear svg { width: 14px; height: 14px; }
  .name { display: flex; align-items: center; gap: var(--sp-10); min-width: 0; }
  h1 { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  h1 { margin: 0; font-size: var(--fs-2xl); font-weight: var(--fw-semibold); }
  .actions { display: flex; gap: var(--sp-8); flex: none; }
  .icon { display: inline-flex; align-items: center; justify-content: center; padding: var(--sp-6) var(--sp-8); color: var(--muted); }
  .icon:hover:not(:disabled) { color: var(--text); }
  .ico { width: 16px; height: 16px; display: block; }
  .spin .ico { animation: spin .8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .open-wrap { position: relative; }
  .menu.right { left: auto; right: 0; min-width: 220px; max-height: 50vh; overflow: auto; }
  .menu {
    position: absolute; top: 32px; left: 0; z-index: var(--z-dropdown); min-width: 260px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg);
    box-shadow: var(--shadow-pop);
  }
  .item { display: flex; justify-content: space-between; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; gap: var(--sp-12); }
  .item:hover:not(:disabled) { background: var(--hover); }
</style>
