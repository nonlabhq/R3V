<script lang="ts">
  import { t } from "./i18n.svelte";
  import type { State } from "./api";

  // Beside the Changes list when no file is picked: the tracks you changed
  // (Live sets), or what to do.
  let { st }: { st: State } = $props();
  let tool = $derived(st.tool === "Ableton Live" ? "Live" : st.tool ? t(st.tool) : t("your app"));
</script>

<section>
  {#if st.myEdits.length}
    <h3>{t("Tracks you changed")}</h3>
    <ul class="tracks">
      {#each st.myEdits as e}
        <li>
          <span class="chg {e.change}"></span>
          <span>{e.name}</span>
          <span class="faint">{e.set}</span>
        </li>
      {/each}
    </ul>
    <p class="faint small">{t("Pick a file on the left for its details, history and, for samples, to listen.")}</p>
  {:else}
    <p class="muted">{st.changes.length ? t("Pick a file on the left to see what changed.") : t("No uncommitted changes. Work in {tool} and save (Ctrl+S) — your changes show up here.", { tool })}</p>
  {/if}
</section>

<style>
  h3 { font-size: var(--fs-sm); text-transform: uppercase; letter-spacing: .06em; color: var(--muted); margin: var(--sp-6) 0 var(--sp-10); }
  .small { font-size: var(--fs-sm); margin: var(--sp-10) 0 0; }
  .tracks { list-style: none; padding: 0; margin: 0 0 var(--sp-18); display: flex; flex-direction: column; gap: var(--sp-4); }
  .tracks li { display: flex; align-items: center; gap: var(--sp-10); }
  .chg { width: 8px; height: 8px; border-radius: 2px; background: var(--mod); }
  .chg.added { background: var(--add); }
  .chg.removed { background: var(--del); }
</style>
