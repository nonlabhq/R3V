// Widths set by dragging (the Files explorer's tree, panel and columns).

/** A drag that sets a width: set gets how far the pointer went (dx). */
export function dragWidth(e: PointerEvent, set: (dx: number) => void) {
  if (e.button !== 0) return;
  e.preventDefault();
  e.stopPropagation();
  const x0 = e.clientX;
  const move = (m: PointerEvent) => { if (!(m.buttons & 1)) return up(); set(m.clientX - x0); };
  const up = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
    window.removeEventListener("pointercancel", up);
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
  window.addEventListener("pointercancel", up);
}

/** n between lo and hi, in whole pixels. */
export const clamp = (n: number, lo: number, hi: number) => Math.round(Math.min(hi, Math.max(lo, n)));
