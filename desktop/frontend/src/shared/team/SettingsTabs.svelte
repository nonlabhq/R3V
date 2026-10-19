<script lang="ts" generics="K extends string">
  import type { Snippet } from "svelte";

  // A settings page in tabs down the left (a team's: General, Workflow,
  // Activity, Usage): the tab picked is remembered on this computer. A tab
  // that isn't there (yet: its page still loading) shows the first one
  // meanwhile, and is shown once it comes.
  let { tabs, label, remember, content }: {
    tabs: { id: K; label: string }[];
    label: string;        // what the tab list is called (for screen readers)
    remember: string;     // where the tab picked is kept (localStorage)
    content: Snippet<[K]>;
  } = $props();

  // svelte-ignore state_referenced_locally
  let tab = $state<K>((() => {
    try { return (localStorage.getItem(remember) as K) || tabs[0].id; } catch { return tabs[0].id; }
  })());
  const shown = $derived(tabs.some((x) => x.id === tab) ? tab : tabs[0].id);
  function pick(x: K) {
    tab = x;
    try { localStorage.setItem(remember, x); } catch { /* not remembered */ }
  }
</script>

<div class="tabbed">
  <nav class="tabs" aria-label={label}>
    <div class="band caption" aria-hidden="true">{label}</div>
    <div class="list">
      {#each tabs as x (x.id)}
        <button class:on={shown === x.id} aria-current={shown === x.id ? "page" : undefined} onclick={() => pick(x.id)}>{x.label}</button>
      {/each}
    </div>
  </nav>
  <div class="content">{@render content(shown)}</div>
</div>

<style>
  /* The R3V look: the tabs beside the page; Flat: two panels, 2px apart,
     filling the height (bleed 1). */
  .tabbed { display: flex; align-items: stretch; gap: calc(var(--sp-28) * (1 - var(--bleed)) + var(--pane-gap)); min-height: 100%; }
  .content { flex: 1; min-width: 0; max-width: calc(720px + 100vw * var(--bleed)); background: var(--surface-head);
    padding: 0 calc(var(--sp-16) * var(--bleed)) calc(var(--sp-16) * var(--bleed)); }
  .tabs { flex: none; width: calc(160px + 60px * var(--bleed)); background: var(--surface-head); }
  /* (Flat: the band over the tabs; this look: a caption) */
  .caption { margin: 0 calc(var(--sp-10) * (1 - var(--bleed))) calc(var(--sp-8) * (1 - var(--bleed))); }
  .list { display: flex; flex-direction: column; gap: calc(var(--sp-2) * (1 - var(--bleed))); position: sticky; top: 0; padding-top: calc(var(--sp-4) * var(--bleed)); }
  .tabs button { justify-content: flex-start; text-align: left; border: none; background: transparent; color: var(--muted);
    padding: var(--sp-6) var(--sp-10) var(--sp-6) calc(var(--sp-10) + var(--sp-4) * var(--bleed)); border-radius: var(--row-radius); font-size: var(--fs-base); }
  .tabs button:hover:not(:disabled) { background: var(--hover-soft); color: var(--text); }
  .tabs button.on, .tabs button.on:hover:not(:disabled) { background: var(--select-bg); box-shadow: var(--select-bar); color: var(--text); font-weight: var(--fw-semibold); }
</style>
