import type { BranchGraph } from "./branchGraph";
import type { GraphVersion } from "./types";

// Segments: agents commit often (hundreds of versions in an afternoon), so
// a run of an agent's versions on one branch's line is drawn as one node
// ("lane/render · 47 versions · 2h"), opened by a click. What tells the
// story of the project always stays a node of its own: where a branch
// starts and what it branches from or is merged from, merges, milestones,
// checked versions, a person's versions, each line's newest and oldest,
// and what the caller keeps (the version you are on, the one picked…).

export type Segment = {
  id: string;     // "seg:" + key: its row in the graph
  key: string;    // its oldest version's id (stays as the run grows at the top)
  ids: string[];  // its versions, newest first
  branch: string; // the branch they were made on
  from: string;   // the oldest one's time, and the newest one's
  to: string;
  files?: number; // files changed, summed over its versions (when the versions say)
};

// (the fewest versions worth a segment: two would hide only one row)
export const MIN_RUN = 3;

/** The runs of versions that can be one node each, in the graph g drawn
 * from versions. keep: versions that stay their own nodes. */
export function segmentRuns(versions: GraphVersion[], g: BranchGraph, keep: Set<string>, min = MIN_RUN): Segment[] {
  const byID = new Map(versions.map((v) => [v.id, v]));
  // Versions a line of another branch comes out of, or is merged from.
  const joints = new Set<string>();
  for (const v of versions) {
    v.parents.forEach((p, i) => { if (i > 0 || g.chainOf.get(p) !== g.chainOf.get(v.id)) joints.add(p); });
  }
  const out: Segment[] = [];
  for (const c of g.chains) {
    const last = c.ids[c.ids.length - 1];
    const stays = (v: GraphVersion) => keep.has(v.id) || joints.has(v.id) || v.parents.length > 1 || !v.actor
      || !!v.check || v.id === c.tip || v.id === last;
    let run: GraphVersion[] = [];
    const flush = () => {
      if (run.length >= min) {
        const files = run.some((v) => v.files !== undefined) ? run.reduce((n, v) => n + (v.files ?? 0), 0) : undefined;
        const key = run[run.length - 1].id;
        out.push({ id: "seg:" + key, key, ids: run.map((v) => v.id), branch: run[0].branch, from: run[run.length - 1].time,
          to: run[0].time, ...(files !== undefined ? { files } : {}) });
      }
      run = [];
    };
    for (const id of c.ids) {
      const v = byID.get(id);
      if (!v || stays(v)) { flush(); continue; }
      const prev = run[run.length - 1];
      // (one line, one step: a run is cut where the agent's step changes)
      if (prev && (prev.parents[0] !== id || (prev.step ?? "") !== (v.step ?? ""))) flush();
      run.push(v);
    }
    flush();
  }
  return out;
}

/** What the graph lays out: the versions, each closed segment one row in
 * place of its versions (at its newest one's row: what comes out of it is
 * above, what it comes from below), parents pointing at it. */
export function collapse<V extends GraphVersion>(versions: V[], segments: Segment[], open: Set<string>):
  { rows: { id: string; parents: string[]; branch: string }[]; segs: Map<string, Segment> } {
  const segs = new Map<string, Segment>();
  const into = new Map<string, Segment>();
  for (const s of segments) {
    if (open.has(s.key)) continue;
    segs.set(s.id, s);
    for (const id of s.ids) into.set(id, s);
  }
  if (!segs.size) return { rows: versions, segs };
  const parentsOf = new Map(versions.filter((v) => into.has(v.id)).map((v) => [v.id, v.parents]));
  const rows: { id: string; parents: string[]; branch: string }[] = [];
  for (const v of versions) {
    const s = into.get(v.id);
    if (s) {
      if (s.ids[0] === v.id) rows.push({ id: s.id, parents: parentsOf.get(s.key) ?? [], branch: s.branch });
      continue;
    }
    rows.push(v.parents.some((p) => into.has(p)) ? { ...v, parents: v.parents.map((p) => into.get(p)?.id ?? p) } : v);
  }
  return { rows, segs };
}

/** How long a segment took, short ("2h", "2 小時"). */
export function spanText(from: string, to: string, lang: string): string {
  const ms = Math.max(0, Date.parse(to) - Date.parse(from));
  if (isNaN(ms)) return "";
  const m = ms / 60_000;
  const [n, unit] = m < 60 ? [Math.max(1, Math.round(m)), "minute"] : m < 48 * 60 ? [Math.round(m / 60), "hour"] : [Math.round(m / 1440), "day"];
  try {
    return new Intl.NumberFormat(lang, { style: "unit", unit, unitDisplay: "narrow" }).format(n);
  } catch {
    return `${n}${unit[0]}`;
  }
}
