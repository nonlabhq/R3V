import type { SectionView } from "./api";
import { readOrder, sortByOrder } from "./order";

// Sections (docs/design/sections.md): the team's named groups of projects,
// a project in at most one; the ungrouped ones come last. Which project is
// in which is the team's; the sections' order is each person's, dragged in
// the left panel and kept on this computer (each team its own), as the
// order of the projects is. The left panel and the team's home group the
// same way, and a section folded in one is folded in the other (this
// computer only).

const FOLD_KEY = "r3v.sectionsFolded";
const orderKey = (team: string) => `r3v.sectionOrder:${team}`;

// The orders set here since the app started, by team (the rest read when
// asked for).
export const sectionOrders = $state<Record<string, string[]>>({});

function savedOrder(team: string): string[] {
  try {
    return readOrder(JSON.parse(localStorage.getItem(orderKey(team)) ?? "[]"));
  } catch {
    return [];
  }
}

/** orderedSections puts a team's sections in this person's order; those
 * never placed (new ones) come after, in the team's record's order. */
export function orderedSections(team: string, sections: SectionView[]): SectionView[] {
  return sortByOrder(sections, (s) => s.id, sectionOrders[team] ?? savedOrder(team));
}

/** setSectionOrder keeps a team's sections' order (their ids) on this
 * computer. */
export function setSectionOrder(team: string, ids: string[]) {
  sectionOrders[team] = ids;
  try { localStorage.setItem(orderKey(team), JSON.stringify(ids)); } catch { /* not kept */ }
}

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

// The team's home packs its sections in one grid of cards: each section a
// block as wide as its cards need (one column up to the whole row), the
// blocks in order, side by side while they fit. A section with more than
// two rows of cards shows two, and a toggle for the rest (kept here).
const ALL_KEY = "r3v.sectionsShowAll";

/** The sections showing all their cards, by id. */
export const sectionShowAll = $state<Record<string, boolean>>((() => {
  try {
    const v = JSON.parse(localStorage.getItem(ALL_KEY) ?? "{}");
    return v && typeof v === "object" && !Array.isArray(v) ? v : {};
  } catch {
    return {};
  }
})());

export function toggleShowAll(id: string) {
  sectionShowAll[id] = !sectionShowAll[id];
  try { localStorage.setItem(ALL_KEY, JSON.stringify(sectionShowAll)); } catch { /* not kept */ }
}

/** How many columns of cards (at least min px wide, gap apart) fit in width. */
export function columnsFor(width: number, min = 240, gap = 12): number {
  return Math.max(1, Math.floor((width + gap) / (min + gap)));
}

/** How many columns a block of count cards spans: what it needs, one at
 * least, the row at most. */
export function blockSpan(count: number, cols: number): number {
  return Math.max(1, Math.min(count, cols));
}

/** How many of a block's cards show: two rows' worth, unless all are asked
 * for (or there are no more). */
export function cardsShown(count: number, cols: number, all: boolean): number {
  return all ? count : Math.min(count, 2 * cols);
}

export type Group<T> = { id: string; name: string; items: T[] };

/** groupBySection puts items in groups: each section in the order given,
 * then the ungrouped (id "", always there) last. One whose section isn't
 * there (any more) is ungrouped. */
export function groupBySection<T>(items: T[], sections: SectionView[], sectionOf: (x: T) => string): Group<T>[] {
  const known = new Set(sections.map((s) => s.id));
  const of = (x: T) => { const s = sectionOf(x); return known.has(s) ? s : ""; };
  return [...sections.map((s) => ({ id: s.id, name: s.name, items: items.filter((x) => of(x) === s.id) })),
    { id: "", name: "", items: items.filter((x) => of(x) === "") }];
}
