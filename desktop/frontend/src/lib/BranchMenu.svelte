<script lang="ts">
  import { t } from "./i18n.svelte";
  import { ago, type State } from "./api";
  import { branchLabel, branchLane } from "./branches";

  // The branch you are on, and its menu: switch to another, merge one into
  // it, or start a new one from where you are. (Above the Overview's graph.)
  let { st, onswitch, onmerge, onnewbranch, onsettings, onmilestone }: {
    st: State;
    onswitch: (branch: string) => void;
    onmerge: (branch: string) => void; // into the current one
    onnewbranch: () => void;
    onsettings?: (branch: string) => void; // a branch's settings (where the team keeps names)
    onmilestone?: (version: string) => void; // show a milestone's version
  } = $props();
  const label = (key: string) => branchLabel(st.branches, key);

  let open = $state(false);
  let current = $derived(st.branches.find((b) => b.current));
  let others = $derived(st.branches.filter((b) => !b.current));
</script>

<svelte:window onclick={(e) => { if (open && !(e.target as HTMLElement).closest(".branch-wrap")) open = false; }}
  onkeydown={(e) => { if (open && e.key === "Escape") { e.preventDefault(); open = false; } }} />

<div class="branch-wrap">
  <button class="branch" onclick={() => (open = !open)} disabled={!st.remoteUrl}
    title={st.remoteUrl ? t("Branches") : t("Share the project with a team to use branches")}>
    <span class="cap">{t("Current branch")}</span>
    <span class="name"><span class="dot" style:--c="var(--lane-{branchLane(st.branches, st.branch)})"></span>{label(st.branch)} ▾</span>
  </button>
  {#if open}
    <div class="menu surface-menu" role="menu">
      <div class="menu-h">{t("Current branch")}</div>
      <div class="item current">
        <span class="bn"><span class="dot" style:--c="var(--lane-{branchLane(st.branches, st.branch)})"></span>{label(st.branch)}</span>
        <span class="faint">{current?.latest ? `${current.latest.author} · ${ago(current.latest.time)}` : ""}</span>
        {#if st.branchNames && onsettings}
          <button class="ghost gear" title={t("Branch settings")} aria-label={t("Branch settings")}
            onclick={() => { open = false; onsettings(st.branch); }}>⋯</button>
        {/if}
      </div>
      <div class="sep"></div>
      <div class="menu-h">{t("Switch to")}</div>
      {#each others as b (b.name)}
        <div class="row">
          <button class="item" onclick={() => { open = false; onswitch(b.name); }}>
            <span class="bn"><span class="dot" style:--c="var(--lane-{branchLane(st.branches, b.name)})"></span>{label(b.name)}</span>
            <span class="faint">{b.latest ? `${b.latest.author} · ${ago(b.latest.time)}` : ""}</span>
          </button>
          {#if st.branchNames && onsettings}
            <button class="ghost gear" title={t("Branch settings")} aria-label={t("Branch settings")}
              onclick={() => { open = false; onsettings(b.name); }}>⋯</button>
          {/if}
        </div>
      {:else}
        <div class="item faint">{t("no other branches")}</div>
      {/each}
      <div class="sep"></div>
      <div class="menu-h">{t("Merge into {branch}", { branch: label(st.branch) })}</div>
      {#each others as b (b.name)}
        <button class="item" onclick={() => { open = false; onmerge(b.name); }}>
          <span class="bn"><span class="dot" style:--c="var(--lane-{branchLane(st.branches, b.name)})"></span>{label(b.name)}</span>
        </button>
      {:else}
        <div class="item faint">{t("no other branches")}</div>
      {/each}
      {#if st.milestones?.length && onmilestone}
        <div class="sep"></div>
        <div class="menu-h">{t("Milestones")}</div>
        <div class="milestones">
          {#each st.milestones as m (m.id)}
            <button class="item" onclick={() => { open = false; onmilestone(m.version); }} title={m.note}>
              <span class="bn">⚑ {m.name}</span>
              <span class="faint">{ago(m.time)}</span>
            </button>
          {/each}
        </div>
      {/if}
      <div class="sep"></div>
      <button class="item" onclick={() => { open = false; onnewbranch(); }}>{t("New branch from here…")}</button>
    </div>
  {/if}
</div>

<style>
  .branch-wrap { position: relative; }
  .branch { display: flex; flex-direction: column; align-items: flex-start; gap: 0; padding: var(--sp-4) var(--sp-10);
    font-size: var(--fs-md); line-height: 1.25; }
  .cap { font-size: var(--fs-2xs); color: var(--faint); text-transform: uppercase; letter-spacing: .06em; }
  .menu {
    position: absolute; top: calc(100% + 6px); left: 0; z-index: var(--z-dropdown); min-width: 260px; padding: var(--sp-6);
    border: var(--border-width) solid var(--line); border-radius: var(--radius-lg); box-shadow: var(--shadow-pop);
  }
  .menu-h { font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); padding: var(--sp-6) var(--sp-8) var(--sp-2); }
  .item { display: flex; justify-content: space-between; width: 100%; border: none; background: transparent; padding: var(--sp-6) var(--sp-8); text-align: left; gap: var(--sp-12); }
  .item:hover:not(:disabled) { background: var(--hover); }
  div.item:hover { background: transparent; }
  .item.current { font-weight: var(--fw-semibold); color: var(--accent); }
  .sep { height: 1px; background: var(--line); margin: var(--sp-6) 0; }
  .row { display: flex; align-items: center; }
  .milestones { max-height: 220px; overflow: auto; }
  .row .item { flex: 1; min-width: 0; }
  .item.current { align-items: center; }
  .bn { display: inline-flex; align-items: center; gap: var(--sp-6); min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name { display: inline-flex; align-items: center; gap: var(--sp-6); }
  .dot { flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--c); }
  .gear { flex: none; padding: var(--sp-2) var(--sp-6); color: var(--muted); line-height: 1; }
  .gear:hover:not(:disabled) { color: var(--text); }
</style>
