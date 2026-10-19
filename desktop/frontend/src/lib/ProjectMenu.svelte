<script lang="ts">
  import { tick } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, errorText, type Program, type ProjectInfo, type TeamProject } from "./api";
  import { toast } from "./notify.svelte";
  import { chooseProgram, loadPrograms, openFolderWith, programLabel } from "./openwith";

  // A project's menu: the sidebar row's right-click, and the "…" on its card
  // in the team's home (one menu, so both offer the same). Placed on the
  // window (at: where it was asked for), it closes itself on a click
  // elsewhere or Esc (focus back on opener, the button that opened it).
  // Archiving is in the project's settings only (docs/design/archive.md).
  let { p, at, opener = null, onsettings, sections, onsection, onclose }: {
    p: TeamProject;
    at: { left: number; top: number };
    opener?: HTMLElement | null;
    onsettings: (p: TeamProject) => void;
    // Move to › (docs/design/sections.md): the team's sections, in this
    // person's order, for those who may change them (none: not offered).
    sections?: { id: string; name: string }[];
    onsection?: (p: TeamProject, section: string) => void; // ("": none)
    onclose: () => void;
  } = $props();

  let menu = $state<HTMLElement>();
  // "Open in" (read as the menu opens).
  let info = $state<ProjectInfo | null>(null);
  $effect(() => {
    const root = p.root;
    info = null;
    if (root && p.status === "downloaded") api.ProjectInfo(root).then((i) => { if (p.root === root) info = i; }).catch(() => {});
  });
  // Opened from the keyboard (or the "…"): the first item takes the focus.
  $effect(() => { if (opener && menu) tick().then(() => items()[0]?.focus()); });

  // A list beside the menu (the project's sets, or the programs to open its
  // folder in), kept open while the pointer crosses over to it.
  let openSub = $state<"" | "sets" | "programs" | "move">("");
  let subAt = $state({ left: 0, top: 0 });
  let subTimer: ReturnType<typeof setTimeout> | undefined;
  function showSub(el: HTMLElement, which: "sets" | "programs" | "move") {
    clearTimeout(subTimer);
    const r = el.getBoundingClientRect();
    subAt = { left: Math.min(r.right - 2, window.innerWidth - 220), top: r.top - 6 };
    openSub = which;
  }
  function hideSub() {
    clearTimeout(subTimer);
    subTimer = setTimeout(() => (openSub = ""), 180);
  }
  // The programs the folder can be opened in (as the header's "Open in"
  // menu lists them), read when that list first shows.
  let programs = $state<Program[] | null>(null);
  function showPrograms(el: HTMLElement) {
    showSub(el, "programs");
    if (programs) return;
    const root = p.root;
    // (not Explorer: the menu's own "Open folder" is it)
    loadPrograms(root).then((l) => { if (p.root === root) programs = l.filter((x) => x.id !== "explorer"); });
  }

  function close(refocus = false) {
    clearTimeout(subTimer);
    const back = opener; // (read before: closing takes the props away)
    onclose();
    if (refocus) back?.focus();
  }
  const toolName = (tool: string) => (tool === "Ableton Live" ? "Live" : tool ? t(tool) : t("its program"));
  // Closes, then does what was picked (with the project: closing takes the
  // props away).
  function then(f: (p: TeamProject) => void) {
    const q = p;
    close();
    f(q);
  }
  const openIn = (rel: string) => then((q) => api.OpenInTool(q.root, rel).catch((e) => toast(errorText(e), "error")));
  const openLabel = (rel: string) => (rel === "." ? p.name : rel);
  const openFolderIn = (prog: Program | null) => then((q) => (prog ? openFolderWith(q.root, prog) : chooseProgram(q.root)));
  // (a section that isn't there any more: none)
  let inSection = $derived(sections?.some((s) => s.id === p.section) ? p.section ?? "" : "");
  function moveTo(id: string) {
    const was = inSection, move = onsection!; // (read before: closing takes the props away)
    then((q) => { if (id !== was) move(q, id); });
  }

  // ↑ ↓ Home End through the items shown (a list beside included), → into
  // a list beside, ← back out of it.
  const ITEM = "[role=menuitem], [role=menuitemradio]";
  const items = () => [...(menu?.querySelectorAll<HTMLElement>(ITEM) ?? [])];
  async function keydown(e: KeyboardEvent) {
    const all = items(), i = all.indexOf(document.activeElement as HTMLElement);
    const go = (j: number) => { e.preventDefault(); all[(j + all.length) % all.length]?.focus(); };
    const el = document.activeElement as HTMLElement;
    if (e.key === "ArrowDown") go(i + 1);
    else if (e.key === "ArrowUp") go(i < 0 ? -1 : i - 1);
    else if (e.key === "Home") go(0);
    else if (e.key === "End") go(-1);
    else if (e.key === "ArrowRight" && el?.getAttribute("aria-haspopup")) {
      e.preventDefault();
      el.click();
      await tick();
      el.parentElement?.querySelector<HTMLElement>(`.submenu :is(${ITEM})`)?.focus();
    } else if (e.key === "ArrowLeft" && el?.closest(".submenu")) {
      e.preventDefault();
      openSub = "";
      el.closest(".sub")?.querySelector<HTMLElement>("[aria-haspopup]")?.focus();
    }
  }
</script>

<svelte:window onpointerdown={(e) => {
  const el = e.target as Node;
  if (!menu?.contains(el) && !opener?.contains(el)) close();
}} onkeydown={(e) => { if (e.key === "Escape") { e.preventDefault(); close(true); } }} />

<!-- svelte-ignore a11y_interactive_supports_focus -->
<div class="row-menu surface-menu" role="menu" aria-label={p.name} bind:this={menu} style:left="{at.left}px" style:top="{at.top}px" onkeydown={keydown}>
  {#if info && info.openable.length === 1}
    <button class="item" role="menuitem" onclick={() => openIn(info!.openable[0])}>{t("Open in {tool}", { tool: toolName(info.tool) })}</button>
  {:else if info && info.openable.length > 1}
    <div class="sub" role="none" onmouseenter={(e) => showSub(e.currentTarget.firstElementChild as HTMLElement, "sets")} onmouseleave={hideSub}>
      <button class="item has-sub" role="menuitem" aria-haspopup="menu" aria-expanded={openSub === "sets"} onclick={(e) => showSub(e.currentTarget, "sets")}>
        {t("Open in {tool}", { tool: toolName(info.tool) })}<span class="arrow">›</span>
      </button>
      {#if openSub === "sets"}
        <div class="row-menu submenu surface-menu" role="menu" tabindex="-1" style:left="{subAt.left}px" style:top="{subAt.top}px"
          onmouseenter={() => clearTimeout(subTimer)} onmouseleave={hideSub}>
          {#each info.openable as rel}
            <button class="item" role="menuitem" onclick={() => openIn(rel)}>{openLabel(rel)}</button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
  {#if p.root && p.status !== "missing"}
    <button class="item" role="menuitem" onclick={() => then((q) => api.ShowFolder(q.root))}>{t("Open folder")}</button>
  {/if}
  {#if p.root && p.status === "downloaded"}
    <div class="sub" role="none" onmouseenter={(e) => showPrograms(e.currentTarget.firstElementChild as HTMLElement)} onmouseleave={hideSub}>
      <button class="item has-sub" role="menuitem" aria-haspopup="menu" aria-expanded={openSub === "programs"}
        onclick={(e) => showPrograms(e.currentTarget)}>{t("Open the folder in")}<span class="arrow">›</span></button>
      {#if openSub === "programs"}
        <div class="row-menu submenu surface-menu" role="menu" tabindex="-1" aria-label={t("Open the folder in")}
          style:left="{subAt.left}px" style:top="{subAt.top}px" onmouseenter={() => clearTimeout(subTimer)} onmouseleave={hideSub}>
          {#if programs === null}
            <div class="item faint">{t("Looking for programs…")}</div>
          {:else}
            {#each programs as prog (prog.id)}
              <button class="item" role="menuitem" title={prog.path || undefined} onclick={() => openFolderIn(prog)}>{programLabel(prog)}</button>
            {/each}
          {/if}
          <div class="sep"></div>
          <button class="item" role="menuitem" onclick={() => openFolderIn(null)}>{t("Choose a program…")}</button>
        </div>
      {/if}
    </div>
  {/if}
  {#if !p.workspace}
    {#if p.root && p.status !== "missing"}<div class="sep"></div>{/if}
    {#if sections && onsection && p.id && !p.archived}
      <div class="sub" role="none" onmouseenter={(e) => showSub(e.currentTarget.firstElementChild as HTMLElement, "move")} onmouseleave={hideSub}>
        <button class="item has-sub" role="menuitem" aria-haspopup="menu" aria-expanded={openSub === "move"}
          onclick={(e) => showSub(e.currentTarget, "move")}>{t("Move to")}<span class="arrow">›</span></button>
        {#if openSub === "move"}
          <div class="row-menu submenu surface-menu" role="menu" tabindex="-1" aria-label={t("Move to")}
            style:left="{subAt.left}px" style:top="{subAt.top}px" onmouseenter={() => clearTimeout(subTimer)} onmouseleave={hideSub}>
            <button class="item pick" role="menuitemradio" aria-checked={inSection === ""} onclick={() => moveTo("")}>{t("None")}</button>
            {#if sections.length}<div class="sep"></div>{/if}
            {#each sections as s (s.id)}
              <button class="item pick" role="menuitemradio" aria-checked={inSection === s.id} title={s.name} onclick={() => moveTo(s.id)}>{s.name}</button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    <button class="item" role="menuitem" onclick={() => then(onsettings)}>{t("Settings…")}</button>
  {/if}
</div>

<style>
  .row-menu {
    position: fixed; z-index: var(--z-menu); min-width: 220px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .item {
    display: flex; flex-direction: column; align-items: flex-start; gap: 1px; width: 100%; border: none;
    background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left;
  }
  .item:hover { background: var(--hover); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-4) var(--sp-2); }
  .sub { position: relative; }
  .has-sub { flex-direction: row; justify-content: space-between; align-items: center; }
  .arrow { color: var(--faint); }
  .row-menu.submenu { position: fixed; right: auto; min-width: 200px; max-width: 340px; z-index: var(--z-submenu); }
  .row-menu.submenu .item { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; }
  .has-sub[aria-expanded="true"] { background: var(--hover); }
  /* (the section it's in: a check before it) */
  .row-menu.submenu .pick { position: relative; padding-left: var(--sp-24); }
  .pick[aria-checked="true"]::before { content: "✓"; position: absolute; left: var(--sp-8); color: var(--accent); }
  div.item.faint { color: var(--faint); cursor: default; }
  div.item.faint:hover { background: none; }
</style>
