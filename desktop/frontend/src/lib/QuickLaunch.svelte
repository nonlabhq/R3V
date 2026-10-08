<script lang="ts" module>
  import type { TeamProject as P } from "./api";
  // What was picked: a team's home, or one of its projects.
  export type Pick = { team: string; project?: P };
</script>

<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n.svelte";
  import { api, type TeamProject, type TeamSummary } from "./api";
  import ProjectIcon from "./ProjectIcon.svelte";
  import { cssColor, initial, pickFor } from "./palette";

  // Ctrl+T or the + by the tabs: a team's home or a project, of any team,
  // found by its name. Empty, it offers the projects opened last and the
  // teams. Enter opens the one marked; Esc or a click outside closes.
  let { teams, current, recent, onpick, onclose }: {
    teams: TeamSummary[];
    current: string; // the team shown
    recent: { team: string; key: string }[]; // newest first
    onpick: (p: Pick) => void;
    onclose: () => void;
  } = $props();

  // Every team's projects as this computer knows them (no network).
  let all = $state<{ team: string; projects: TeamProject[] }[]>([]);
  let input = $state<HTMLInputElement>();
  onMount(() => {
    input?.focus();
    api.AllProjects().then((x) => (all = x ?? [])).catch(() => {});
  });
  const teamName = (id: string) => teams.find((x) => x.id === id)?.name ?? "";
  const keyOf = (p: TeamProject) => p.root || p.id;

  type Item = { kind: "team"; team: string } | { kind: "project"; team: string; p: TeamProject };
  let query = $state("");
  const matches = (s: string, q: string) => s.toLocaleLowerCase().includes(q);
  let groups = $derived.by((): { title: string; items: Item[] }[] => {
    const q = query.trim().toLocaleLowerCase();
    const projects: Item[] = all.flatMap((x) => x.projects.map((p): Item => ({ kind: "project", team: x.team, p })));
    const teamItems: Item[] = teams.map((x): Item => ({ kind: "team", team: x.id }));
    if (q) {
      return [
        { title: t("Projects"), items: projects.filter((i) => i.kind === "project" && matches(i.p.name, q)).slice(0, 12) },
        { title: t("Teams"), items: teamItems.filter((i) => matches(teamName(i.team), q)) },
      ].filter((g) => g.items.length);
    }
    const byKey = new Map(projects.map((i) => [i.kind === "project" ? `${i.team}/${keyOf(i.p)}` : "", i]));
    const last = recent.map((r) => byKey.get(`${r.team}/${r.key}`)).filter((i): i is Item => !!i).slice(0, 5);
    return [{ title: t("Recent"), items: last }, { title: t("Teams"), items: teamItems }].filter((g) => g.items.length);
  });
  let flat = $derived(groups.flatMap((g) => g.items));
  let at = $state(0);
  $effect(() => { query; at = 0; });

  function pick(i: Item | undefined) {
    if (!i) return;
    onpick(i.kind === "team" ? { team: i.team } : { team: i.team, project: i.p });
  }
  function keydown(e: KeyboardEvent) {
    if (e.key === "Escape") { e.preventDefault(); onclose(); }
    else if (e.key === "ArrowDown") { e.preventDefault(); if (flat.length) at = (at + 1) % flat.length; }
    else if (e.key === "ArrowUp") { e.preventDefault(); if (flat.length) at = (at - 1 + flat.length) % flat.length; }
    else if (e.key === "Enter") { e.preventDefault(); pick(flat[at]); }
  }
  let list = $state<HTMLElement>();
  $effect(() => { list?.querySelector<HTMLElement>(`[data-i="${at}"]`)?.scrollIntoView?.({ block: "nearest" }); });
  const indexOf = (i: Item) => flat.indexOf(i);
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="scrim" onclick={onclose}></div>
<div class="panel" role="dialog" aria-modal="true" aria-label={t("Open a team or project")}>
  <div class="search">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
    <input bind:this={input} bind:value={query} onkeydown={keydown} placeholder={t("Open a project or team…")} aria-label={t("Open a project or team…")}
      role="combobox" aria-expanded="true" aria-controls="ql-list" aria-activedescendant={flat.length ? `ql-${at}` : undefined} spellcheck="false" />
    <kbd>Ctrl T</kbd>
  </div>
  <div class="list" id="ql-list" role="listbox" bind:this={list}>
    {#each groups as g (g.title)}
      <div class="group" role="presentation">{g.title}</div>
      {#each g.items as i (i.kind + i.team + (i.kind === "project" ? keyOf(i.p) : ""))}
        {@const n = indexOf(i)}
        <button class="item" class:on={n === at} id="ql-{n}" data-i={n} role="option" aria-selected={n === at}
          onmouseenter={() => (at = n)} onclick={() => pick(i)}>
          {#if i.kind === "project"}
            <ProjectIcon p={i.p} size={26} />
            <span class="text"><span class="name">{i.p.name}</span><span class="sub">{teamName(i.team)}</span></span>
            {#if i.p.status === "remote"}<span class="hint">☁ {t("On the team")}</span>
            {:else if n === at}<span class="hint">{t("Enter to open")}</span>{/if}
          {:else}
            <span class="mark" style:--c={cssColor(pickFor(i.team))} aria-hidden="true">{initial(teamName(i.team))}</span>
            <span class="text"><span class="name">{teamName(i.team)}</span><span class="sub">{t("Team home")}</span></span>
            {#if i.team === current}<span class="hint">{t("shown")}</span>{:else if n === at}<span class="hint">{t("Enter to open")}</span>{/if}
          {/if}
        </button>
      {/each}
    {:else}
      <p class="none">{t("Nothing matches “{text}”", { text: query.trim() })}</p>
    {/each}
  </div>
  <div class="foot" aria-hidden="true"><span>{t("↑↓ to move")}</span><span>{t("Enter to open")}</span><span>{t("Esc to close")}</span></div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: var(--z-dialog); background: var(--scrim); }
  .panel { position: fixed; top: 48px; left: 50%; transform: translateX(-50%); z-index: var(--z-dialog); width: 560px; max-width: calc(100vw - 32px);
    display: flex; flex-direction: column; max-height: min(560px, calc(100vh - 80px)); padding: var(--sp-10);
    border-radius: var(--radius-xl); background: var(--panel-2); border: var(--border-width) solid var(--line); box-shadow: var(--shadow-pop); }
  .search { display: flex; align-items: center; gap: var(--sp-8); padding: 0 var(--sp-12); border-radius: var(--radius-lg); background: var(--panel); }
  .search svg { width: 16px; height: 16px; flex: none; color: var(--accent); }
  .search input { flex: 1; min-width: 0; border: none; background: transparent; padding: var(--sp-12) 0; font-size: var(--fs-lg); outline: none; box-shadow: none; }
  kbd { flex: none; padding: 0 var(--sp-6); border: var(--border-width) solid var(--line-strong); border-radius: var(--radius); font-size: var(--fs-xs); color: var(--faint); font-family: inherit; }
  .list { flex: 1; min-height: 0; overflow: auto; margin-top: var(--sp-6); }
  .group { padding: var(--sp-10) var(--sp-10) var(--sp-4); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: .06em; color: var(--faint); font-weight: var(--fw-semibold); }
  .item { width: 100%; display: flex; align-items: center; gap: var(--sp-10); padding: var(--sp-8) var(--sp-10); border: none; border-radius: var(--radius-lg);
    background: transparent; text-align: left; }
  .item:hover:not(:disabled) { background: transparent; }
  .item.on, .item.on:hover:not(:disabled) { background: var(--accent-bg); }
  .text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .name { font-weight: var(--fw-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { font-size: var(--fs-sm); color: var(--faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .hint { flex: none; font-size: var(--fs-sm); color: var(--faint); }
  .mark { flex: none; width: 26px; height: 26px; border-radius: var(--radius); display: flex; align-items: center; justify-content: center;
    font-size: var(--fs-sm); font-weight: var(--fw-bold); color: var(--c); background: color-mix(in srgb, var(--c) 20%, var(--panel)); }
  .none { margin: var(--sp-12) var(--sp-10); color: var(--faint); font-size: var(--fs-md); }
  .foot { display: flex; gap: var(--sp-16); padding: var(--sp-10) var(--sp-10) var(--sp-2); margin-top: var(--sp-6);
    border-top: var(--border-width) solid var(--line); font-size: var(--fs-xs); color: var(--faint); }
</style>
