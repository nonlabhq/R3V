<script lang="ts">
  import { tick } from "svelte";
  import { t } from "./i18n.svelte";

  // A section's menu in the left panel (a right click on its header, or its
  // ⋯), for those who may change the team's sections; without a section
  // (the panel's Projects band), New section… alone; from a + (onadd),
  // Project and Section. Placed on the window, it closes on a click elsewhere or
  // Esc (focus back on opener). (No Move: a section is dragged to its place,
  // each person's own.)
  let { name = "", at, opener = null, onrename, ondelete, onadd, onnew, onclose }: {
    name?: string; // the section's ("": none, New section… only)
    at: { left: number; top: number };
    opener?: HTMLElement | null;
    onrename?: () => void;
    ondelete?: () => void;
    onadd?: () => void;
    onnew: () => void;
    onclose: () => void;
  } = $props();

  let menu = $state<HTMLElement>();
  const items = () => [...(menu?.querySelectorAll<HTMLElement>("[role=menuitem]") ?? [])];
  $effect(() => { if (opener && menu) tick().then(() => items()[0]?.focus()); });

  function close(refocus = false) {
    const back = opener;
    onclose();
    if (refocus) back?.focus();
  }
  // Does what was picked, then closes (what it acts on goes with the menu).
  function then(f: () => void) {
    f();
    close();
  }
  function keydown(e: KeyboardEvent) {
    const all = items(), i = all.indexOf(document.activeElement as HTMLElement);
    const go = (j: number) => { e.preventDefault(); all[(j + all.length) % all.length]?.focus(); };
    if (e.key === "ArrowDown") go(i + 1);
    else if (e.key === "ArrowUp") go(i < 0 ? -1 : i - 1);
    else if (e.key === "Home") go(0);
    else if (e.key === "End") go(-1);
  }
</script>

<svelte:window onpointerdown={(e) => {
  const el = e.target as Node;
  if (!menu?.contains(el) && !opener?.contains(el)) close();
}} onkeydown={(e) => { if (e.key === "Escape") { e.preventDefault(); close(true); } }} />

<!-- svelte-ignore a11y_interactive_supports_focus -->
<div class="sect-menu surface-menu" role="menu" aria-label={name || t("Sections")} bind:this={menu} style:left="{at.left}px" style:top="{at.top}px"
  onkeydown={keydown}>
  {#if name}
    <button class="item" role="menuitem" onclick={() => then(onrename!)}>{t("Rename…")}</button>
    <button class="item danger-text" role="menuitem" onclick={() => then(ondelete!)}>{t("Delete section…")}</button>
    <div class="sep"></div>
  {/if}
  <!-- (from a +: the + says add, so just what to add) -->
  {#if onadd}
    <button class="item" role="menuitem" onclick={() => then(onadd)}>{t("Project")}</button>
    <button class="item" role="menuitem" onclick={() => then(onnew)}>{t("Section")}</button>
  {:else}
    <button class="item" role="menuitem" onclick={() => then(onnew)}>{t("New section…")}</button>
  {/if}
</div>

<style>
  .sect-menu {
    position: fixed; z-index: var(--z-menu); min-width: 200px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .item { display: block; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; }
  .item:hover { background: var(--hover); }
  .danger-text { color: var(--danger); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-4) var(--sp-2); }
</style>
