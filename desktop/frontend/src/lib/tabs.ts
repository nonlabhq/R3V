// The open tabs at the top of the window, as an ordered list of keys: a
// project's (its folder, or its id while it isn't on this computer) or a new
// tab's ("new:…"). App.svelte keeps the list per team; these are its steps.

export const isNew = (k: string) => k.startsWith("new:");

/** The tabs a team had (as saved, maybe broken), with the new tabs open now. */
export function loadTabs(saved: unknown, open: string[]): string[] {
  const keys = Array.isArray(saved) ? saved.filter((k): k is string => typeof k === "string" && !isNew(k)) : [];
  return [...new Set([...keys, ...open.filter(isNew)])];
}

/** What to keep of them: no new tabs. */
export const toSave = (keys: string[]) => keys.filter((k) => !isNew(k));

/** A project's key changed (downloaded, found elsewhere, unlinked): its tab
 *  keeps its place (and a tab already open under the new key goes). */
export function renameTab(keys: string[], from: string, to: string): string[] {
  if (from === to || !keys.includes(from)) return keys;
  return keys.filter((k) => k !== to).map((k) => (k === from ? to : k));
}

/** Drops the tabs of projects the team no longer has (new tabs stay). */
export function pruneTabs(keys: string[], known: Set<string>): string[] {
  return keys.filter((k) => isNew(k) || known.has(k));
}

/** Closes key; next is the tab to show if it was the one shown: the one
 *  after it, else the one before, else none. */
export function closeTab(keys: string[], key: string): { keys: string[]; next: string } {
  const at = keys.indexOf(key);
  const rest = keys.filter((k) => k !== key);
  return { keys: rest, next: at < 0 ? "" : rest[Math.min(at, rest.length - 1)] ?? "" };
}

/** Moves key to place `to` among the shown tabs (index into `shown`, the
 *  tabs drawn, in order; `to` may be shown.length for the end). Tabs not
 *  shown keep after them. */
export function moveTab(keys: string[], shown: string[], key: string, to: number): string[] {
  const from = shown.indexOf(key);
  if (from < 0) return keys;
  if (to > from) to--;
  if (to === from) return keys;
  const order = shown.filter((k) => k !== key);
  order.splice(Math.max(0, Math.min(to, order.length)), 0, key);
  return [...order, ...keys.filter((k) => !shown.includes(k))];
}
