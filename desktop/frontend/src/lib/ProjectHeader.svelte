<script lang="ts">
  import { t } from "./i18n.svelte";
  import { api, errorText, type Program, type State, type TeamProject, type TeamSummary } from "./api";
  import { chooseProgram, loadPrograms, openFolderWith, programLabel } from "./openwith";
  import ProjectIcon from "$shared/ProjectIcon.svelte";
  import { toast } from "./notify.svelte";
  import PreuploadIcon from "./PreuploadIcon.svelte";
  import { preuploads } from "./preupload.svelte";
  import type { Snippet } from "svelte";

  // The top of a project's page: its icon, then its team (a click: the
  // team's settings) and its section, if it's in one, over its name; in the middle the notice slot (transfers, the team's new
  // versions: its size kept, so what comes and goes moves nothing); and
  // opening it in its tool or another program. (Branches are above the Overview's graph;
  // settings, and the project check, are a tab; refreshing is F5.)
  let { st, team, entry, onteamsettings, notice }: {
    st: State;
    team?: TeamSummary; // none: on this computer only
    entry?: Pick<TeamProject, "id" | "icon" | "color" | "picture"> & { sectionName?: string }; // the team's look for it, its section
    onteamsettings?: (team: TeamSummary) => void;
    notice?: Snippet;
  } = $props();

  let isLive = $derived(st.tool === "Ableton Live");
  const label = (rel: string) => (rel === "." ? st.name ?? t("the project") : rel);
  const open = (rel: string) => api.OpenInTool(st.root, rel).catch((e) => toast(errorText(e), "error"));
  let toolName = $derived(isLive ? "Live" : st.tool ? t(st.tool) : t("app"));

  // "Open in": a split button. Its main part opens the project in its tool
  // (when its kind has one), else its folder in the program last picked
  // for it; with neither it opens the menu. The menu: the project's files
  // its tool opens, then the folder in another program (found when the menu
  // first opens) or one the user chooses. What was picked is kept by R3V on
  // this computer (openwith.ts).
  let menu = $state(false);
  let programs = $state<Program[] | null>(null);
  let programsFor = "";
  let last = $state<{ id: string; name: string } | null>(null);
  let lastFor = $state("");
  $effect(() => {
    const root = st.root;
    last = null;
    lastFor = root;
    api.OpenedWith(root).then((l) => { if (lastFor === root) last = l ?? null; }).catch(() => {});
  });
  let main = $derived<"tool" | "program" | "menu">(st.openable.length ? "tool" : last ? "program" : "menu");
  let wrap: HTMLElement | undefined = $state();
  let toggle: HTMLButtonElement | undefined = $state();

  let mainLabel = $derived(main === "tool" ? t("Open in {tool}", { tool: toolName })
    : main === "program" ? t("Open in {tool}", { tool: programLabel(last!) }) : t("Open in…"));
  let mainTitle = $derived((main === "tool" ? (st.openable.length === 1
      ? t("Open {file} in {tool}", { file: label(st.openable[0]), tool: (st.tool ? t(st.tool) : t("its program")) })
      : t("Open in {tool}", { tool: (st.tool ? t(st.tool) : t("its program")) }))
    : main === "program" ? t("Open the project folder in {program}", { program: programLabel(last!) })
    : t("Open the project folder in another program")) + " (F12)");

  function first() { requestAnimationFrame(() => wrap?.querySelector<HTMLElement>("[role=menuitem]")?.focus()); }

  function openMenu() {
    menu = true;
    first();
    const root = st.root;
    if (programsFor !== root) programs = null; // another project's chosen ones
    loadPrograms(root).then((l) => {
      if (root !== st.root) return;
      programs = l;
      programsFor = root;
      // a chosen one no longer there is forgotten, and the last with it
      if (last && !l.some((p) => p.id === last!.id)) {
        api.OpenedWith(root).then((x) => { if (root === st.root) last = x ?? null; }).catch(() => {});
      }
    });
  }
  function closeMenu(focus = false) { menu = false; if (focus) toggle?.focus(); }

  function mainAction() {
    if (main === "tool" && st.openable.length === 1) open(st.openable[0]);
    else if (main === "program") openWith(last!);
    else if (menu) closeMenu();
    else openMenu();
  }

  async function openWith(p: { id: string; name: string }) {
    closeMenu();
    const root = st.root;
    if (await openFolderWith(root, p) && root === st.root) last = { id: p.id, name: p.name };
  }

  async function choose() {
    closeMenu();
    const root = st.root;
    const p = await chooseProgram(root);
    if (!p || root !== st.root) return;
    programs = null;
    last = { id: p.id, name: p.name };
  }

  // ↑ ↓ Home End through the menu, Esc back to the ▾, Tab out closes it.
  function menuKey(e: KeyboardEvent) {
    const items = [...(wrap?.querySelectorAll<HTMLElement>("[role=menuitem]") ?? [])];
    const i = items.indexOf(document.activeElement as HTMLElement);
    const to = { ArrowDown: i + 1, ArrowUp: i < 0 ? items.length - 1 : i - 1, Home: 0, End: items.length - 1 }[e.key];
    if (to !== undefined && items.length) {
      e.preventDefault();
      items[(to + items.length) % items.length].focus();
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      closeMenu(true);
    } else if (e.key === "Tab") closeMenu();
  }
</script>

<svelte:window onclick={(e) => {
  const el = e.target as HTMLElement;
  if (menu && !el.closest(".open-wrap")) closeMenu();
}} onkeydown={(e) => {
  if (menu && e.key === "Escape") { e.preventDefault(); closeMenu(); return; }
  // F12: the main action of "Open in"
  if (e.key !== "F12" || e.repeat || e.ctrlKey || e.altKey || e.shiftKey || document.querySelector("[aria-modal='true']")) return;
  e.preventDefault();
  mainAction();
}} />

<header>
  <ProjectIcon p={{ id: entry?.id, name: st.name, status: "downloaded", icon: entry?.icon, color: entry?.color, picture: entry?.picture }} size={30} />
  <div class="title">
    <!-- where it is: its team (a click opens the team's settings) -->
    {#if team}
      <div class="where">
        {#if onteamsettings}
          <button class="team-name" title={t("Team settings: names, connection code, keys")} onclick={() => onteamsettings(team)}>{team.name}</button>
        {:else}
          <span class="team-name" title={team.name}>{team.name}</span>
        {/if}
        {#if entry?.sectionName}
          <span class="where-sep" aria-hidden="true">/</span><span class="section-name" title={t("Section")}>{entry.sectionName}</span>
        {/if}
      </div>
    {/if}
    <h1>{st.name}{#if preuploads[st.root]} <PreuploadIcon p={preuploads[st.root]} />{/if}</h1>
  </div>
  {#if notice}{@render notice()}{:else}<div class="spacer"></div>{/if}
  <div class="actions">
    <button class="icon" onclick={() => api.ShowFolder(st.root)} title={t("Show folder")} aria-label={t("Show folder")}>
      <svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></svg>
    </button>
    <!-- the project in its program: at the far right (F12, the key at the far right too) -->
    <div class="open-wrap" bind:this={wrap}>
      <div class="split">
        <button class="main" class:primary={main !== "menu"} onclick={mainAction} title={mainTitle}
          aria-haspopup={main === "menu" || st.openable.length > 1 ? "menu" : undefined}>{main !== "menu" ? "▶ " : ""}{mainLabel}</button>
        <button class="more" class:primary={main !== "menu"} bind:this={toggle} onclick={() => (menu ? closeMenu() : openMenu())}
          aria-haspopup="menu" aria-expanded={menu} aria-label={t("More ways to open the project")} title={t("More ways to open the project")}>▾</button>
      </div>
      {#if menu}
        <div class="menu right surface-menu" role="menu" tabindex="-1" aria-label={t("Open in…")} onkeydown={menuKey}>
          {#if st.openable.length}
            <div class="menu-h">{t("Project files")}</div>
            {#each st.openable as s}
              <button class="item" role="menuitem" onclick={() => { closeMenu(); open(s); }}>{label(s)}</button>
            {/each}
            <div class="sep"></div>
          {/if}
          <div class="menu-h">{t("Open the folder in")}</div>
          {#if programs === null}
            <div class="item faint">{t("Looking for programs…")}</div>
          {:else}
            {#each programs as p (p.id)}
              <button class="item" role="menuitem" onclick={() => openWith(p)} title={p.path || undefined}>{programLabel(p)}</button>
            {/each}
          {/if}
          <div class="sep"></div>
          <button class="item" role="menuitem" onclick={choose}>{t("Choose a program…")}</button>
        </div>
      {/if}
    </div>
  </div>
</header>

<style>
  header { display: flex; align-items: center; min-height: 62px; padding: var(--sp-12) var(--sp-24) var(--sp-6); gap: var(--sp-12); }
  .title { flex: 0 1 auto; min-width: 120px; max-width: 28%; display: flex; flex-direction: column; justify-content: center; }
  .spacer { flex: 1; }
  .where { display: flex; align-items: center; gap: var(--sp-6); min-width: 0; font-size: var(--fs-xs); line-height: 1.3; }
  .team-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 0; border: none; background: none;
    font-size: inherit; color: var(--muted); text-align: left; }
  button.team-name:hover:not(:disabled) { background: none; color: var(--text); text-decoration: underline; }
  .where-sep { color: var(--faint); }
  .section-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); }
  h1 { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  h1 { margin: 0; font-size: var(--fs-lg); font-weight: var(--fw-semibold); line-height: 1.3; }
  .actions { display: flex; gap: var(--sp-8); flex: none; }
  .icon { display: inline-flex; align-items: center; justify-content: center; width: 32px; padding: 0; color: var(--muted); }
  .icon:hover:not(:disabled) { color: var(--text); }
  .ico { width: 16px; height: 16px; display: block; }
  .open-wrap { position: relative; }
  .menu.right { left: auto; right: 0; min-width: 220px; max-height: 50vh; overflow: auto; }
  .menu {
    position: absolute; top: 32px; left: 0; z-index: var(--z-dropdown); min-width: 260px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg);
    box-shadow: var(--shadow-pop);
  }
  .item { display: flex; justify-content: space-between; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; gap: var(--sp-12); }
  .item:hover:not(:disabled), .item:focus-visible { background: var(--hover); }
  div.item { cursor: default; }
  .faint { color: var(--faint); }
  .menu-h { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-6) var(--sp-8) var(--sp-2); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
  /* the two parts of the split button, one button to the eye */
  .split { display: flex; }
  .split .main { border-top-right-radius: 0; border-bottom-right-radius: 0; }
  .split .more { border-top-left-radius: 0; border-bottom-left-radius: 0; border-left-width: 0; padding: var(--sp-6) var(--sp-8); }
  .split .more.primary { border-left: var(--border-width) solid var(--accent-hover); }
</style>
