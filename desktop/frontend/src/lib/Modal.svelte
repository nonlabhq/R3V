<script lang="ts" module>
  // Open dialogs, the newest last: Esc closes only the one on top.
  const open: symbol[] = [];
</script>

<script lang="ts">
  import { t } from "./i18n.svelte";
  import type { Snippet } from "svelte";

  let { title, onclose, width = 560, backdropCloses = true, escCloses = true, children, footer }: {
    title: string;
    onclose: () => void;
    width?: number;
    // false for forms that take a while to fill in: a click beside them
    // doesn't close them (✕ and Esc do).
    backdropCloses?: boolean;
    // false while a step it runs can't be left (Esc does nothing then).
    escCloses?: boolean;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  const me = Symbol();
  $effect(() => {
    open.push(me);
    return () => { open.splice(open.indexOf(me), 1); };
  });
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== "Escape" || open[open.length - 1] !== me || e.defaultPrevented) return;
    e.preventDefault();
    if (escCloses) onclose();
  }
</script>

<svelte:window {onkeydown} />

<div class="backdrop" role="presentation" onclick={(e) => { if (backdropCloses && e.target === e.currentTarget) onclose(); }}>
  <div class="modal surface-dialog" style:width="{width}px" role="dialog" aria-modal="true" aria-label={title}>
    <header>
      <h2>{title}</h2>
      <button class="ghost close" onclick={onclose} aria-label={t("Close")} title={t("Close") + " (Esc)"}>✕</button>
    </header>
    <div class="body">{@render children()}</div>
    {#if footer}
      <footer>{@render footer()}</footer>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed; inset: 0; background: var(--scrim); backdrop-filter: var(--scrim-filter);
    display: flex; align-items: center; justify-content: center; z-index: var(--z-dialog);
  }
  .modal {
    max-width: calc(100vw - 48px); max-height: calc(100vh - 48px);
    position: relative; border: var(--border-width) solid var(--line); border-radius: var(--radius-xl);
    display: flex; flex-direction: column; box-shadow: var(--shadow-dialog);
  }
  header { display: flex; align-items: center; padding: var(--sp-16) var(--sp-18) var(--sp-8); }
  h2 { margin: 0; font-size: var(--fs-lg); font-weight: var(--fw-semibold); flex: 1; }
  .close { padding: var(--sp-2) var(--sp-8); }
  .body { padding: var(--sp-6) var(--sp-18) var(--sp-12); overflow: auto; }
  footer {
    display: flex; justify-content: flex-end; gap: var(--sp-8); padding: var(--sp-12) var(--sp-18);
    border-top: var(--border-width) solid var(--line);
  }
</style>
