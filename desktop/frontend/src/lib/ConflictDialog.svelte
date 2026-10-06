<script lang="ts">
  import { t } from "./i18n.svelte";
  import Modal from "./Modal.svelte";
  import type { Conflict } from "./api";

  let { conflicts, onresolve, onclose }: {
    conflicts: Conflict[];
    onresolve: (resolutions: Record<string, string>) => void;
    onclose: () => void;
  } = $props();

  let choices = $state<Record<string, string>>({});
  let complete = $derived(conflicts.every((c) => choices[c.key]));

  function all(choice: string) {
    for (const c of conflicts) {
      choices[c.key] = choice === "both" && !c.canKeepBoth ? "ours" : choice;
    }
  }
</script>

<Modal title={t("You and the team changed the same things")} {onclose} width={680}>
  <p class="muted">
    {t("Everything else was combined automatically. Choose what to keep for each of these:")}
  </p>
  <div class="bulk row">
    <span class="faint">{t("All:")}</span>
    <button class="ghost" onclick={() => all("ours")}>{t("Keep mine")}</button>
    <button class="ghost" onclick={() => all("theirs")}>{t("Take theirs")}</button>
    <button class="ghost" onclick={() => all("both")}>{t("Keep both")}</button>
  </div>
  <ul>
    {#each conflicts as c (c.key)}
      <li>
        <div class="what">
          <div class="unit">{c.unit === c.file ? c.file : c.unit}</div>
          <div class="faint">{c.unit === c.file ? "" : c.file + " · "}{c.description}</div>
        </div>
        <div class="seg" role="radiogroup" aria-label={c.unit}>
          <button class:on={choices[c.key] === "ours"} onclick={() => (choices[c.key] = "ours")}>{t("Keep mine")}</button>
          <button class:on={choices[c.key] === "theirs"} onclick={() => (choices[c.key] = "theirs")}>{t("Take theirs")}</button>
          {#if c.canKeepBoth}
            <button class:on={choices[c.key] === "both"} onclick={() => (choices[c.key] = "both")}
              title={t("Keeps your version and adds theirs as a copy")}>{t("Keep both")}</button>
          {/if}
        </div>
      </li>
    {/each}
  </ul>
  {#snippet footer()}
    <button onclick={onclose}>{t("Cancel")}</button>
    <button class="primary" disabled={!complete} onclick={() => onresolve({ ...choices })}>{t("Continue")}</button>
  {/snippet}
</Modal>

<style>
  ul { list-style: none; padding: 0; margin: var(--sp-8) 0 0; display: flex; flex-direction: column; gap: var(--sp-8); }
  li {
    display: flex; align-items: center; gap: var(--sp-14); padding: var(--sp-10) var(--sp-12);
    background: var(--bg); border: var(--border-width) solid var(--line); border-radius: var(--radius-lg);
  }
  .what { flex: 1; min-width: 0; }
  .unit { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .seg { display: flex; }
  .seg button { border-radius: 0; margin-left: -1px; padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .seg button:first-child { border-radius: var(--radius) 0 0 var(--radius); }
  .seg button:last-child { border-radius: 0 var(--radius) var(--radius) 0; }
  .seg button.on { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
  .bulk { margin: var(--sp-4) 0; }
  .bulk button { padding: var(--sp-2) var(--sp-8); font-size: var(--fs-md); }
</style>
