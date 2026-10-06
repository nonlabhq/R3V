<script lang="ts">
  import { toasts, dismiss } from "./notify.svelte";
</script>

<div class="toasts">
  {#each toasts as t (t.id)}
    <button class="toast surface-menu {t.kind}" onclick={() => dismiss(t.id)}>{t.text}</button>
  {/each}
</div>

<style>
  .toasts {
    position: fixed; right: 18px; bottom: 18px; z-index: var(--z-toast);
    display: flex; flex-direction: column; gap: var(--sp-8); max-width: 380px;
  }
  .toast {
    text-align: left; white-space: pre-line; padding: var(--sp-10) var(--sp-14); border-radius: var(--radius-lg);
    position: relative; border: var(--border-width) solid var(--line); box-shadow: var(--shadow-pop);
  }
  .ok { border-left: 3px solid var(--ok); }
  .warn { border-left: 3px solid var(--warn); }
  .error { border-left: 3px solid var(--danger); }
  .info { border-left: 3px solid var(--mod); }
</style>
