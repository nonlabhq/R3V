import { flushSync } from "svelte";

// Dragging an item to move it among its list's items (the list's
// [data-key] elements), along one axis: the sidebar's projects, the tabs.
// Past a few pixels a press becomes a drag (a click stays a click); the item
// then follows the pointer and takes its place among the others as it goes,
// which slide out of its way (animate:flip, the dragged one left out). It
// ends when the button is let go, wherever, or the drag is cancelled.

export type DragSort = {
  axis: "x" | "y";
  /** The press became a drag. */
  start(): void;
  /** The item goes before the item keyed before (undefined: at the end). */
  move(before: string | undefined): void;
  /** How far the item is drawn from its place, to stay under the pointer. */
  follow(d: number): void;
  /** Where the pointer is, before each move (to drop the item elsewhere:
   * a project on another section). */
  pointer?(m: PointerEvent): void;
  /** Let go; moved: it was dragged (not a click). */
  end(moved: boolean): void;
};

/** How far a press goes before it is a drag. */
export const DRAG_START = 5;

export function dragSort(e: PointerEvent, o: DragSort) {
  const item = e.currentTarget as HTMLElement;
  const list = item.parentElement;
  if (e.button !== 0 || !list) return;
  const x = o.axis === "x";
  const at = (m: PointerEvent) => (x ? m.clientX : m.clientY);
  const size = (el: HTMLElement) => (x ? el.offsetWidth : el.offsetHeight);
  // Where an item's place is on screen: offsetTop/Left leave out the slides
  // and the follow (transforms), so places don't swing while items move.
  const place = (el: HTMLElement) => {
    const r = list.getBoundingClientRect();
    return x ? r.left + el.offsetLeft - list.offsetLeft - list.scrollLeft : r.top + el.offsetTop - list.offsetTop - list.scrollTop;
  };
  const p0 = at(e);
  const grab = p0 - place(item);
  let moved = false;
  const move = (m: PointerEvent) => {
    if (!(m.buttons & 1)) return up();
    if (!moved && Math.abs(at(m) - p0) < DRAG_START) return;
    if (!moved) { moved = true; o.start(); }
    o.pointer?.(m);
    // Before the first other item whose middle is past the dragged one's.
    const mid = at(m) - grab + size(item) / 2;
    const before = [...list.querySelectorAll<HTMLElement>(":scope > [data-key]")]
      .find((el) => el !== item && mid < place(el) + size(el) / 2)?.dataset.key;
    // The order is drawn first, then the item is put under the pointer from
    // its new place, both before the next frame: it never jumps.
    flushSync(() => o.move(before));
    flushSync(() => o.follow(at(m) - grab - place(item)));
  };
  const up = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
    window.removeEventListener("pointercancel", up);
    o.end(moved);
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
  window.addEventListener("pointercancel", up);
}
