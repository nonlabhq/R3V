// Moving through a list (or a tree of folders) with the keyboard, as in a
// file explorer: ↑ ↓ the next and previous row, Home End the first and last,
// PageUp PageDown a page; in a tree → opens a folder, then goes into it, ←
// closes it, else goes up to the folder the row is in. Pure: the list says
// what to do with the answer.

export type NavRow = { key: string; dir?: boolean; open?: boolean; depth?: number };
export type Nav = { to: string } | { toggle: string };

export function navKey(rows: NavRow[], at: string, key: string, page = 10): Nav | null {
  if (!rows.length) return null;
  const i = rows.findIndex((r) => r.key === at);
  const go = (j: number): Nav => ({ to: rows[Math.max(0, Math.min(rows.length - 1, j))].key });
  switch (key) {
    case "ArrowDown": return go(i < 0 ? 0 : i + 1);
    case "ArrowUp": return go(i < 0 ? 0 : i - 1);
    case "Home": return go(0);
    case "End": return go(rows.length - 1);
    case "PageDown": return go(i < 0 ? 0 : i + page);
    case "PageUp": return go(i < 0 ? 0 : i - page);
  }
  if (i < 0) return null;
  const r = rows[i], depth = r.depth ?? 0;
  if (key === "ArrowRight") {
    if (!r.dir) return null;
    if (!r.open) return { toggle: r.key };
    const next = rows[i + 1];
    return next && (next.depth ?? 0) > depth ? { to: next.key } : null;
  }
  if (key === "ArrowLeft") {
    if (r.dir && r.open) return { toggle: r.key };
    for (let j = i - 1; j >= 0; j--) if (rows[j].dir && (rows[j].depth ?? 0) < depth) return { to: rows[j].key };
    return null;
  }
  return null;
}

// In a grid of cols columns: ← → the one before and after, ↑ ↓ a row up
// and down, Home End the first and last.
export function gridKey(keys: string[], at: string, key: string, cols: number): string | null {
  if (!keys.length) return null;
  const i = keys.indexOf(at);
  const step = ({ ArrowRight: 1, ArrowLeft: -1, ArrowDown: cols, ArrowUp: -cols } as Record<string, number>)[key];
  if (key === "Home") return keys[0];
  if (key === "End") return keys[keys.length - 1];
  if (step === undefined) return null;
  if (i < 0) return keys[0];
  const j = i + step;
  return j < 0 || j >= keys.length ? keys[i] : keys[j];
}

// Keys a list leaves alone: typing in a field, or with Ctrl/Alt (the app's
// shortcuts).
export function ownKey(e: KeyboardEvent): boolean {
  const el = e.target as HTMLElement | null;
  if (e.ctrlKey || e.metaKey || e.altKey) return false;
  return !el?.closest?.("input:not([type=checkbox]), textarea, select, [contenteditable]");
}
