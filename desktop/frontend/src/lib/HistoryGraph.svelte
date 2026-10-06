<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, type Version } from "./api";
  import { layout } from "./graph";

  // The Overview's left column: the versions as a graph, newest at the top,
  // and above them your changes not committed yet (a dashed dot on the
  // version you're on). Picking one shows it on the right; ↑ ↓ move.
  let { versions, head, latest = head, incoming, pending, selected, onselect }: {
    versions: Version[]; head: string; latest?: string; incoming: Set<string>;
    pending: number;   // files changed and not committed (0: no row for them)
    selected: string;  // a version id, or "pending"
    onselect: (id: string) => void;
  } = $props();

  const ROW = 44, LANE = 16, PAD = 14;
  let rows = $derived(layout(versions, [head, latest, ...versions.filter((v) => v.branches.length).map((v) => v.id)].filter(Boolean)));
  const autoMerge = (v: Version) => v.parents.length > 1 && v.message === "Merge versions from the team";
  let width = $derived(PAD * 2 + LANE * Math.max(1, ...rows.map((r) => r.width)));
  // The pending row takes the top: versions start one row down.
  let top = $derived(pending ? ROW : 0);
  const x = (lane: number) => PAD + lane * LANE;
  const mid = (i: number) => top + i * ROW + ROW / 2;
  const segment = (i: number, from: number, to: number) =>
    `M ${x(from)} ${mid(i)} C ${x(from)} ${mid(i) + ROW / 2}, ${x(to)} ${mid(i) + ROW / 2}, ${x(to)} ${mid(i) + ROW}`;
  let headIndex = $derived(versions.findIndex((v) => v.id === head));
  let headLane = $derived(headIndex >= 0 ? rows[headIndex].lane : 0);

  let ids = $derived([...(pending ? ["pending"] : []), ...versions.map((v) => v.id)]);
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const i = ids.indexOf(selected) + (e.key === "ArrowDown" ? 1 : -1);
    if (i >= 0 && i < ids.length) {
      onselect(ids[i]);
      list?.querySelectorAll<HTMLElement>("[data-id]")[i]?.focus();
    }
  }
  let list = $state<HTMLElement>();
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="graph-col" role="listbox" tabindex="-1" aria-label={t("Versions")} bind:this={list} {onkeydown}>
  {#if versions.length === 0 && !pending}
    <p class="muted empty">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
  {:else}
    <svg width={width} height={top + versions.length * ROW} class="lines">
      {#each rows as r, i}
        {#each r.down as s}
          <path d={segment(i, s.from, s.to)} stroke="var(--lane-{s.color})" stroke-width={s.side ? 1.5 : 2} fill="none"
            stroke-dasharray={s.side ? "4 4" : undefined} opacity={s.side ? 0.55 : 1} />
        {/each}
      {/each}
      {#if pending && headIndex >= 0}
        <path d="M {x(headLane)} {ROW / 2 + 8} L {x(headLane)} {mid(headIndex) - 6}" stroke="var(--muted)" stroke-width="1.5"
          stroke-dasharray="3 4" fill="none" />
        <circle cx={x(headLane)} cy={ROW / 2} r="7" fill="var(--bg)" stroke="var(--muted)" stroke-width="1.5" stroke-dasharray="3 2.5" />
        <path d="M {x(headLane) - 3} {ROW / 2} h 6 M {x(headLane)} {ROW / 2 - 3} v 6" stroke="var(--muted)" stroke-width="1.5" />
      {/if}
      {#each rows as r, i}
        {@const v = versions[i]}
        <circle cx={x(r.lane)} cy={mid(i)} r={v.id === selected ? 7 : v.id === head ? 6 : 4.5}
          fill={incoming.has(v.id) ? "var(--bg)" : `var(--lane-${r.color})`}
          stroke="var(--lane-{r.color})" stroke-width="2" />
        {#if v.id === selected}
          <circle cx={x(r.lane)} cy={mid(i)} r="10.5" fill="none" stroke="var(--lane-{r.color})" stroke-width="1.5" opacity=".6" />
        {/if}
      {/each}
    </svg>
    <ul style:padding-left="{width}px">
      {#if pending}
        <li>
          <button class="row pending" class:on={selected === "pending"} style:height="{ROW}px" data-id="pending"
            role="option" aria-selected={selected === "pending"} onclick={() => onselect("pending")}>
            <span class="msg">{t("Your changes")}</span>
            <span class="meta">{tn(pending, "{count} file · not committed yet", "{count} files · not committed yet", { count: pending })}</span>
          </button>
        </li>
      {/if}
      {#each versions as v (v.id)}
        <li>
          <button class="row" class:on={selected === v.id} class:incoming={incoming.has(v.id)} class:automerge={autoMerge(v)}
            style:height="{ROW}px" data-id={v.id} role="option" aria-selected={selected === v.id}
            aria-label={`${v.message || t("(no description)")}, ${v.author}, ${ago(v.time)}`} onclick={() => onselect(v.id)}>
            <span class="msg">
              {autoMerge(v) ? t("Combined with the team's versions") : v.message || t("(no description)")}
            </span>
            <span class="meta">
              {v.author} · {ago(v.time)}
              {#each v.branches as b}<span class="tag">{b}</span>{/each}
              {#if v.id === head}<span class="tag here">{t("you are here")}</span>{/if}
              {#if incoming.has(v.id)}<span class="tag new">{t("new")}</span>{/if}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .graph-col { position: relative; height: 100%; overflow: auto; outline: none; }
  .lines { position: absolute; left: 0; top: 0; overflow: visible; pointer-events: none; }
  .empty { padding: var(--sp-16); }
  ul { list-style: none; margin: 0; padding-right: var(--sp-8); }
  .row { display: flex; flex-direction: column; justify-content: center; align-items: flex-start; gap: 1px; width: 100%;
    border: none; border-radius: var(--radius); background: transparent; padding: 0 var(--sp-8); text-align: left; min-width: 0; }
  .row:hover:not(.on) { background: var(--panel); }
  .row.on { background: var(--panel-2); }
  .row:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .msg, .meta { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .msg { font-size: var(--fs-md); }
  .meta { font-size: var(--fs-xs); color: var(--faint); }
  .pending .msg { color: var(--accent); font-weight: var(--fw-semibold); }
  .incoming .msg { color: var(--muted); }
  .automerge .msg { color: var(--faint); font-style: italic; }
  .tag { display: inline-block; margin-left: var(--sp-6); padding: 0 var(--sp-6); border-radius: var(--radius-pill);
    background: var(--info-chip); color: var(--info-text); }
  .tag.here { background: var(--accent-bg); color: var(--accent); }
  .tag.new { background: var(--warn-bg); color: var(--warn); }
</style>
