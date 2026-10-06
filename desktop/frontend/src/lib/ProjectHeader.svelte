<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type State } from "./api";
  import { toast } from "./notify.svelte";
  import PreuploadIcon from "./PreuploadIcon.svelte";
  import { preuploads } from "./preupload.svelte";

  // The top of a project's page: its name, and opening it in its tool.
  // (Branches are above the Overview's graph; settings are a tab.)
  let { st, refreshing, oncheck, onrefresh }: {
    st: State;
    refreshing: boolean;
    oncheck: () => void;
    onrefresh: () => void;
  } = $props();

  let setMenu = $state(false);
  let isLive = $derived(st.tool === "Ableton Live");
  const label = (rel: string) => (rel === "." ? st.name ?? t("the project") : rel);
  const open = (rel: string) => api.OpenInTool(st.root, rel).catch((e) => toast(errorText(e), "error"));
  let toolName = $derived(isLive ? "Live" : st.tool ? t(st.tool) : t("app"));
</script>

<svelte:window onclick={(e) => {
  const el = e.target as HTMLElement;
  if (setMenu && !el.closest(".open-wrap")) setMenu = false;
}} />

<header>
  <div class="title">
    <h1>{st.name}{#if preuploads[st.root]} <PreuploadIcon p={preuploads[st.root]} />{/if}</h1>
  </div>
  <div class="actions">
    {#if st.openable.length === 1}
      <button onclick={() => open(st.openable[0])}
        title={t("Open {file} in {tool}", { file: label(st.openable[0]), tool: (st.tool ? t(st.tool) : t("its program")) })}>▶ {t("Open in {tool}", { tool: toolName })}</button>
    {:else if st.openable.length > 1}
      <div class="open-wrap">
        <button onclick={() => (setMenu = !setMenu)} title={t("Open in {tool}", { tool: (st.tool ? t(st.tool) : t("its program")) })}>▶ {t("Open in {tool}", { tool: toolName })} ▾</button>
        {#if setMenu}
          <div class="menu right surface-menu" role="menu">
            {#each st.openable as s}
              <button class="item" onclick={() => { setMenu = false; open(s); }}>{label(s)}</button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    <button class="ghost icon" onclick={oncheck} title={t("Check the project: Live version, samples, plugins")} aria-label={t("Project check")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"/></svg>
    </button>
    <button class="ghost icon" class:spin={refreshing} onclick={onrefresh} title={t("Refresh")} aria-label={t("Refresh")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/></svg>
    </button>
    <button class="ghost icon" onclick={() => api.ShowFolder(st.root)} title={t("Show folder")} aria-label={t("Show folder")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>
    </button>
  </div>
</header>

<style>
  header { display: flex; align-items: flex-start; padding: var(--sp-18) var(--sp-24) var(--sp-10); gap: var(--sp-16); }
  .title { flex: 1; min-width: 0; }
  h1 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
