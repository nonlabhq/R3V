<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import Modal from "$shared/Modal.svelte";

  // Putting the files back after a switch that stopped halfway would replace
  // these files, changed since R3V stopped: said first, done only when the
  // user agrees.
  let { paths, onconfirm, onclose }: { paths: string[]; onconfirm: () => void; onclose: () => void } = $props();
  const MAX = 50;
</script>

<Modal title={t("Put the files back?")} {onclose}>
  <p>{tn(paths.length, "This file changed after R3V stopped: putting the files back replaces it (or deletes it, when the version doesn't have it).",
    "These {n} files changed after R3V stopped: putting the files back replaces them (or deletes those the version doesn't have).")}</p>
  <ul class="files">
    {#each paths.slice(0, MAX) as f}<li class="mono">{f}</li>{/each}
    {#if paths.length > MAX}<li class="faint">… {t("and {n} more", { n: paths.length - MAX })}</li>{/if}
  </ul>
  <p class="warn">{t("Copy what you want to keep of them somewhere else first. This can't be undone.")}</p>
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="danger" onclick={onconfirm}>{t("Replace them and put files back")}</button>
  {/snippet}
</Modal>

<style>
  .files { margin: var(--sp-4) 0 var(--sp-10); padding-left: var(--sp-18); font-size: var(--fs-md); max-height: 220px; overflow: auto; user-select: text; }
  .mono { font-family: var(--font-mono); }
  .warn { color: var(--warn); }
</style>
