<script lang="ts">
  import { splits, saveSplits, splitPx } from "./splits.svelte";

  // The line between two panes, dragged to share their width. It sits in a
  // positioned parent whose first column is splitPx(key, …) wide.
  let { key, def, width, minLeft = 240, minRight = 280 }: {
    key: string; def: number; width: number; minLeft?: number; minRight?: number;
  } = $props();

  let left = $derived(splitPx(key, def, width, minLeft, minRight));

  function start(e: PointerEvent) {
    e.preventDefault();
    try { (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId); } catch { /* the window listens anyway */ }
    const x0 = e.clientX, w0 = left;
    const move = (m: PointerEvent) => {
      if (!(m.buttons & 1)) return up(); // let go where it wasn't seen
      if (width > 0) splits[key] = Math.min(Math.max(minLeft, w0 + m.clientX - x0), width - minRight) / width;
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
      saveSplits();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="splitter" style:left="{left}px" onpointerdown={start}></div>

<style>
  .splitter { position: absolute; top: 0; bottom: 0; width: 7px; margin-left: -4px; cursor: col-resize; z-index: 3; touch-action: none; }
  .splitter:hover, .splitter:active { background: linear-gradient(to right, transparent 3px, var(--accent) 3px, var(--accent) 4px, transparent 4px); }
</style>
