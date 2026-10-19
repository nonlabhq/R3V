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
    {#each tabs as x (x.id)}
      <button class:on={shown === x.id} aria-current={shown === x.id ? "page" : undefined} onclick={() => pick(x.id)}>{x.label}</button>
    {/each}
  </nav>
  <div class="content">{@render content(shown)}</div>
</div>

<style>
  .tabbed { display: flex; align-items: flex-start; gap: var(--sp-28); }
  .content { flex: 1; min-width: 0; max-width: 720px; }
  .tabs { flex: none; width: 160px; display: flex; flex-direction: column; gap: var(--sp-2); position: sticky; top: 0; }
  .tabs button { justify-content: flex-start; text-align: left; border: none; background: transparent; color: var(--muted);
    padding: var(--sp-6) var(--sp-10); border-radius: var(--radius); font-size: var(--fs-base); }
  .tabs button:hover:not(:disabled) { background: var(--hover-soft); color: var(--text); }
  .tabs button.on { background: var(--panel-2); color: var(--text); font-weight: var(--fw-semibold); }
</style>
