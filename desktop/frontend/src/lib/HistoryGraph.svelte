<script lang="ts">
  import { t } from "./i18n.svelte";
  import { ago, type Version } from "./api";
  import { branchGraph, short } from "./branchGraph";

  // The Overview's left column: the branch graph (see branchGraph.ts), main
  // in the middle. Each branch's newest version is labelled with the branch
  // and its title; every other version is a dot with its author's initial
  // (its title on hover). Your changes not committed yet are a dashed dot
  // above the version you're on. Picking one shows it on the right; ↑ ↓ move.
  let { versions, branches, branch, head, incoming, pending, selected, onselect }: {
    versions: Version[];
    branches: { name: string; latest: string }[];
    branch: string;    // the branch you are on
    head: string;      // the version you are on
    incoming: Set<string>;
    pending: number;   // files changed and not committed (0: no dot for them)
    selected: string;  // a version id, or "pending"
    onselect: (id: string) => void;
  } = $props();

  const ROW = 40, COL = 64, PAD = 24, LABEL_W = 180, LABEL_H = 36, TITLE = 22;
  let g = $derived(branchGraph(versions, branches, branch, "main", head));
  let byID = $derived(new Map(versions.map((v) => [v.id, v])));
  let off = $derived(pending ? 1 : 0); // the pending dot takes the first row
  let headChain = $derived(g.chainOf.get(head));

  let width = $state(0); // of the column
  let center = $derived(Math.max(width / 2, PAD + LABEL_W / 2 + g.left * COL));
  let full = $derived(Math.max(width, center + g.right * COL + PAD + LABEL_W / 2));
  const x = (col: number) => center + col * COL;

  // Labels: one per branch, with its newest version's title, above its top
  // dot (your changes, on your branch); moved up when two would meet.
  type Label = { id: string; name: string; title: string; col: number; color: number; y: number; w: number; dim: boolean };
  let labels = $derived.by(() => {
    const out: Label[] = [];
    for (const c of g.chains) {
      if (!c.name) continue;
      const atPending = pending && c === headChain;
      const tip = byID.get(c.tip)!;
      const title = short(tip.message || t("(no description)"), TITLE);
      const rowY = atPending ? 0 : g.row.get(c.tip)! + off;
      const w = Math.min(LABEL_W, Math.max(c.name.length * 6.5, title.length * 7.5) + 20);
      let y = rowY * ROW - LABEL_H + 6; // bottom just above the dot
      const meets = (l: Label) => Math.abs(x(l.col) - x(c.col)) < (l.w + w) / 2 + 4 && Math.abs(l.y - y) < LABEL_H;
      while (out.some(meets)) y -= LABEL_H;
      out.push({ id: c.tip, name: c.name, title, col: c.col, color: c.color, y, w, dim: false });
    }
    return out;
  });
  // Room at the top for the labels.
  let top = $derived(Math.max(LABEL_H, 8 - Math.min(0, ...labels.map((l) => l.y))));
  const yRow = (r: number) => top + r * ROW + ROW / 2;
  const y = (id: string) => yRow(g.row.get(id)! + off);
  let height = $derived(yRow(versions.length + off - 1) + ROW);

  function path(e: { from: string; to: string; kind: string }) {
    const a = g.chainOf.get(e.from)!, b = g.chainOf.get(e.to)!;
    const xa = x(a.col), ya = y(e.from), xb = x(b.col), yb = y(e.to);
    if (e.kind === "line") return `M ${xa} ${ya} L ${xb} ${yb}`;
    if (e.kind === "fork") { // down its own column, then over to where it split
      const turn = Math.max(ya, yb - ROW);
      return `M ${xa} ${ya} L ${xa} ${turn} C ${xa} ${turn + ROW / 2}, ${xb} ${turn + ROW / 2}, ${xb} ${yb}`;
    }
    // merge: over to the branch merged in, then down its column
    const turn = Math.min(yb, ya + ROW);
    return `M ${xa} ${ya} C ${xa} ${ya + ROW / 2}, ${xb} ${ya + ROW / 2}, ${xb} ${turn} L ${xb} ${yb}`;
  }
  const lineChain = (e: { from: string; to: string; kind: string }) =>
    e.kind === "merge" ? g.chainOf.get(e.to)! : g.chainOf.get(e.from)!;

  const initial = (name: string) => ([...name.trim()][0] ?? "?").toUpperCase();
  const tip = (v: Version) => `${v.message || t("(no description)")}\n${v.author} · ${ago(v.time)}`;

  let ids = $derived([...(pending ? ["pending"] : []), ...versions.map((v) => v.id)]);
  let box = $state<HTMLElement>();
  function onkeydown(e: KeyboardEvent) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const i = ids.indexOf(selected) + (e.key === "ArrowDown" ? 1 : -1);
    if (i < 0 || i >= ids.length) return;
    onselect(ids[i]);
    box?.querySelector<HTMLElement>(`[data-id="${ids[i]}"]`)?.focus();
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="graph-col" role="listbox" tabindex="-1" aria-label={t("Versions")} bind:this={box} bind:clientWidth={width} {onkeydown}>
  {#if versions.length === 0 && !pending}
    <p class="muted empty">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
  {:else}
    <div class="canvas" style:width="{full}px" style:height="{height}px">
      <svg width={full} height={height} aria-hidden="true">
        {#each g.edges as e (e.from + ">" + e.to)}
          {@const c = lineChain(e)}
          <path d={path(e)} stroke="var(--lane-{c.color})" stroke-width={c.main ? 3 : 2} fill="none" opacity={c.name ? 1 : 0.55} />
        {/each}
        {#if pending && headChain}
          <path d="M {x(headChain.col)} {yRow(0)} L {x(headChain.col)} {y(head)}" stroke="var(--muted)" stroke-width="1.5"
            stroke-dasharray="3 4" fill="none" />
        {/if}
      </svg>

      {#each labels as l (l.id)}
        <button class="label" style:left="{x(l.col)}px" style:top="{top + l.y}px" style:width="{l.w}px" style:--c="var(--lane-{l.color})"
          tabindex="-1" onclick={() => onselect(l.id)}>
          <span class="bname">{l.name}</span>
          <span class="btitle">{l.title}</span>
        </button>
      {/each}

      {#if pending && headChain}
        <button class="node pending" class:on={selected === "pending"} data-id="pending" role="option"
          aria-selected={selected === "pending"} style:left="{x(headChain.col)}px" style:top="{yRow(0)}px"
          style:--c="var(--lane-{headChain.color})" title={t("Your changes")} aria-label={t("Your changes")}
          onclick={() => onselect("pending")}>+</button>
      {/if}
      {#each versions as v (v.id)}
        {@const c = g.chainOf.get(v.id)!}
        <button class="node" class:on={selected === v.id} class:here={v.id === head} class:incoming={incoming.has(v.id)}
          class:side={!c.name} data-id={v.id} role="option" aria-selected={selected === v.id}
          style:left="{x(c.col)}px" style:top="{y(v.id)}px" style:--c="var(--lane-{c.color})"
          title={tip(v)} aria-label={`${v.message || t("(no description)")}, ${v.author}, ${ago(v.time)}`}
          onclick={() => onselect(v.id)}>{initial(v.author)}</button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .graph-col { height: 100%; overflow: auto; outline: none; }
  .canvas { position: relative; }
  svg { position: absolute; left: 0; top: 0; }
  .empty { padding: var(--sp-16); }
  .node { position: absolute; width: 24px; height: 24px; margin: -12px 0 0 -12px; padding: 0; border-radius: 50%;
    border: 2px solid var(--c); background: var(--panel-2); color: var(--text); font-size: var(--fs-xs);
    font-weight: var(--fw-semibold); line-height: 20px; text-align: center; }
  .node:hover:not(:disabled) { border-color: var(--c); background: var(--hover); }
  .node.here { background: var(--c); color: var(--bg); }
  .node.incoming { border-style: dashed; color: var(--muted); }
  .node.side { opacity: .75; }
  .node.pending { border-style: dashed; border-color: var(--muted); color: var(--muted); background: var(--bg); font-size: var(--fs-md); }
  .node.on { box-shadow: 0 0 0 3px var(--bg), 0 0 0 5px var(--c); }
  .node.pending.on { box-shadow: 0 0 0 3px var(--bg), 0 0 0 5px var(--muted); }
  .node:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
  .label { position: absolute; transform: translateX(-50%); height: 32px; padding: var(--sp-2) var(--sp-8);
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0;
    border: var(--border-width) solid var(--line); border-radius: var(--radius); background: var(--panel); line-height: 1.2; }
  .label:hover:not(:disabled) { border-color: var(--c); background: var(--panel); }
  .bname { font-size: var(--fs-2xs); font-weight: var(--fw-semibold); color: var(--c); max-width: 100%;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .btitle { font-size: var(--fs-xs); color: var(--text); max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
