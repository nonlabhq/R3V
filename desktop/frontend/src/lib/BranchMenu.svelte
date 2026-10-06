<script lang="ts">
  import { t } from "./i18n.svelte";
  import { ago, type State } from "./api";

  // The branch you are on, and its menu: switch to another, merge one into
  // it, or start a new one from where you are. (Above the Overview's graph.)
  let { st, onswitch, onmerge, onnewbranch }: {
    st: State;
    onswitch: (branch: string) => void;
    onmerge: (branch: string) => void; // into the current one
    onnewbranch: () => void;
  } = $props();

  let open = $state(false);
</script>

<svelte:window onclick={(e) => { if (open && !(e.target as HTMLElement).closest(".branch-wrap")) open = false; }} />

<div class="branch-wrap">
  <button class="branch" onclick={() => (open = !open)} disabled={!st.remoteUrl}
    title={st.remoteUrl ? t("Branches") : t("Share the project with a team to use branches")}>
    ⑂ {st.branch} ▾
  </button>
  {#if open}
    <div class="menu surface-menu" role="menu">
      <div class="menu-h">{t("Switch to")}</div>
      {#each st.branches as b (b.name)}
        <button class="item" disabled={b.current} onclick={() => { open = false; onswitch(b.name); }}>
          <span>{b.name}</span>
          <span class="faint">{b.current ? t("current") : b.latest ? `${b.latest.author} · ${ago(b.latest.time)}` : ""}</span>
        </button>
      {/each}
      <div class="sep"></div>
      <div class="menu-h">{t("Merge into {branch}", { branch: st.branch })}</div>
      {#each st.branches.filter((b) => !b.current) as b (b.name)}
        <button class="item" onclick={() => { open = false; onmerge(b.name); }}>{b.name}</button>
      {:else}
        <div class="item faint">{t("no other branches")}</div>
      {/each}
      <div class="sep"></div>
      <button class="item" onclick={() => { open = false; onnewbranch(); }}>{t("New branch from here…")}</button>
    </div>
  {/if}
</div>

<style>
  .branch-wrap { position: relative; }
  .branch { padding: var(--sp-4) var(--sp-10); font-size: var(--fs-md); }
  .menu {
    position: absolute; top: 32px; left: 0; z-index: var(--z-dropdown); min-width: 260px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .menu-h { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-6) var(--sp-8) var(--sp-2); }
  .item { display: flex; justify-content: space-between; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; gap: var(--sp-12); }
  .item:hover:not(:disabled) { background: var(--hover); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
</style>
