<script lang="ts">
  import { t } from "./i18n.svelte";
  import { ago, type Version } from "./api";
  import { branchGraph, short } from "./branchGraph";
  import { portal } from "./portal";
  import type { Snippet } from "svelte";

  // The Overview's left column: the branch graph (see branchGraph.ts), main
  // in the middle. Each branch's newest version is labelled with the branch
  // and its title; every other version is a dot with its author's initial
  // (its title on hover). Your changes not committed yet are a dashed dot
  // above the version you're on. Picking one shows it on the right; ↑ ↓ move.
  // The view moves: drag to pan, scroll to go up and down, Ctrl+scroll to
  // zoom, double-click the background to put it back.
  let { versions, branches, branch, head, incoming, pending, selected, onselect, actions, reserve = 0, panelInset = 0 }: {
    versions: Version[];
    branches: { name: string; latest: string }[];
    branch: string;    // the branch you are on
    head: string;      // the version you are on
    incoming: Set<string>;
    pending: number;   // files changed and not committed (0: no dot for them)
    selected: string;  // a version id, or "pending"
    onselect: (id: string) => void;
    actions?: Snippet<[Version]>; // the hover card's buttons for a version
    reserve?: number;    // px on the right covered by the details (the graph centres in the rest)
    panelInset?: number; // the details' distance from the top and bottom
  } = $props();

  // A row per version, a column per line of work (at 100%). Zoom spreads
  // them out or packs them in; the dots, lines and labels keep their size.
  const ROW = 56, COL = 64, PAD = 24, LABEL_W = 180, LABEL_H = 36, TITLE = 22;
  // The view: pan (px on the screen) and zoom.
  let panX = $state(0), panY = $state(0), zoom = $state(1);
  const ZOOM_MIN = 0.5, ZOOM_MAX = 2.5;
  let rowH = $derived(ROW * zoom), colW = $derived(COL * zoom);
  let g = $derived(branchGraph(versions, branches, branch, "main", head));
  let byID = $derived(new Map(versions.map((v) => [v.id, v])));
  let off = $derived(pending ? 1 : 0); // the pending dot takes the first row
  // Where your changes go: the branch you are on (its own column even when it
  // has no versions yet), else the line of the version you are on.
  let headChain = $derived(g.chains.find((c) => c.empty && c.name === branch) ?? g.chainOf.get(head));
  // Where your changes' dot goes: on that line, or in the middle before the
  // first version (a project just added: everything in it is your changes).
  let pendAt = $derived(headChain ? { col: headChain.col, color: headChain.color } : { col: 0, color: 0 });
  // Branches with no versions yet, other than yours with changes: a short
  // line out of where they start, ending in a hollow dot (row: a little above).
  let stubs = $derived(g.chains.filter((c) => c.empty && !(pending && c === headChain))
    .map((c) => ({ c, from: g.chainOf.get(c.tip)!, row: g.row.get(c.tip)! + off - 0.8 })));
  // Branches merged into another: quieter.
  let merged = $derived(new Set(g.edges.filter((e) => e.kind === "merge").map((e) => g.chainOf.get(e.to)!)));

  let fullWidth = $state(0); // of the column
  // What can be seen: the column less the part under the details.
  let width = $derived(Math.max(120, fullWidth - reserve));
  let boxHeight = $state(0);

  // Labels: one per branch, with its newest version's title, above its top
  // dot (your changes, on your branch). Placed clear of the dots and of each
  // other: centred, else leaning to the branch's side, else a row higher.
  // Positions are from the middle column (dx) and the first row (y).
  type Label = { id: string; name: string; title: string; color: number; dx: number; y: number; w: number; h: number };
  let labels = $derived.by(() => {
    const out: Label[] = [];
    const dots = [...versions.map((v) => ({ col: g.chainOf.get(v.id)!.col, row: g.row.get(v.id)! + off })),
      ...(pending ? [{ col: pendAt.col, row: 0 }] : []), ...stubs.map((st) => ({ col: st.c.col, row: st.row }))];
    const clear = (dx: number, y: number, w: number, h: number) =>
      dots.every((d) => d.col * colW + 12 < dx || d.col * colW - 12 > dx + w || d.row * rowH + rowH / 2 + 12 < y || d.row * rowH + rowH / 2 - 12 > y + h) &&
      out.every((l) => l.dx + l.w + 4 < dx || l.dx > dx + w + 4 || l.y + l.h + 4 < y || l.y > y + h + 4);
    // (before the first version: your branch's name over your changes)
    const first = pending && !headChain ? [{ name: branch, tip: "", col: 0, color: 0, empty: true }] : [];
    for (const c of [...g.chains, ...first]) {
      if (!c.name) continue;
      const atPending = pending && (c === headChain || !c.tip);
      const tip = byID.get(c.tip);
      // On your branch with changes not committed: just its name.
      // (an empty branch: just its name too; its newest version is another's)
      const title = atPending || c.empty || !tip ? "" : short(tip.message || t("(no description)"), TITLE);
      const rowY = atPending ? 0 : c.empty ? g.row.get(c.tip)! + off - 0.8 : g.row.get(c.tip)! + off;
      const w = Math.min(LABEL_W, Math.max(c.name.length * 6.5, title.length * 7.5) + 20);
      const h = title ? 32 : 24;
      const at = c.col * colW;
      const lean = c.col > 0 ? [at - w / 2, at - 14, at + 14 - w] : [at - w / 2, at + 14 - w, at - 14];
      let place = { dx: lean[0], y: rowY * rowH + rowH / 2 - 16 - h };
      search: for (let k = 0; k < 30; k++) {
        const y = rowY * rowH + rowH / 2 - 16 - h - k * (LABEL_H - 4);
        for (const dx of lean) if (clear(dx, y, w, h)) { place = { dx, y }; break search; }
      }
      out.push({ id: c.tip || "pending", name: c.name, title, color: c.color, ...place, w, h });
    }
    return out;
  });
  let center = $derived(PAD - Math.min(-g.left * colW - 12, ...labels.map((l) => l.dx)));
  let full = $derived(center + Math.max(g.right * colW + 12, ...labels.map((l) => l.dx + l.w)) + PAD);
  const x = (col: number) => center + col * colW;
  // Room at the top for the labels.
  let top = $derived(Math.max(LABEL_H, -Math.min(0, ...labels.map((l) => l.y))) + 20);
  const yRow = (r: number) => top + r * rowH + rowH / 2;
  const y = (id: string) => yRow(g.row.get(id)! + off);
  let height = $derived(yRow(versions.length + off - 1) + rowH / 2 + 16);

  function path(e: { from: string; to: string; kind: string }) {
    const a = g.chainOf.get(e.from)!, b = g.chainOf.get(e.to)!;
    const xa = x(a.col), ya = y(e.from), xb = x(b.col), yb = y(e.to);
    if (e.kind === "line") return `M ${xa} ${ya} L ${xb} ${yb}`;
    if (e.kind === "fork") { // down its own column, then over to where it split
      const turn = Math.max(ya, yb - rowH);
      return `M ${xa} ${ya} L ${xa} ${turn} C ${xa} ${turn + rowH / 2}, ${xb} ${turn + rowH / 2}, ${xb} ${yb}`;
    }
    // merge: over to the branch merged in, then down its column
    const turn = Math.min(yb, ya + rowH);
    return `M ${xa} ${ya} C ${xa} ${ya + rowH / 2}, ${xb} ${ya + rowH / 2}, ${xb} ${turn} L ${xb} ${yb}`;
  }
  // Your changes down to the version you are on (over to it when your
  // branch has no versions yet).
  function pendingPath() {
    const c = headChain!;
    const hx = x(g.chainOf.get(head)?.col ?? c.col), px = x(c.col), turn = Math.max(yRow(0), y(head) - rowH);
    return hx === px ? `M ${px} ${yRow(0)} L ${px} ${y(head)}`
      : `M ${px} ${yRow(0)} L ${px} ${turn} C ${px} ${turn + rowH / 2}, ${hx} ${turn + rowH / 2}, ${hx} ${y(head)}`;
  }
  function stubPath(st: { c: { tip: string; col: number }; from: { col: number }; row: number }) {
    const fx = x(st.from.col), fy = y(st.c.tip), sx = x(st.c.col), sy = yRow(st.row);
    return `M ${fx} ${fy} C ${fx} ${fy - rowH * 0.5}, ${sx} ${sy + rowH * 0.4}, ${sx} ${sy}`;
  }
  const lineChain = (e: { from: string; to: string; kind: string }) =>
    e.kind === "merge" ? g.chainOf.get(e.to)! : g.chainOf.get(e.from)!;

  const initial = (name: string) => ([...name.trim()][0] ?? "?").toUpperCase();

  // The card for the version under the pointer: what it is, and what can be
  // done with it. It stays while the pointer is on the dot or the card.
  const CARD_W = 280;
  let hovered = $state<string | null>(null);
  let cardHeight = $state(140);
  let hideTimer: ReturnType<typeof setTimeout> | undefined;
  // Where the graph is in the window when a card opens (the card is put
  // over everything, the sidebar included: see portal).
  let origin = $state({ left: 0, top: 0 });
  function hover(id: string) {
    clearTimeout(hideTimer);
    const r = box?.getBoundingClientRect();
    if (r) origin = { left: r.left, top: r.top };
    hovered = id;
  }
  function unhover() { clearTimeout(hideTimer); hideTimer = setTimeout(() => (hovered = null), 180); }
  let card = $derived.by(() => {
    const v = hovered ? byID.get(hovered) : undefined;
    if (!v) return null;
    const c = g.chainOf.get(v.id)!;
    // To the left of the dot, centred on it: the pointer can go up and down
    // the versions without the card in the way. The little arrow points at
    // the dot. (In the window: over the sidebar if it comes to that.)
    const nx = origin.left + panX + x(c.col), ny = origin.top + panY + y(v.id), gap = 22;
    const vh = typeof window === "undefined" ? 800 : window.innerHeight;
    const left = Math.max(8, nx - gap - CARD_W);
    const top = Math.min(Math.max(8, ny - cardHeight / 2), Math.max(8, vh - 8 - cardHeight));
    return { v, branch: c.name, left, top, arrow: Math.min(Math.max(14, ny - top), cardHeight - 14) };
  });

  // At first (and on double-click): main in the middle, a short history in
  // the middle too, a long one from the top.
  function resetView() {
    zoom = 1;
    // main as near the middle as the labels allow
    const lo = Math.min(0, width - full), hi = Math.max(0, width - full);
    panX = Math.min(Math.max(width / 2 - center, lo), hi);
    panY = Math.max(0, (boxHeight - height) / 2);
  }
  // Until you move it, the view follows the column's size (and the graph's).
  let moved = false;
  $effect(() => {
    width; boxHeight; center; height;
    if (!moved && width && boxHeight && (versions.length || pending)) resetView();
  });
  // Some of the graph always stays in sight.
  let toolbarWidth = $state(0); // (kept inside the card when the graph is narrow)
  // The toolbar's zoom: around the middle of what can be seen.
  function zoomBy(f: number) {
    const z = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom * f));
    const mx = width / 2, my = boxHeight / 2;
    panX = mx - ((mx - panX) / zoom) * z;
    panY = my - ((my - panY) / zoom) * z;
    zoom = z;
    moved = true;
    clamp();
  }
  function zoomReset() {
    if (zoom === 1) { moved = false; resetView(); return; }
    zoomBy(1 / zoom);
  }
  // The crosshair: back to 100% and the first view, with your changes (or
  // the version you are on) in sight; what is picked stays picked.
  function locate() {
    moved = false;
    resetView();
    const id = pending ? "pending" : head;
    const c = id === "pending" ? pendAt : g.chainOf.get(id);
    if (!id || !c) return;
    const nx = panX + x(c.col), ny = panY + (id === "pending" ? yRow(0) : y(id));
    if (nx < 40 || nx > width - 40) { panX = width / 2 - x(c.col); moved = true; }
    if (ny < 40 || ny > boxHeight - 40) { panY = boxHeight / 3 - (ny - panY); moved = true; }
    clamp();
  }
  // Back to what matters now: your changes when you have some, else the
  // version you are on (the toolbar offers it when something else is picked).
  let backTo = $derived(pending ? (selected !== "pending" ? "pending" : "") : head && selected !== head ? head : "");
  function goBack() {
    onselect(backTo);
    reveal(backTo);
  }

  function clamp() {
    const w = full, h = height, mx = Math.min(80, width / 3), my = Math.min(80, boxHeight / 3);
    panX = Math.min(Math.max(panX, mx - w), width - mx);
    panY = Math.min(Math.max(panY, my - h), boxHeight - my);
  }

  // Drag to pan (from anywhere but the card); a drag isn't a click.
  let drag: { x: number; y: number; px: number; py: number; moved: boolean; id: number } | null = null;
  let dragging = $state(false);
  function onpointerdown(e: PointerEvent) {
    if (e.button !== 0 || (e.target as HTMLElement).closest(".card, .toolbar, .zoombar")) return;
    drag = { x: e.clientX, y: e.clientY, px: panX, py: panY, moved: false, id: e.pointerId };
  }
  function onpointermove(e: PointerEvent) {
    if (!drag) return;
    // The button let go outside (where its release wasn't seen): no drag.
    if (!(e.buttons & 1)) { onpointerup(); return; }
    const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
    if (!drag.moved && Math.hypot(dx, dy) < 4) return;
    if (!drag.moved) { drag.moved = true; dragging = true; moved = true; hovered = null; try { box?.setPointerCapture(drag.id); } catch { /* not where it can be captured */ } }
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
    if (!(e.target as HTMLElement).closest("button, .card, .toolbar")) { moved = false; resetView(); }
  }
  // Scroll: up and down (sideways with a trackpad or Shift); Ctrl+scroll
  // zooms around the pointer. (Not passive: the page mustn't scroll.)
  $effect(() => {
    const el = box;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      hovered = null;
      moved = true;
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
    const yy = panY + (id === "pending" ? yRow(0) : y(id));
    if (yy < 40) panY += 40 - yy;
    else if (yy > boxHeight - 40) panY -= yy - (boxHeight - 40);
  }

  let link = $derived.by(() => {
    if (!reserve || !selected) return null;
    const id = selected;
    const c = id === "pending" ? (pending ? pendAt : undefined) : g.chainOf.get(id);
    if (!c || (id !== "pending" && !byID.has(id))) return null;
    const x1 = panX + x(c.col) + 12, y1 = panY + (id === "pending" ? yRow(0) : y(id));
    const x2 = width, y2 = Math.min(Math.max(y1, panelInset + 24), boxHeight - panelInset - 24);
    return x1 < x2 - 8 ? { x1, y1, x2, y2 } : null;
  });

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
<div class="graph-col" role="listbox" tabindex="-1" aria-label={t("Versions")} bind:this={box} bind:clientWidth={fullWidth} bind:clientHeight={boxHeight} {onkeydown}
  {onpointerdown} {onpointermove} {onpointerup} onpointercancel={onpointerup} {ondblclick} {onclickcapture}
  class:dragging>
  {#if versions.length === 0 && !pending}
    <p class="muted empty">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
  {:else}
    <div class="canvas" style:width="{full}px" style:height="{height}px" style:transform="translate({panX}px, {panY}px)">
      <svg width={full} height={height} aria-hidden="true">
        {#each g.edges as e (e.from + ">" + e.to)}
          {@const c = lineChain(e)}
          <path d={path(e)} stroke="var(--lane-{c.color})" stroke-width={c.main ? 4 : 3} fill="none" stroke-linecap="round"
            opacity={!c.name ? 0.55 : merged.has(c) ? 0.75 : 1} />
        {/each}
        {#if pending && headChain}
          <path d={pendingPath()} stroke="var(--lane-{headChain.color})" stroke-width="2" stroke-dasharray="3 4" fill="none" />
        {/if}
        {#each stubs as st (st.c.name)}
          <path d={stubPath(st)} stroke="var(--lane-{st.c.color})" stroke-width="3" fill="none" stroke-linecap="round" />
          <circle cx={x(st.c.col)} cy={yRow(st.row)} r="5" fill="var(--panel)" stroke="var(--lane-{st.c.color})" stroke-width="2" />
        {/each}
      </svg>

      {#each labels as l (l.name + "@" + l.id)}
        <button class="label" style:left="{center + l.dx}px" style:top="{top + l.y}px" style:width="{l.w}px" style:height="{l.h}px" style:--c="var(--lane-{l.color})"
          tabindex="-1" onclick={() => onselect(l.id)}>
          <span class="bname">{l.name}</span>
          {#if l.title}<span class="btitle">{l.title}</span>{/if}
        </button>
      {/each}

      {#if pending}
        <button class="node pending" class:on={selected === "pending"} data-id="pending" role="option"
          aria-selected={selected === "pending"} style:left="{x(pendAt.col)}px" style:top="{yRow(0)}px"
          style:--c="var(--lane-{pendAt.color})" title={t("Your changes")} aria-label={t("Your changes")}
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
    <div class="zoombar" style:left="{width - 14}px">
      <button class="ghost" onclick={() => zoomBy(1 / 1.2)} aria-label={t("Zoom out")} title={t("Zoom out") + " (Ctrl+scroll)"}>−</button>
      <button class="ghost pct" onclick={zoomReset} title={t("Back to 100%")}>{Math.round(zoom * 100)}%</button>
      <button class="ghost" onclick={() => zoomBy(1.2)} aria-label={t("Zoom in")} title={t("Zoom in") + " (Ctrl+scroll)"}>+</button>
      <button class="ghost locate" onclick={locate} aria-label={t("Back to 100% and to where you are")}
        title={t("Back to 100% and to where you are")}>
        <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
          <circle cx="8" cy="8" r="5" /><circle cx="8" cy="8" r="1" fill="currentColor" stroke="none" />
          <path d="M8 1v2.5M8 12.5V15M1 8h2.5M12.5 8H15" />
        </svg>
      </button>
    </div>
    {#if backTo}
      <div class="toolbar surface-menu" style:left="{Math.max(toolbarWidth / 2 + 12, width / 2)}px" bind:offsetWidth={toolbarWidth}>
        {#if backTo === "pending"}
          <button class="primary back" onclick={goBack}>{t("View pending changes")}</button>
        {:else}
          <button class="back light" onclick={goBack}>{t("View current version")}</button>
        {/if}
      </div>
    {/if}
    {#if link}
      <svg class="link" aria-hidden="true">
        <path d="M {link.x1} {link.y1} C {(link.x1 + link.x2) / 2} {link.y1}, {(link.x1 + link.x2) / 2} {link.y2}, {link.x2} {link.y2}"
          stroke="var(--muted)" stroke-width="1.2" stroke-dasharray="3 4" fill="none" />
      </svg>
    {/if}
    {#if card}
      {@const v = card.v}
      <div class="card surface-menu" role="group" aria-label={v.message || t("(no description)")} use:portal
        style:left="{card.left}px" style:top="{card.top}px" style:width="{CARD_W}px" style:--ay="{card.arrow}px"
        bind:clientHeight={cardHeight} onmouseenter={() => hover(v.id)} onmouseleave={unhover}>
        <span class="arrow" aria-hidden="true"></span>
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
  /* The toolbar floats at the bottom, in the middle of what the details leave. */
  .toolbar { position: absolute; bottom: var(--sp-20); transform: translateX(-50%); z-index: 3; display: flex; align-items: center;
    gap: var(--sp-2); padding: var(--sp-6); border: var(--border-width) solid var(--line); border-radius: var(--radius-pill);
    box-shadow: var(--shadow-pop); white-space: nowrap; cursor: default; }
  .toolbar button { border-radius: var(--radius-pill); padding: var(--sp-6) var(--sp-12); font-size: var(--fs-md); }
  /* Zoom: small and quiet, top right of what the details leave. */
  .zoombar { position: absolute; top: var(--sp-14); transform: translateX(-100%); z-index: 3; display: flex; align-items: center;
    gap: 0; cursor: default; opacity: .7; transition: opacity .15s; }
  .zoombar:hover, .zoombar:focus-within { opacity: 1; }
  .zoombar button { display: flex; align-items: center; justify-content: center; min-width: 22px; height: 22px;
    padding: 0 var(--sp-4); border: none; border-radius: var(--radius-sm); font-size: var(--fs-sm); color: var(--muted); }
  .zoombar button:hover:not(:disabled) { color: var(--text); background: var(--hover); }
  .zoombar .pct { min-width: 40px; font-family: var(--font-mono); font-size: var(--fs-xs); }
  .zoombar .locate { margin-left: var(--sp-2); }
  .zoombar svg { position: static; }
  .toolbar .back { font-weight: var(--fw-semibold); }
  .toolbar .light { background: var(--text); color: var(--bg); border-color: var(--text); }
  .toolbar .light:hover:not(:disabled) { background: var(--switch-knob); border-color: var(--switch-knob); }
  .link { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; overflow: visible; }
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
  .node.pending { border-style: dashed; color: var(--c); background: var(--panel); }
  .node.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .node.pending.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .node:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
  .card { position: fixed; z-index: var(--z-menu); padding: var(--sp-10) var(--sp-12); border: var(--border-width) solid var(--line);
    border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .arrow { position: absolute; right: -6px; top: var(--ay); width: 10px; height: 10px; margin-top: -5px;
    transform: rotate(45deg); background: var(--surface-menu);
    border-right: var(--border-width) solid var(--line); border-top: var(--border-width) solid var(--line); }
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
