// The open tabs at the top of the window: one ordered list for every team.
// A tab is a project's (its key: its folder, or its id while it isn't on
// this computer) on a team, or a new tab ("new:…", no team). A project tab
// keeps a small picture of its project, so it can be drawn while another
// team is shown; it is refreshed whenever its team is. App.svelte keeps the
// list; these are its steps.

export const isNew = (k: string) => k.startsWith("new:");

/** What a tab shows of its project while its team isn't the one shown. */
export type TabSnap = { id: string; root: string; name: string; icon: string; color: string; status: string };
export type TabRef = { team: string; key: string; p?: TabSnap };

/** A tab's identity: the same project key may be on two teams. */
export const tabId = (x: { team: string; key: string }) => (isNew(x.key) ? x.key : `${x.team}\n${x.key}`);

const str = (v: unknown) => (typeof v === "string" ? v : "");
const snapFields = ["id", "root", "name", "icon", "color", "status"] as const;

/** The picture a tab keeps of a project. */
export function snapOf(p: Partial<Record<(typeof snapFields)[number], unknown>>): TabSnap {
  return { id: str(p.id), root: str(p.root), name: str(p.name), icon: str(p.icon), color: str(p.color), status: str(p.status) };
}

function readTab(v: unknown): TabRef | null {
  if (!v || typeof v !== "object") return null;
  const o = v as Record<string, unknown>;
  if (typeof o.team !== "string" || !o.team || typeof o.key !== "string" || !o.key || isNew(o.key)) return null;
  const p = o.p && typeof o.p === "object" && typeof (o.p as Record<string, unknown>).name === "string" ? snapOf(o.p) : undefined;
  return p ? { team: o.team, key: o.key, p } : { team: o.team, key: o.key };
}

/** The tabs as saved (maybe broken), with the new tabs open now. */
export function loadTabs(saved: unknown, open: TabRef[]): TabRef[] {
  const seen = new Set<string>();
  const out: TabRef[] = [];
  for (const x of [...(Array.isArray(saved) ? saved.map(readTab) : []), ...open.filter((x) => isNew(x.key))]) {
    if (!x || seen.has(tabId(x))) continue;
    seen.add(tabId(x));
    out.push(x);
  }
  return out;
}

/** A team's tabs as R3V kept them before (one list of keys per team): its
 *  projects, without pictures until the team is shown. */
export function oldTabs(team: string, saved: unknown): TabRef[] {
  const keys = Array.isArray(saved) ? saved.filter((k): k is string => typeof k === "string" && !!k && !isNew(k)) : [];
  return [...new Set(keys)].map((key) => ({ team, key }));
}

/** What to keep of them: no new tabs. */
export const toSave = (tabs: TabRef[]) => tabs.filter((x) => !isNew(x.key));

/** The pictures of a team's tabs, from what it lists now (by key); the same
 *  list back when nothing changed. */
export function refreshSnaps(tabs: TabRef[], team: string, known: Map<string, TabSnap>): TabRef[] {
  let changed = false;
  const out = tabs.map((x) => {
    const p = x.team === team ? known.get(x.key) : undefined;
    if (!p || (x.p && snapFields.every((f) => x.p![f] === p[f]))) return x;
    changed = true;
    return { ...x, p };
  });
  return changed ? out : tabs;
}

/** A project's key changed on a team (downloaded, found elsewhere,
 *  unlinked): its tab keeps its place (and a tab already open under the new
 *  key goes). */
export function renameTab(tabs: TabRef[], team: string, from: string, to: string): TabRef[] {
  const at = tabs.findIndex((x) => x.team === team && x.key === from);
  if (from === to || at < 0) return tabs;
  return tabs.filter((x) => !(x.team === team && x.key === to)).map((x) => (x.team === team && x.key === from ? { ...x, key: to } : x));
}

/** Drops the tabs of teams no longer on this computer and, when the shown
 *  team's list is whole (known), of projects it no longer has. New tabs
 *  stay. */
export function pruneTabs(tabs: TabRef[], teams: Set<string>, team: string, known: Set<string> | null): TabRef[] {
  return tabs.filter((x) => isNew(x.key) || (teams.has(x.team) && !(known && x.team === team && !known.has(x.key))));
}

/** How many teams the project tabs are on. */
export const teamsOf = (tabs: TabRef[]) => new Set(tabs.filter((x) => !isNew(x.key)).map((x) => x.team)).size;

/** Closes id; next is the tab to show if it was the one shown: the one
 *  after it, else the one before, else none. */
export function closeTab(ids: string[], id: string): { ids: string[]; next: string } {
  const at = ids.indexOf(id);
  const rest = ids.filter((k) => k !== id);
  return { ids: rest, next: at < 0 ? "" : rest[Math.min(at, rest.length - 1)] ?? "" };
}

/** Moves the tab id to place `to` among the shown tabs (ids, in order; `to`
 *  may be shown.length for the end). Tabs not shown keep after them. */
export function moveTab(tabs: TabRef[], shown: string[], id: string, to: number): TabRef[] {
  const from = shown.indexOf(id);
  if (from < 0) return tabs;
  if (to > from) to--;
  if (to === from) return tabs;
  const order = shown.filter((k) => k !== id);
  order.splice(Math.max(0, Math.min(to, order.length)), 0, id);
  const byId = new Map(tabs.map((x) => [tabId(x), x]));
  return [...order.flatMap((k) => byId.get(k) ?? []), ...tabs.filter((x) => !shown.includes(tabId(x)))];
}
