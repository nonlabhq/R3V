<script lang="ts">
  import { t } from "./i18n.svelte";
  import { ago, type Version } from "./api";
  import { branchGraph, short } from "./branchGraph";
  import type { Snippet } from "svelte";

  // The Overview's left column: the branch graph (see branchGraph.ts), main
  // in the middle. Each branch's newest version is labelled with the branch
  // and its title; every other version is a dot with its author's initial
  // (its title on hover). Your changes not committed yet are a dashed dot
  // above the version you're on. Picking one shows it on the right; ↑ ↓ move.
  // The view moves: drag to pan, scroll to go up and down, Ctrl+scroll to
  // zoom, double-click the background to put it back.
  let { versions, branches, branch, head, incoming, pending, selected, onselect, actions }: {
    versions: Version[];
    branches: { name: string; latest: string }[];
    branch: string;    // the branch you are on
    head: string;      // the version you are on
    incoming: Set<string>;
    pending: number;   // files changed and not committed (0: no dot for them)
    selected: string;  // a version id, or "pending"
    onselect: (id: string) => void;
    actions?: Snippet<[Version]>; // the hover card's buttons for a version
  } = $props();

  const ROW = 40, COL = 64, PAD = 24, LABEL_W = 180, LABEL_H = 36, TITLE = 22;
  let g = $derived(branchGraph(versions, branches, branch, "main", head));
  let byID = $derived(new Map(versions.map((v) => [v.id, v])));
  let off = $derived(pending ? 1 : 0); // the pending dot takes the first row
  let headChain = $derived(g.chainOf.get(head));

  let width = $state(0); // of the column
  let boxHeight = $state(0);

  // Labels: one per branch, with its newest version's title, above its top
  // dot (your changes, on your branch). Placed clear of the dots and of each
  // other: centred, else leaning to the branch's side, else a row higher.
  // Positions are from the middle column (dx) and the first row (y).
  type Label = { id: string; name: string; title: string; color: number; dx: number; y: number; w: number; h: number };
  let labels = $derived.by(() => {
    const out: Label[] = [];
    const dots = [...versions.map((v) => ({ col: g.chainOf.get(v.id)!.col, row: g.row.get(v.id)! + off })),
      ...(pending && headChain ? [{ col: headChain.col, row: 0 }] : [])];
    const clear = (dx: number, y: number, w: number, h: number) =>
      dots.every((d) => d.col * COL + 12 < dx || d.col * COL - 12 > dx + w || d.row * ROW + ROW / 2 + 12 < y || d.row * ROW + ROW / 2 - 12 > y + h) &&
      out.every((l) => l.dx + l.w + 4 < dx || l.dx > dx + w + 4 || l.y + l.h + 4 < y || l.y > y + h + 4);
    for (const c of g.chains) {
      if (!c.name) continue;
      const atPending = pending && c === headChain;
      const tip = byID.get(c.tip)!;
      // On your branch with changes not committed: just its name.
      const title = atPending ? "" : short(tip.message || t("(no description)"), TITLE);
      const rowY = atPending ? 0 : g.row.get(c.tip)! + off;
      const w = Math.min(LABEL_W, Math.max(c.name.length * 6.5, title.length * 7.5) + 20);
      const h = title ? 32 : 24;
      const at = c.col * COL;
      const lean = c.col > 0 ? [at - w / 2, at - 14, at + 14 - w] : [at - w / 2, at + 14 - w, at - 14];
      let place = { dx: lean[0], y: rowY * ROW + ROW / 2 - 16 - h };
      search: for (let k = 0; k < 30; k++) {
        const y = rowY * ROW + ROW / 2 - 16 - h - k * (LABEL_H - 4);
        for (const dx of lean) if (clear(dx, y, w, h)) { place = { dx, y }; break search; }
      }
      out.push({ id: c.tip, name: c.name, title, color: c.color, ...place, w, h });
    }
    return out;
  });
  let center = $derived(PAD - Math.min(-g.left * COL - 12, ...labels.map((l) => l.dx)));
  let full = $derived(center + Math.max(g.right * COL + 12, ...labels.map((l) => l.dx + l.w)) + PAD);
  const x = (col: number) => center + col * COL;
  // Room at the top for the labels.
  let top = $derived(Math.max(LABEL_H, -Math.min(0, ...labels.map((l) => l.y))) + 20);
  const yRow = (r: number) => top + r * ROW + ROW / 2;
  const y = (id: string) => yRow(g.row.get(id)! + off);
  let height = $derived(yRow(versions.length + off - 1) + ROW / 2 + 16);

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

  // The card for the version under the pointer: what it is, and what can be
  // done with it. It stays while the pointer is on the dot or the card.
  const CARD_W = 280;
  let hovered = $state<string | null>(null);
  let hideTimer: ReturnType<typeof setTimeout> | undefined;
  function hover(id: string) { clearTimeout(hideTimer); hovered = id; }
  function unhover() { clearTimeout(hideTimer); hideTimer = setTimeout(() => (hovered = null), 180); }
  let card = $derived.by(() => {
    const v = hovered ? byID.get(hovered) : undefined;
    if (!v) return null;
    const c = g.chainOf.get(v.id)!;
    const nx = panX + x(c.col) * zoom, ny = panY + y(v.id) * zoom, r = 12 * zoom + 10;
    const right = nx + r + CARD_W <= width - 4;
    return { v, branch: c.name, left: right ? nx + r : Math.max(4, nx - r - CARD_W), top: Math.min(Math.max(4, ny - 28), Math.max(4, boxHeight - 150)) };
  });

  // The view: pan (px on the screen) and zoom.
  let panX = $state(0), panY = $state(0), zoom = $state(1);
  const ZOOM_MIN = 0.4, ZOOM_MAX = 2.5;
  // At first (and on double-click): main in the middle, a short history in
  // the middle too, a long one from the top.
  function resetView() {
    zoom = 1;
    panX = width / 2 - center;
    panY = Math.max(0, (boxHeight - height) / 2);
  }
  let viewed = false;
  $effect(() => {
    if (!viewed && width && boxHeight && (versions.length || pending)) { resetView(); viewed = true; }
  });
  // Some of the graph always stays in sight.
  function clamp() {
    const w = full * zoom, h = height * zoom, mx = Math.min(80, width / 3), my = Math.min(80, boxHeight / 3);
    panX = Math.min(Math.max(panX, mx - w), width - mx);
    panY = Math.min(Math.max(panY, my - h), boxHeight - my);
  }

  // Drag to pan (from anywhere but the card); a drag isn't a click.
  let drag: { x: number; y: number; px: number; py: number; moved: boolean; id: number } | null = null;
  let dragging = $state(false);
  function onpointerdown(e: PointerEvent) {
    if (e.button !== 0 || (e.target as HTMLElement).closest(".card")) return;
    drag = { x: e.clientX, y: e.clientY, px: panX, py: panY, moved: false, id: e.pointerId };
  }
  function onpointermove(e: PointerEvent) {
    if (!drag) return;
    const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
    if (!drag.moved && Math.hypot(dx, dy) < 4) return;
    if (!drag.moved) { drag.moved = true; dragging = true; hovered = null; box?.setPointerCapture(drag.id); }
    panX = drag.px + dx;
    panY = drag.py + dy;
    clamp();
  }
  function onpointerup() {
    if (drag?.moved) setTimeout(() => (dragging = false));
    drag = null;
  }
  function onclickcapture(e: MouseEvent) {
    if (dragging) { e.stopPropagation(); e.preventDefault(); dragging = false; }
  }
  function ondblclick(e: MouseEvent) {
    if (!(e.target as HTMLElement).closest("button, .card")) resetView();
  }
  // Scroll: up and down (sideways with a trackpad or Shift); Ctrl+scroll
  // zooms around the pointer. (Not passive: the page mustn't scroll.)
  $effect(() => {
    const el = box;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      hovered = null;
      if (e.ctrlKey) {
        const r = el.getBoundingClientRect(), mx = e.clientX - r.left, my = e.clientY - r.top;
        const z = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom * Math.exp(-e.deltaY * 0.0015)));
        panX = mx - ((mx - panX) / zoom) * z;
        panY = my - ((my - panY) / zoom) * z;
        zoom = z;
      } else {
        panX -= e.deltaX;
        panY -= e.deltaY;
      }
      clamp();
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  });
  // Keep the picked version in sight (↑ ↓).
  function reveal(id: string) {
    const yy = panY + (id === "pending" ? yRow(0) : y(id)) * zoom;
    if (yy < 40) panY += 40 - yy;
    else if (yy > boxHeight - 40) panY -= yy - (boxHeight - 40);
  }

  let ids = $derived([...(pending ? ["pending"] : []), ...versions.map((v) => v.id)]);
  let box = $state<HTMLElement>();
  function onkeydown(e: KeyboardEvent) {
    if (e.key === "Escape") { hovered = null; return; }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const i = ids.indexOf(selected) + (e.key === "ArrowDown" ? 1 : -1);
    if (i < 0 || i >= ids.length) return;
    onselect(ids[i]);
    reveal(ids[i]);
    box?.querySelector<HTMLElement>(`[data-id="${ids[i]}"]`)?.focus({ preventScroll: true });
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="graph-col" role="listbox" tabindex="-1" aria-label={t("Versions")} bind:this={box} bind:clientWidth={width} bind:clientHeight={boxHeight} {onkeydown}
  {onpointerdown} {onpointermove} {onpointerup} onpointercancel={onpointerup} {ondblclick} {onclickcapture}
  class:dragging>
  {#if versions.length === 0 && !pending}
    <p class="muted empty">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
  {:else}
    <div class="canvas" style:width="{full}px" style:height="{height}px" style:transform="translate({panX}px, {panY}px) scale({zoom})">
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
        <button class="label" style:left="{center + l.dx}px" style:top="{top + l.y}px" style:width="{l.w}px" style:height="{l.h}px" style:--c="var(--lane-{l.color})"
          tabindex="-1" onclick={() => onselect(l.id)}>
          <span class="bname">{l.name}</span>
          {#if l.title}<span class="btitle">{l.title}</span>{/if}
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
          aria-label={`${v.message || t("(no description)")}, ${v.author}, ${ago(v.time)}`}
          onmouseenter={() => hover(v.id)} onmouseleave={unhover} onfocus={() => hover(v.id)}
          onclick={() => onselect(v.id)}>{initial(v.author)}</button>
      {/each}

    </div>
    {#if card}
      {@const v = card.v}
      <div class="card surface-menu" role="group" aria-label={v.message || t("(no description)")}
        style:left="{card.left}px" style:top="{card.top}px" style:width="{CARD_W}px"
        onmouseenter={() => hover(v.id)} onmouseleave={unhover}>
        <div class="card-h">
          <span class="avatar" style:--c="var(--lane-{g.chainOf.get(v.id)!.color})">{initial(v.author)}</span>
          <div class="card-t">
            <div class="card-msg">{v.message || t("(no description)")}</div>
            <div class="card-meta">{v.author} · {ago(v.time)}{#if card.branch} · {card.branch}{/if} · <span class="mono">{v.short}</span></div>
          </div>
        </div>
        {#if actions}<div class="card-acts">{@render actions(v)}</div>{/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .graph-col { position: relative; height: 100%; overflow: hidden; outline: none; cursor: grab; touch-action: none; }
  .graph-col.dragging { cursor: grabbing; }
  .graph-col.dragging * { cursor: grabbing; }
  .canvas { position: absolute; left: 0; top: 0; transform-origin: 0 0; }
  svg { position: absolute; left: 0; top: 0; }
  .empty { padding: var(--sp-16); }
  .node { position: absolute; width: 24px; height: 24px; margin: -12px 0 0 -12px; padding: 0; border-radius: 50%;
    border: 2px solid var(--c); background: var(--panel-2); color: var(--text); font-size: var(--fs-xs);
    font-weight: var(--fw-semibold); line-height: 20px; text-align: center; }
  .node:hover:not(:disabled) { border-color: var(--c); background: var(--hover); }
  .node.here { background: var(--c); color: var(--bg); }
  .node.incoming { border-style: dashed; color: var(--muted); }
  .node.side { opacity: .75; }
  .node.pending { border-style: dashed; border-color: var(--muted); color: var(--muted); background: var(--bg); }
  .node.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .node.pending.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--muted); }
  .node:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
  .card { position: absolute; z-index: 2; padding: var(--sp-10) var(--sp-12); border: var(--border-width) solid var(--line);
    border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .card-h { display: flex; gap: var(--sp-10); align-items: flex-start; }
  .avatar { flex: none; width: 24px; height: 24px; border-radius: 50%; border: 2px solid var(--c); display: flex;
    align-items: center; justify-content: center; font-size: var(--fs-xs); font-weight: var(--fw-semibold); }
  .card-t { min-width: 0; }
  .card-msg { font-size: var(--fs-md); font-weight: var(--fw-semibold); display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3;
    -webkit-box-orient: vertical; overflow: hidden; user-select: text; }
  .card-meta { font-size: var(--fs-xs); color: var(--faint); margin-top: var(--sp-2); }
  .card-acts { display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-6); margin-top: var(--sp-10); }
  .card-acts :global(button) { padding: var(--sp-4) var(--sp-8); font-size: var(--fs-sm); }
  .label { position: absolute; height: auto; padding: var(--sp-2) var(--sp-8);
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0;
    border: var(--border-width) solid var(--line); border-radius: var(--radius); background: var(--panel); line-height: 1.2; }
  .label:hover:not(:disabled) { border-color: var(--c); background: var(--panel); }
  .bname { font-size: var(--fs-2xs); font-weight: var(--fw-semibold); color: var(--c); max-width: 100%;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .btitle { font-size: var(--fs-xs); color: var(--text); max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
