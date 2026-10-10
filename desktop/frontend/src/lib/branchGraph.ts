// The Overview's branch graph, drawn as the tree of branches
// (docs/design/branch-tree.md): time runs down (newest at the top, one row
// per version), main is the middle column, and each branch grows to the
// right of the branch it was made from, those made earlier nearer it. A
// column is used again once a branch above has ended. The same wherever
// you are: the branch you're on is marked, not moved.
//
// A version is on the branch it was made on (its `branch`). Versions from
// before R3V kept that are followed down from each branch's newest version.

export type GraphVersion = { id: string; parents: string[]; branch?: string };

export type Chain = {
  name: string;   // the branch ("" for one gone since, or versions of no branch)
  tip: string;    // its newest version here
  ids: string[];  // its versions, newest first
  col: number;    // 0 the middle; > 0 right (< 0: left, not used now)
  main: boolean;
  mine: boolean;  // the branch you are on (when it isn't main)
  color: number;  // lane colour index
  // A branch with no versions of its own yet (just made): it starts at tip,
  // a version of another branch, and has no versions here (ids is empty).
  empty?: boolean;
  parent?: Chain; // the branch it was made from (none for main)
};

export type Edge = { from: string; to: string; kind: "line" | "fork" | "merge" };

export type BranchGraph = {
  chains: Chain[];
  chainOf: Map<string, Chain>;
  row: Map<string, number>; // a version's row (0 at the top)
  edges: Edge[];            // child to parent
  left: number;             // columns used to the left of the middle
  right: number;            // and to the right
};

// branches: each branch's key, newest version and (where the team keeps it)
// the branch it was made from; current: the branch you are on. mainName:
// the middle column (the team's main branch). laneOf: a branch's own colour
// (its lane index), when branches have one; lines of no branch any more
// take the palette's in order.
export function branchGraph(versions: GraphVersion[], branches: { name: string; latest: string; parent?: string }[],
  current: string, mainName = "main", head = "", laneOf?: (name: string) => number): BranchGraph {
  const byID = new Map(versions.map((v) => [v.id, v]));
  const row = new Map(versions.map((v, i) => [v.id, i]));
  const chainOf = new Map<string, Chain>();
  const chains: Chain[] = [];
  const known = new Set(branches.map((b) => b.name));
  const mainBranch = known.has(mainName) ? mainName : current;

  // Versions that say their branch: on it, for good (one line per branch
  // key; a key no branch has any more is a line of no name).
  const byKey = new Map<string, Chain>();
  for (const v of versions) {
    if (!v.branch) continue;
    let c = byKey.get(v.branch);
    if (!c) {
      const named = known.has(v.branch) || v.branch === current;
      c = { name: named ? v.branch : "", tip: v.id, ids: [], col: 0, main: v.branch === mainBranch, mine: false, color: 0 };
      byKey.set(v.branch, c);
      chains.push(c);
    }
    c.ids.push(v.id);
    chainOf.set(v.id, c);
  }

  // Versions from before: followed down from a branch's newest version
  // (first parents), as long as no other line has them.
  function walk(name: string, tip: string, main: boolean) {
    if (!byID.has(tip) || chainOf.has(tip)) return;
    let c = name ? chains.find((x) => x.name === name) : undefined;
    if (!c) {
      c = { name, tip, ids: [], col: 0, main, mine: false, color: 0 };
      chains.push(c);
    }
    for (let id: string | undefined = tip; id && byID.has(id) && !chainOf.has(id); id = byID.get(id)!.parents[0]) {
      chainOf.set(id, c);
      c.ids.push(id);
    }
    c.ids.sort((a, b) => row.get(a)! - row.get(b)!);
    c.tip = c.ids[0];
  }
  // Your branch's line starts at the version you are on when that is newer
  // than what the team has of it (versions not shared yet).
  const descends = (from: string, to: string) => {
    for (let id: string | undefined = from, n = 0; id && byID.has(id) && n <= versions.length; id = byID.get(id)!.parents[0], n++) {
      if (id === to) return true;
    }
    return false;
  };
  const tipOf = (name: string) => {
    const t = branches.find((b) => b.name === name)?.latest ?? "";
    return name === current && head && t !== head && (!t || descends(head, t)) ? head : t;
  };
  walk(mainBranch, tipOf(mainBranch), true);
  if (current !== mainBranch) walk(current, tipOf(current), false);
  for (const b of branches) walk(b.name, b.latest, false);
  // Where you are, if no branch has it: a line of its own, unnamed.
  if (head && !chainOf.has(head)) walk("", head, false);
  // Branches whose newest version is another's (just made, nothing committed
  // on them yet): drawn too, starting from that version.
  for (const b of branches) {
    const tip = b.latest || (b.name === current ? head : "");
    if (!b.name || !byID.has(tip) || chains.some((c) => c.name === b.name)) continue;
    chains.push({ name: b.name, tip, ids: [], col: 0, main: false, mine: false, color: 0, empty: true });
  }
  // Whatever is left came in through merges: its own lines.
  for (const v of versions) walk("", v.id, false);
  if (!chains.some((c) => c.main) && chains.length) chains[0].main = true;
  for (const c of chains) c.mine = !c.main && !!c.name && c.name === current;

  // The tree: a branch's parent is the one the team says it was made from,
  // else the line of the version it starts from.
  const top = chains.find((c) => c.main);
  const named = new Map(chains.filter((c) => c.name).map((c) => [c.name, c]));
  const startOf = (c: Chain) => c.empty ? c.tip : byID.get(c.ids[c.ids.length - 1])!.parents[0];
  for (const c of chains) {
    if (c.main) continue;
    const said = branches.find((b) => b.name === c.name)?.parent;
    const from = startOf(c);
    let p = (said && named.get(said)) || (from !== undefined ? chainOf.get(from) : undefined) || top;
    // (never under itself, nor in a loop)
    for (let q: Chain | undefined = p, n = 0; q; q = q.parent, n++) if (q === c || n > chains.length) { p = top; break; }
    c.parent = p === c ? undefined : p;
  }

  // Columns: down the tree from main (children in the order they were
  // made), each in the nearest free column right of its parent's (free from
  // a row above its top, for its label, to where it split).
  const span = (c: Chain) => {
    // (an empty branch: from where it starts up to its label; the one you
    // are on, up to the top, where your changes are)
    if (c.empty) return { top: c.name === current ? -2 : row.get(c.tip)! - 2, bottom: row.get(c.tip)! };
    const from = startOf(c);
    return { top: row.get(c.tip)! - 1, bottom: from !== undefined && row.has(from) ? row.get(from)! : versions.length };
  };
  const children = (p: Chain | undefined) => chains.filter((c) => !c.main && c.parent === p)
    .sort((a, b) => span(b).bottom - span(a).bottom || a.name.localeCompare(b.name));
  const used = new Map<number, { top: number; bottom: number }[]>();
  let right = 0;
  const placed = new Set<Chain>();
  const place = (c: Chain, from: number) => {
    const s = span(c);
    for (let col = from + 1; ; col++) {
      const taken = used.get(col) ?? [];
      if (taken.every((t) => s.bottom < t.top || s.top > t.bottom)) {
        taken.push(s);
        used.set(col, taken);
        c.col = col;
        break;
      }
    }
    placed.add(c);
    right = Math.max(right, c.col);
    for (const k of children(c)) if (!placed.has(k)) place(k, c.col);
  };
  if (top) {
    placed.add(top);
    for (const k of children(top)) place(k, 0);
  }
  for (const c of chains) if (!c.main && !placed.has(c)) place(c, 0); // (loops, none above)

  // The main branch's colour, then the palette's in order (tokens.css).
  let n = 0;
  for (const c of chains) c.color = c.main ? 0 : c.name && laneOf ? laneOf(c.name) : (n++ % 12) + 1;

  const edges: Edge[] = [];
  for (const v of versions) {
    v.parents.forEach((p, i) => {
      if (!byID.has(p)) return;
      const same = chainOf.get(v.id) === chainOf.get(p);
      edges.push({ from: v.id, to: p, kind: same && i === 0 ? "line" : i === 0 ? "fork" : "merge" });
    });
  }
  return { chains, chainOf, row, edges, left: 0, right };
}
