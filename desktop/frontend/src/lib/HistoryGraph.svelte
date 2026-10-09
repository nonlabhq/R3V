<script lang="ts">
  import { t, tn } from "./i18n.svelte";
  import { ago, type MemberLook, type Version } from "./api";
  import { colorOf, cssColor } from "./palette";
  import Avatar from "./Avatar.svelte";
  import { branchGraph } from "./branchGraph";
  import { branchLabel, branchLane } from "./branches";
  import { portal } from "./portal";
  import type { Snippet } from "svelte";

  // The Overview's left column: the branch graph (see branchGraph.ts), main
  // in the middle. Each branch has its name at the top (picking it picks
  // its newest version; its right half opens the branch's settings); every
  // version is a dot with its author's picture, else their initial (on
  // their colour, when the team keeps looks); its title on hover.
  // Milestones are capsules at the left edge, a dashed line to their dot.
  // Your changes not committed yet are a dashed dot
  // above the version you're on. Picking one shows it on the right; ↑ ↓ move.
  // The view moves: drag to pan, scroll to go up and down, Ctrl+scroll to
  // zoom, double-click the background to put it back.
  let { versions, branches, branch, head, incoming, pending, parked = [], selected, onselect, actions, reserve = 0, panelInset = 0, looks, milestones, onsettings }: {
    versions: Version[];
    branches: { name: string; latest: string; label?: string; color?: string }[];
    branch: string;    // the branch you are on
    head: string;      // the version you are on
    incoming: Set<string>;
    pending: number;   // files changed and not committed (0: no dot for them)
    // Changes parked elsewhere (Nightly): a dashed tag beside the version they were made on.
    parked?: { key: string; base: string; files: number; since: string }[];
    selected: string;  // a version id, or "pending"
    onselect: (id: string) => void;
    actions?: Snippet<[Version]>; // the hover card's buttons for a version
    reserve?: number;    // px on the right covered by the details (the graph centres in the rest)
    panelInset?: number; // the details' distance from the top and bottom
    looks?: Record<string, MemberLook | undefined>; // by member id (none: the team keeps no looks)
    milestones?: { version: string; name: string }[]; // versions the team named
    onsettings?: (branch: string) => void; // a branch's settings (none: its label only picks)
  } = $props();

  // A row per version, a column per line of work (at 100%). Zoom spreads
  // them out or packs them in; the dots, lines and labels keep their size.
  const ROW = 56, COL = 64, PAD = 24, LABEL_W = 180, LABEL_H = 36;
  // The view: pan (px on the screen) and zoom.
  let panX = $state(0), panY = $state(0), zoom = $state(1);
  const ZOOM_MIN = 0.5, ZOOM_MAX = 2.5;
  let rowH = $derived(ROW * zoom), colW = $derived(COL * zoom);
  let g = $derived(branchGraph(versions, branches, branch, "main", head, (name) => branchLane(branches, name)));
  const labelOf = (name: string) => branchLabel(branches, name);
  // A label's text width, as drawn (its font measured; estimated where
  // there's no canvas).
  let measure: CanvasRenderingContext2D | null | undefined;
  function textWidth(text: string): number {
    if (measure === undefined) {
      try {
        measure = document.createElement("canvas").getContext("2d");
        if (measure) measure.font = `600 10px ${getComputedStyle(document.body).fontFamily}`;
      } catch { measure = null; }
    }
    return measure ? measure.measureText(text).width
      : [...text].reduce((n, ch) => n + (ch.codePointAt(0)! >= 0x2e80 ? 11 : 6), 0);
  }
  let flags = $derived.by(() => {
    const m = new Map<string, string[]>();
    for (const ms of milestones ?? []) m.set(ms.version, [...(m.get(ms.version) ?? []), ms.name]);
    return m;
  });
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
  // Parked changes: a tag up beside their version (to the outside of the graph), a dashed line to it.
  let parks = $derived(parked.filter((p) => byID.has(p.base)).map((p) => {
    const c = g.chainOf.get(p.base)!;
    return { ...p, col: c.col, color: c.color, side: c.col < 0 ? -1 : 1 };
  }));
  // Branches merged into another: quieter.
  let merged = $derived(new Set(g.edges.filter((e) => e.kind === "merge").map((e) => g.chainOf.get(e.to)!)));

  let fullWidth = $state(0); // of the column
  // What can be seen: the column less the part under the details.
  let width = $derived(Math.max(120, fullWidth - reserve));
  let boxHeight = $state(0);

  // Milestones: a capsule at the graph's left edge (⚑, the version number
  // when the name starts with one, then the name), a dashed line over to
  // the version's dot and a ring round the dot, in its branch's colour.
  // Positions are from the canvas's left edge (x) and the first row (y).
  const MS_W = 160, MS_GAP = 28;
  type Mark = { id: string; text: string; ver: string; name: string; w: number; y: number; col: number; color: number };
  let marks = $derived.by(() => {
    const out: Mark[] = [];
    for (const [id, names] of flags) {
      const c = g.chainOf.get(id);
      const row = g.row.get(id);
      if (!c || row === undefined) continue;
      const text = names.join(" · ");
      const m = /^(v\d+(?:\.\d+)*|\d+(?:\.\d+)+)\s+(.+)$/i.exec(text);
      // (wide scripts, CJK and the like, take about twice a Latin letter)
      const w = Math.min(MS_W, [...text].reduce((n, ch) => n + (ch.codePointAt(0)! >= 0x2e80 ? 13 : 7), 0) + 34);
      out.push({ id, text, ver: m ? m[1] : "", name: m ? m[2] : text, w, y: (row + off) * rowH + rowH / 2, col: c.col, color: c.color });
    }
    return out;
  });
  // Room for them at the left (the graph starts to their right).
  let markRoom = $derived(marks.length ? Math.max(...marks.map((m) => m.w)) + MS_GAP : 0);
  // The milestones' capsules line up on one edge: the view's left edge as
  // the graph moves (their dashed lines stretching), short of the dot most
  // to the left; they leave the view only with it.
  let marksAt = $derived(Math.min(PAD - panX, ...marks.map((m) => x(m.col) - 19 - 12 - m.w)));
  // Labels: one per branch, its name, above its top dot (your changes, on
  // your branch). Placed clear of the dots and of each other: centred, else
  // leaning to the branch's side, else a row higher. Positions are from the
  // middle column (dx) and the first row (y).
  type Label = { id: string; name: string; color: number; dx: number; y: number; w: number; h: number };
  let labels = $derived.by(() => {
    const out: Label[] = [];
    const dots = [...versions.map((v) => ({ col: g.chainOf.get(v.id)!.col, row: g.row.get(v.id)! + off })),
      ...(pending ? [{ col: pendAt.col, row: 0 }] : []), ...stubs.map((st) => ({ col: st.c.col, row: st.row }))];
    const clear = (dx: number, y: number, w: number, h: number) =>
      dots.every((d) => d.col * colW + 12 < dx || d.col * colW - 12 > dx + w || d.row * rowH + rowH / 2 + 12 < y || d.row * rowH + rowH / 2 - 12 > y + h) &&
      out.every((l) => l.dx + l.w + 4 < dx || l.dx > dx + w + 4 || l.y + l.h + 4 < y || l.y > y + h + 4);
    // (before the first version: your branch's name over your changes)
    const first = pending && !headChain ? [{ name: branch, tip: "", col: 0, color: branchLane(branches, branch), empty: true }] : [];
    for (const c of [...g.chains, ...first]) {
      if (!c.name) continue;
      const atPending = pending && (c === headChain || !c.tip);
      const rowY = atPending ? 0 : c.empty ? g.row.get(c.tip)! + off - 0.8 : g.row.get(c.tip)! + off;
      // As wide as the name, up to LABEL_W (the settings button grows out
      // on hover: it takes no room).
      const w = Math.min(LABEL_W, Math.ceil(textWidth(labelOf(c.name))) + 20);
      const h = 24;
      const at = c.col * colW;
      const lean = c.col > 0 ? [at - w / 2, at - 14, at + 14 - w] : [at - w / 2, at + 14 - w, at - 14];
      let place = { dx: lean[0], y: rowY * rowH + rowH / 2 - 16 - h };
      search: for (let k = 0; k < 30; k++) {
        const y = rowY * rowH + rowH / 2 - 16 - h - k * (LABEL_H - 4);
        for (const dx of lean) if (clear(dx, y, w, h)) { place = { dx, y }; break search; }
      }
      out.push({ id: c.tip || "pending", name: c.name, color: c.color, ...place, w, h });
    }
    return out;
  });
  let center = $derived(PAD + markRoom - Math.min(-g.left * colW - 12, ...labels.map((l) => l.dx)));
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
  // How a version's author shows (null: initials as they always were).
  const lookOf = (v: Version) => looks && {
    color: colorOf(looks[v.authorId]?.color ?? "", v.authorId || v.author), picture: looks[v.authorId]?.picture ?? "" };
  // Pictures that can't be shown (bad data from the team): initials.
  let broken = $state(new Set<string>());
  const showPic = (lk: ReturnType<typeof lookOf>) => !!lk && !!lk.picture && !broken.has(lk.picture);

  // The card for the version under the pointer: what it is, and what can be
  // done with it. It stays while the pointer is on the dot or the card.
  const CARD_W = 220;
  let hovered = $state<string | null>(null);
  // Placed by its height folded (its buttons open below on hover, the card
  // staying where it is): the head's, and the padding.
  let headHeight = $state(60);
  let cardHeight = $derived(headHeight + 22);
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
  function unhover() { clearTimeout(hideTimer); hideTimer = setTimeout(() => { hovered = null; previewing = ""; }, 180); }
  // What a card's button would do, shown on the graph while the pointer is on
  // it (the button says which: data-preview): going to a version, its dot
  // glows; merging it or branching from it, the version that would make,
  // faint, with its lines.
  let previewing = $state("");
  function cardOver(e: Event) {
    previewing = (e.target as HTMLElement).closest<HTMLElement>("[data-preview]")?.dataset.preview ?? "";
  }
  let ghost = $derived.by(() => {
    const v = card?.v;
    if (!v || !previewing || previewing === "goto") return null;
    const at = g.chainOf.get(v.id)!;
    if (previewing === "branch") {
      const col = g.right + 1, gy = y(v.id) - rowH;
      return { x: x(col), y: gy, color: at.color, from: [{ x: x(at.col), y: y(v.id), kind: "fork" }], label: t("New branch") };
    }
    const into = previewing === "main" ? "main" : branch;
    const tip = branches.find((b) => b.name === into)?.latest || (into === branch ? head : "");
    const c = tip ? g.chainOf.get(tip) : undefined;
    if (!c || !byID.has(tip)) return null;
    // (above your changes too, on your branch)
    const gy = Math.min(y(tip), y(v.id), pending && into === branch ? yRow(0) : Infinity) - rowH;
    return { x: x(c.col), y: gy, color: c.color, label: "",
      from: [{ x: x(c.col), y: y(tip), kind: "line" }, { x: x(at.col), y: y(v.id), kind: "merge" }] };
  });
  const ghostPath = (a: { x: number; y: number }, b: { x: number; y: number }) => a.x === b.x
    ? `M ${a.x} ${a.y} L ${b.x} ${b.y}` : `M ${a.x} ${a.y} C ${a.x} ${(a.y + b.y) / 2}, ${b.x} ${(a.y + b.y) / 2}, ${b.x} ${b.y}`;
  let card = $derived.by(() => {
    const v = hovered ? byID.get(hovered) : undefined;
    if (!v) return null;
    const c = g.chainOf.get(v.id)!;
    // To the left of the dot, centred on it: the pointer can go up and down
    // the versions without the card in the way. No room on the left (the
    // window's edge): to its right, never over the dot. The little arrow
    // points at the dot. (In the window: over the sidebar if it comes to that.)
    const nx = origin.left + panX + x(c.col), ny = origin.top + panY + y(v.id), gap = 22;
    const vh = typeof window === "undefined" ? 800 : window.innerHeight;
    const right = nx - gap - CARD_W < 8;
    const left = right ? nx + gap : nx - gap - CARD_W;
    const top = Math.min(Math.max(8, ny - cardHeight / 2), Math.max(8, vh - 8 - cardHeight));
    return { v, branch: c.name, left, top, right, arrow: Math.min(Math.max(14, ny - top), cardHeight - 14) };
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
  let drag: { x: number; y: number; px: number; py: number; moved: boolean; id: number; mask: number } | null = null;
  let dragging = $state(false);
  // (the left button, or the middle one anywhere: it never clicks)
  function onpointerdown(e: PointerEvent) {
    if (e.button === 1) e.preventDefault(); // (no autoscroll)
    if ((e.button !== 0 && e.button !== 1) || (e.button === 0 && (e.target as HTMLElement).closest(".card, .toolbar, .zoombar"))) return;
    drag = { x: e.clientX, y: e.clientY, px: panX, py: panY, moved: false, id: e.pointerId, mask: e.button === 1 ? 4 : 1 };
  }
  function onpointermove(e: PointerEvent) {
    if (!drag) return;
    // The button let go outside (where its release wasn't seen): no drag.
    if (!(e.buttons & drag.mask)) { onpointerup(); return; }
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
  // A click on the background: back to what matters now (your changes, else
  // the version you are on).
  function onclick(e: MouseEvent) {
    if (!(e.target as HTMLElement).closest("button, .card, .toolbar, .zoombar, .label")) {
      const id = pending ? "pending" : head;
      if (id && id !== selected) onselect(id);
    }
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
  {onpointerdown} {onpointermove} {onpointerup} onpointercancel={onpointerup} {onclick} {ondblclick} {onclickcapture}
  onauxclick={(e) => e.button === 1 && e.preventDefault()}
  class:dragging>
  {#if versions.length === 0 && !pending}
    <p class="muted empty">{t("No versions yet. Commit your first version from the Changes tab.")}</p>
  {:else}
    <div class="canvas" style:width="{full}px" style:height="{height}px" style:transform="translate({panX}px, {panY}px)">
      <svg width={full} height={height} aria-hidden="true">
        {#each marks as m (m.id)}
          <path class="mline" d="M {marksAt + m.w} {top + m.y} L {x(m.col) - 19} {top + m.y}" stroke="var(--lane-{m.color})"
            stroke-width="1.5" stroke-dasharray="4 4" fill="none" />
        {/each}
        {#each g.edges as e (e.from + ">" + e.to)}
          {@const c = lineChain(e)}
          <path d={path(e)} stroke="var(--lane-{c.color})" stroke-width={c.main ? 4 : 3} fill="none" stroke-linecap="round"
            opacity={!c.name ? 0.55 : merged.has(c) ? 0.75 : 1} />
        {/each}
        {#each parks as p (p.key)}
          <path d="M {x(p.col)} {y(p.base)} Q {x(p.col) + p.side * 22} {y(p.base) - rowH * 0.45}, {x(p.col) + p.side * 34} {y(p.base) - rowH * 0.45}"
            stroke="var(--lane-{p.color})" stroke-width="1.5" stroke-dasharray="3 3" fill="none" />
        {/each}
        {#if pending && headChain}
          <path d={pendingPath()} stroke="var(--lane-{headChain.color})" stroke-width="2" stroke-dasharray="3 4" fill="none" />
        {/if}
        {#each stubs as st (st.c.name)}
          <path d={stubPath(st)} stroke="var(--lane-{st.c.color})" stroke-width="3" fill="none" stroke-linecap="round" />
          <circle cx={x(st.c.col)} cy={yRow(st.row)} r="5" fill="var(--panel)" stroke="var(--lane-{st.c.color})" stroke-width="2" />
        {/each}
        {#if ghost}
          <g class="would">
            {#each ghost.from as f, i (i)}
              <path d={ghostPath(f, ghost)} stroke="var(--lane-{ghost.color})" stroke-width="3" stroke-dasharray="5 4" fill="none" stroke-linecap="round" />
            {/each}
            <circle cx={ghost.x} cy={ghost.y} r="11" fill="var(--panel)" stroke="var(--lane-{ghost.color})" stroke-width="2" stroke-dasharray="4 3" />
            {#if ghost.label}<text x={ghost.x + 18} y={ghost.y + 4} fill="var(--lane-{ghost.color})">{ghost.label}</text>{/if}
          </g>
        {/if}
        {#each marks as m (m.id)}
          <circle class="mring" cx={x(m.col)} cy={top + m.y} r="18" fill="none" stroke="var(--lane-{m.color})" stroke-width="2" />
        {/each}
      </svg>

      {#each marks as m (m.id)}
        <button class="mtag" style:--c="var(--lane-{m.color})" style:left="{marksAt}px" style:top="{top + m.y}px" style:max-width="{MS_W}px"
          tabindex="-1" title={m.text} aria-label={m.text} onclick={() => onselect(m.id)}>
          <span class="mflag" aria-hidden="true">⚑</span>{#if m.ver}<b>{m.ver}</b>{/if}<span class="mname">{m.name}</span>
        </button>
      {/each}
      {#each labels as l (l.name + "@" + l.id)}
        <div class="label" style:left="{center + l.dx}px" style:top="{top + l.y}px" style:width="{l.w}px" style:height="{l.h}px" style:--c="var(--lane-{l.color})">
          <button class="lpick" tabindex="-1" onclick={() => onselect(l.id)}><span class="bname">{labelOf(l.name)}</span></button>
          {#if onsettings}
            <button class="lset" tabindex="-1" title={t("Branch settings")} aria-label={t("Branch settings")}
              onclick={() => onsettings(l.name)}><span aria-hidden="true">⋯</span></button>
          {/if}
        </div>
      {/each}

      {#if pending}
        <button class="node pending now" class:on={selected === "pending"} data-id="pending" role="option"
          aria-selected={selected === "pending"} style:left="{x(pendAt.col)}px" style:top="{yRow(0)}px"
          style:--c="var(--lane-{pendAt.color})" title={t("Your changes")} aria-label={t("Your changes")}
          onclick={() => onselect("pending")}>+</button>
      {/if}
      {#each parks as p (p.key)}
        <button class="park" class:on={selected === p.key} class:left={p.side < 0} data-id={p.key}
          style:left="{x(p.col) + p.side * 34}px" style:top="{y(p.base) - rowH * 0.45}px" style:--c="var(--lane-{p.color})"
          title={`${tn(p.files, "{n} parked change", "{n} parked changes")} · ${ago(p.since)}`} onclick={() => onselect(p.key)}>
          <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M6 4v8M10 4v8" stroke="currentColor" stroke-width="2" stroke-linecap="round" /></svg>{t("Parked")} · {p.files}</button>
      {/each}
      {#each versions as v (v.id)}
        {@const c = g.chainOf.get(v.id)!}
        {@const lk = lookOf(v)}
        <button class="node" class:on={selected === v.id} class:here={v.id === head} class:now={(v.id === head && !pending) || (previewing === "goto" && card?.v.id === v.id)} class:incoming={incoming.has(v.id)}
          class:side={!c.name} data-id={v.id} role="option" aria-selected={selected === v.id}
          style:left="{x(c.col)}px" style:top="{y(v.id)}px" style:--c="var(--lane-{c.color})"
          class:tinted={!!lk} class:pic={showPic(lk)} style:--m={lk ? cssColor(lk.color) : undefined}
          aria-label={`${v.message || t("(no description)")}, ${v.author}, ${ago(v.time)}${flags.has(v.id) ? `, ⚑ ${flags.get(v.id)!.join(", ")}` : ""}`}
          onmouseenter={() => hover(v.id)} onmouseleave={unhover} onfocus={() => hover(v.id)}
          onclick={() => onselect(v.id)}>{#if showPic(lk)}<img src={lk!.picture} alt="" draggable="false"
            onerror={() => (broken = new Set(broken).add(lk!.picture))} />{:else}{initial(v.author)}{/if}</button>
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
      <!-- svelte-ignore a11y_mouse_events_have_key_events (focusin: the same, from the keyboard) -->
      <div class="card surface-menu" class:right={card.right} role="group" aria-label={v.message || t("(no description)")} use:portal
        style:left="{card.left}px" style:top="{card.top}px" style:width="{CARD_W}px" style:--ay="{card.arrow}px"
        onmouseenter={() => hover(v.id)} onmouseleave={() => { previewing = ""; unhover(); }} onmouseover={cardOver} onfocusin={cardOver}>
        <span class="arrow" aria-hidden="true"></span>
        <div class="card-h" bind:clientHeight={headHeight}>
          {#if looks}
            {@const lk = lookOf(v)!}
            <Avatar name={v.author} seed={v.authorId || v.author} color={lk.color} picture={lk.picture} />
          {:else}
            <span class="avatar" style:--c="var(--lane-{g.chainOf.get(v.id)!.color})">{initial(v.author)}</span>
          {/if}
          <div class="card-t">
            <div class="card-msg" title={v.short}>{v.message || t("(no description)")}</div>
            <div class="card-meta">{v.author} · {ago(v.time)}{#if card.branch} · {labelOf(card.branch)}{/if}</div>
            {#if flags.has(v.id)}<div class="card-flag">⚑ {flags.get(v.id)!.join(" · ")}</div>{/if}
          </div>
        </div>
        {#if actions}<div class="acts-fold"><div class="card-acts">{@render actions(v)}</div></div>{/if}
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
  svg { position: absolute; left: 0; top: 0; overflow: visible; } /* (a preview may reach past the graph) */
  .empty { padding: var(--sp-16); }
  .node { position: absolute; width: 24px; height: 24px; margin: -12px 0 0 -12px; padding: 0; border-radius: 50%;
    border: 2px solid var(--c); background: var(--panel-2); color: var(--text); font-size: var(--fs-xs);
    font-weight: var(--fw-semibold); line-height: 20px; text-align: center; }
  .node:hover:not(:disabled) { border-color: var(--c); background: var(--hover); }
  .node.tinted { color: var(--m); background: color-mix(in srgb, var(--m) 22%, var(--panel)); }
  .node.pic { line-height: 0; } /* (the img is round itself: the flag may stick out) */
  .node img { width: 100%; height: 100%; border-radius: 50%; object-fit: cover; pointer-events: none; }
  .node.here { background: var(--c); color: var(--bg); }
  /* where your files are now (your changes, else the version you're on): a glow in its branch's colour */
  .node.now { filter: drop-shadow(0 0 6px color-mix(in srgb, var(--c) 75%, transparent)) drop-shadow(0 0 14px color-mix(in srgb, var(--c) 40%, transparent)); }
  .node.here.pic { box-shadow: 0 0 0 2px var(--c); }
  .node.incoming { border-style: dashed; color: var(--muted); }
  .node.side { opacity: .75; }
  .node.pending { border-style: dashed; color: var(--c); background: var(--panel); }
  .node.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .node.pending.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .node:focus-visible { outline: 2px solid var(--accent); outline-offset: 4px; }
  /* parked changes: a dashed tag, not a version */
  /* a card's button previewed: what it would make, faint */
  .would { opacity: .5; pointer-events: none; }
  .would text { font-size: var(--fs-xs); font-weight: var(--fw-semibold); }
  .park { position: absolute; display: inline-flex; align-items: center; gap: var(--sp-4); height: 20px; padding: 0 var(--sp-8) 0 var(--sp-6);
    transform: translateY(-50%); border: 1.5px dashed var(--c); border-radius: var(--radius-pill); background: var(--panel);
    color: var(--c); font-size: var(--fs-xs); font-weight: var(--fw-semibold); white-space: nowrap; }
  .park.left { transform: translate(-100%, -50%); }
  .park svg { width: 10px; height: 10px; }
  .park:hover:not(:disabled) { background: var(--hover); border-color: var(--c); }
  .park.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 3.5px var(--c); }
  .card { position: fixed; z-index: var(--z-menu); padding: var(--sp-10) var(--sp-12); border: var(--border-width) solid var(--line);
    border-radius: var(--radius-lg); box-shadow: var(--shadow-pop); }
  .arrow { position: absolute; right: -6px; top: var(--ay); width: 10px; height: 10px; margin-top: -5px;
    transform: rotate(45deg); background: var(--surface-menu);
    border-right: var(--border-width) solid var(--line); border-top: var(--border-width) solid var(--line); }
  .card.right .arrow { right: auto; left: -6px; transform: rotate(-135deg); }
  .card-h { display: flex; gap: var(--sp-10); align-items: flex-start; }
  .avatar { flex: none; width: 24px; height: 24px; border-radius: 50%; border: 2px solid var(--c); display: flex;
    align-items: center; justify-content: center; font-size: var(--fs-xs); font-weight: var(--fw-semibold); }
  .card-t { min-width: 0; }
  .card-msg { font-size: var(--fs-md); font-weight: var(--fw-semibold); display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3;
    -webkit-box-orient: vertical; overflow: hidden; user-select: text; }
  .card-meta { font-size: var(--fs-xs); color: var(--faint); margin-top: var(--sp-2); }
  .card-flag { font-size: var(--fs-xs); color: var(--accent); margin-top: var(--sp-2); font-weight: var(--fw-semibold); }
  /* A milestone: a capsule at the left edge, a dashed line to its dot, a ring round the dot. */
  .mtag { position: absolute; height: 22px; transform: translateY(-50%); display: inline-flex; align-items: center;
    gap: var(--sp-4); padding: 0 var(--sp-8); font-size: var(--fs-xs); white-space: nowrap; z-index: 1;
    --tint: color-mix(in srgb, var(--c) 16%, var(--panel)); /* (opaque: the lines stay under it) */
    color: var(--c); background: var(--tint); border: var(--border-width) solid transparent; border-radius: var(--radius-pill); }
  .mtag:hover:not(:disabled) { background: var(--tint); border-color: var(--c); }
  .mtag b { color: var(--text); font-weight: var(--fw-semibold); }
  .mtag .mname { overflow: hidden; text-overflow: ellipsis; }
  .mline { opacity: .55; }
  /* A card comes in quietly; its buttons open below on hover (the card
     stays where it is). */
  .card { animation: card-in .16s ease-out; }
  @keyframes card-in { from { opacity: 0; transform: translateX(-8px); } to { opacity: 1; transform: none; } }
  .acts-fold { display: grid; grid-template-rows: 0fr; transition: grid-template-rows .16s ease-out; }
  .card:hover .acts-fold, .card:focus-within .acts-fold { grid-template-rows: 1fr; }
  .card-acts { min-height: 0; overflow: hidden; display: flex; flex-direction: column; gap: var(--sp-2); }
  .card-acts > :global(:first-child) { margin-top: var(--sp-10); }
  .card-acts :global(button) { justify-content: flex-start; text-align: left; padding: var(--sp-4) var(--sp-8); font-size: var(--fs-xs); }
  .card-acts :global(button.danger-act) { color: var(--danger); }
  /* A branch's label: its name (picks its newest version); with settings,
     a ⋯ floats just outside its right end on hover (the label keeps its
     size and frame). */
  .label { position: absolute; border: var(--border-width) solid var(--line); border-radius: var(--radius);
    background: var(--panel); line-height: 1.2; }
  .label:hover { border-color: var(--c); z-index: 2; }
  .label button { position: absolute; margin: 0; border: none; border-radius: 0; background: transparent; }
  .lpick { inset: 0; display: flex; align-items: center; justify-content: center; padding: 0 var(--sp-8); border-radius: var(--radius) !important; }
  .label .lpick:hover:not(:disabled) { background: transparent; }
  /* (from the label's edge, so the pointer crosses to it without leaving) */
  .lset { top: 50%; left: 100%; transform: translateY(-50%); width: 26px; height: 22px; padding: 0 0 0 var(--sp-4) !important;
    display: flex; align-items: center; justify-content: center; cursor: pointer; opacity: 0; pointer-events: none; transition: opacity .12s; }
  .lset span { width: 22px; height: 22px; display: flex; align-items: center; justify-content: center; border-radius: 50%;
    color: var(--c); font-size: var(--fs-sm); background: var(--panel-2); box-shadow: var(--shadow-pop); }
  .label:hover .lset, .lset:focus-visible { opacity: 1; pointer-events: auto; }
  .label .lset:hover:not(:disabled) { background: transparent; }
  .lset:hover span { background: color-mix(in srgb, var(--c) 22%, var(--panel-2)); }
  .bname { font-size: var(--fs-2xs); font-weight: var(--fw-semibold); color: var(--c); max-width: 100%;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
