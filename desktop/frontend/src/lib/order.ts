// The sidebar's order of a team's projects, as dragged (this computer
// only): keys in order; projects not in it yet come after, as they came.

// sortByOrder puts list in order's order, the rest after in theirs.
export function sortByOrder<T>(list: T[], keyOf: (x: T) => string, order: string[]): T[] {
  const at = new Map(order.map((k, i) => [k, i]));
  const rank = (x: T) => at.get(keyOf(x)) ?? order.length;
  return list.map((x, i) => ({ x, i })).sort((a, b) => rank(a.x) - rank(b.x) || a.i - b.i).map((y) => y.x);
}

// moveKey moves key to position to (an index among keys, before the move:
// keys.length for the end).
export function moveKey(keys: string[], key: string, to: number): string[] {
  const from = keys.indexOf(key);
  if (from < 0) return keys;
  const at = to > from ? to - 1 : to;
  if (at === from) return keys;
  const out = keys.filter((k) => k !== key);
  out.splice(Math.max(0, Math.min(at, out.length)), 0, key);
  return out;
}

// readOrder takes what was saved: strings only.
export function readOrder(saved: unknown): string[] {
  return Array.isArray(saved) ? saved.filter((k): k is string => typeof k === "string") : [];
}
