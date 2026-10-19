import type { SectionView } from "./api";

// Sections (docs/design/sections.md; Nightly): the team's named groups of
// projects, in the team's order, a project in at most one; the ungrouped
// ones come first. The left panel and the team's home group the same way,
// and a section folded in one is folded in the other (this computer only).

const FOLD_KEY = "r3v.sectionsFolded";

function readFolds(): Record<string, boolean> {
  try {
    const v = JSON.parse(localStorage.getItem(FOLD_KEY) ?? "{}");
    return v && typeof v === "object" && !Array.isArray(v) ? v : {};
  } catch {
    return {};
  }
}

/** The sections folded, by id. */
export const sectionFolds = $state<Record<string, boolean>>(readFolds());

export function foldSection(id: string) {
  sectionFolds[id] = !sectionFolds[id];
  try { localStorage.setItem(FOLD_KEY, JSON.stringify(sectionFolds)); } catch { /* not kept */ }
}

export type Group<T> = { id: string; name: string; items: T[] };

/** groupBySection puts items in groups: the ungrouped (id "") first, then
 * each section in the team's order. One whose section isn't there (any
 * more) is ungrouped. */
export function groupBySection<T>(items: T[], sections: SectionView[], sectionOf: (x: T) => string): Group<T>[] {
  const known = new Set(sections.map((s) => s.id));
  const of = (x: T) => { const s = sectionOf(x); return known.has(s) ? s : ""; };
  return [{ id: "", name: "", items: items.filter((x) => of(x) === "") },
    ...sections.map((s) => ({ id: s.id, name: s.name, items: items.filter((x) => of(x) === s.id) }))];
}
