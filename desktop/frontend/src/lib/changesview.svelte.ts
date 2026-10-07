// How the list of changes is shown, per project: a flat list, or a tree of
// folders when there are many (more than TREE_FROM). One picked by hand is
// kept (remembered) until the next commit.
export type ChangesView = "list" | "tree";
export const TREE_FROM = 10;

const key = (root: string) => `r3v.changesView:${root}`;
const picked = $state<Record<string, ChangesView | null>>({});

function stored(root: string): ChangesView | null {
  try {
    const v = localStorage.getItem(key(root));
    return v === "list" || v === "tree" ? v : null;
  } catch {
    return null;
  }
}

export function changesView(root: string, count: number): ChangesView {
  const v = root in picked ? picked[root] : stored(root);
  return v ?? (count > TREE_FROM ? "tree" : "list");
}

export function pickChangesView(root: string, v: ChangesView) {
  picked[root] = v;
  try { localStorage.setItem(key(root), v); } catch { /* kept for now */ }
}

// After a commit: back to choosing by the number of changes.
export function resetChangesView(root: string) {
  picked[root] = null;
  try { localStorage.removeItem(key(root)); } catch { /* none */ }
}
