// How the app's side-by-side panes share their width: a fraction per split
// (so a window made bigger or smaller keeps the same proportions),
// remembered on this computer. Splitter.svelte moves them.

const KEY = "r3v.splits";

export const splits = $state<Record<string, number>>((() => {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? "{}");
    return v && typeof v === "object" && !Array.isArray(v) ? v : {};
  } catch { return {}; }
})());

export function saveSplits() {
  try { localStorage.setItem(KEY, JSON.stringify(splits)); } catch { /* not remembered */ }
}

/** The left pane's width in px: the split's fraction of width, kept within
 *  the panes' smallest widths. */
export function splitPx(key: string, def: number, width: number, minLeft: number, minRight: number): number {
  const f = splits[key] ?? def;
  return Math.round(Math.min(Math.max(minLeft, f * width), Math.max(minLeft, width - minRight)));
}
