// Puts a node at the end of the page, for menus and cards placed on the
// window (position: fixed): inside a part with a filter or blur (the
// Overview's floating details), fixed is from that part, not the window,
// and the part may clip it.
export function portal(node: HTMLElement) {
  document.body.appendChild(node);
  return { destroy: () => node.remove() };
}
