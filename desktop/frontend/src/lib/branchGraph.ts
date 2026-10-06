// The Overview's branch graph: time runs down (newest at the top, one row
// per version), the main branch is the middle column, other branches grow
// to the sides: the one you are on to the right, the others to the left,
// those that split off earlier nearer the middle. A column is used again
// once a branch above has ended.

export type GraphVersion = { id: string; parents: string[] };

export type Chain = {
  name: string;   // the branch ("" for versions merged in from a branch gone since)
  tip: string;    // its newest version here
  ids: string[];  // its versions, newest first (first parents from the tip)
  col: number;    // 0 the middle; < 0 left, > 0 right
  main: boolean;
  mine: boolean;  // the branch you are on (when it isn't main)
  color: number;  // lane colour index
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

// branches: name and newest version of each; current: the branch you are on.
// mainName: the middle column (the team's main branch).
export function branchGraph(versions: GraphVersion[], branches: { name: string; latest: string }[],
  current: string, mainName = "main", head = ""): BranchGraph {
  const byID = new Map(versions.map((v) => [v.id, v]));
  const row = new Map(versions.map((v, i) => [v.id, i]));
  const chainOf = new Map<string, Chain>();
  const chains: Chain[] = [];

  function walk(name: string, tip: string, main: boolean) {
    if (!byID.has(tip) || chainOf.has(tip)) return;
    const c: Chain = { name, tip, ids: [], col: 0, main, mine: !main && name === current, color: 0 };
    for (let id: string | undefined = tip; id && byID.has(id) && !chainOf.has(id); id = byID.get(id)!.parents[0]) {
      chainOf.set(id, c);
      c.ids.push(id);
    }
    chains.push(c);
  }
  const tipOf = (name: string) => branches.find((b) => b.name === name)?.latest ?? "";
  const mainBranch = branches.some((b) => b.name === mainName) ? mainName : current;
  walk(mainBranch, tipOf(mainBranch), true);
  if (current !== mainBranch) walk(current, tipOf(current), false);
  if (head && !chainOf.has(head)) walk(current, head, current === mainBranch);
  for (const b of branches) walk(b.name, b.latest, false);
  // Whatever is left came in through merges: its own lines.
  for (const v of versions) walk("", v.id, false);
  // No main branch in this history: the first line is the middle.
  if (!chains.some((c) => c.main) && chains.length) chains[0].main = true;

  // Columns, side by side: from the earliest split, the nearest free column
  // (free from a row above the branch's top, for its label, to where it split).
  const span = (c: Chain) => {
    const top = row.get(c.tip)!;
    const oldest = byID.get(c.ids[c.ids.length - 1])!;
    const from = oldest.parents[0] !== undefined && row.has(oldest.parents[0]) ? row.get(oldest.parents[0])! : versions.length;
    return { top: top - 1, bottom: from };
  };
  const used = new Map<number, { top: number; bottom: number }[]>();
  const sides = [...chains.filter((c) => !c.main)].sort((a, b) => span(b).bottom - span(a).bottom);
  let left = 0, right = 0;
  for (const c of sides) {
    const s = span(c);
    const dir = c.mine ? 1 : -1;
    for (let col = dir; ; col += dir) {
      const taken = used.get(col) ?? [];
      if (taken.every((t) => s.bottom < t.top || s.top > t.bottom)) {
        taken.push(s);
        used.set(col, taken);
        c.col = col;
        break;
      }
    }
    left = Math.max(left, -c.col);
    right = Math.max(right, c.col);
  }
  let n = 1;
  for (const c of chains) c.color = c.main ? 0 : n++ % 5 || 1;

  const edges: Edge[] = [];
  for (const v of versions) {
    v.parents.forEach((p, i) => {
      if (!byID.has(p)) return;
      const same = chainOf.get(v.id) === chainOf.get(p);
      edges.push({ from: v.id, to: p, kind: same ? "line" : i === 0 ? "fork" : "merge" });
    });
  }
  return { chains, chainOf, row, edges, left, right };
}

/** A title cut to n characters, with an ellipsis. */
export function short(text: string, n = 24): string {
  const t = text.trim();
  return [...t].length > n ? [...t].slice(0, n - 1).join("").trimEnd() + "…" : t;
}
