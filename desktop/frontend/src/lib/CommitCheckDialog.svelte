<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import Modal from "./Modal.svelte";

  // Before a commit: missing samples R3V has copies of (restore them
  // first?), what the project's checks warn about (a Unity asset without its
  // .meta), and files the rules now leave out.
  let { leaving, warnings, restorable, missingSamples, oncommit, onrestore, onclose }: {
    leaving: string[]; // no longer tracked
    warnings: string[];
    restorable: number;
    missingSamples: number;
    oncommit: () => void; // as it is
    onrestore: () => void; // the samples, then commit
    onclose: () => void;
  } = $props();
  const MAX = 12;
</script>

<Modal title={warnings.length || restorable ? t("Before you commit") : t("No longer tracked")} {onclose}>
  {#if restorable}
    <p class="restore-note">⚠ {tn(missingSamples, "{n} sample is missing.", "{n} samples are missing.")}
      {tn(restorable, "R3V has a copy: recover it into the project first, so the team hears the same.", "R3V has copies of {n}: recover them into the project first, so the team hears the same.")}</p>
  {/if}
  {#if warnings.length}
    <ul class="warnings">
      {#each warnings.slice(0, MAX) as w}<li>⚠ {w}</li>{/each}
      {#if warnings.length > MAX}<li class="faint">… {t("and {n} more", { n: warnings.length - MAX })}</li>{/if}
    </ul>
  {/if}
  {#if leaving.length}
    <p>{tn(leaving.length, "The project's rules now leave this file out of versions. It stays on this computer, and on your teammates' computers too.",
      "The project's rules now leave these {n} files out of versions. They stay on this computer, and on your teammates' computers too.")}</p>
    <ul class="untracked mono">
      {#each leaving.slice(0, MAX) as f}<li>{f}</li>{/each}
      {#if leaving.length > MAX}<li class="faint">… {t("and {n} more", { n: leaving.length - MAX })}</li>{/if}
    </ul>
  {/if}
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    {#if restorable}
      <button onclick={oncommit}>{t("Commit without them")}</button>
      <button class="primary" onclick={onrestore}>{t("Recover and commit")}</button>
    {:else}
      <button class="primary" onclick={oncommit}>{warnings.length ? t("Commit anyway") : t("Commit")}</button>
    {/if}
  {/snippet}
</Modal>

<style>
  .untracked { list-style: none; padding: var(--sp-8) var(--sp-12); margin: var(--sp-10) 0 0; background: var(--bg); border-radius: var(--radius-lg);
    font-size: var(--fs-md); max-height: 220px; overflow: auto; }
  .warnings { list-style: none; padding: 0; margin: 0 0 var(--sp-12); display: flex; flex-direction: column; gap: var(--sp-6);
    color: var(--warn); font-size: var(--fs-base); user-select: text; }
  .restore-note { color: var(--warn-text); }
</style>
