// A project's workspaces as last read, kept in the background and from one
// start of the app to the next: the Workspaces tab shows them at once and
// reads them again (reading them scans every workspace's files and asks the
// team, which takes a while, longer while agents are at work).
import { api, type AgentWorkspace } from "./api";

const KEY = "r3v.workspaces";
function stored(): Record<string, AgentWorkspace[]> {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? "{}");
    return v && typeof v === "object" ? v : {};
  } catch {
    return {};
  }
}

const lists = $state<Record<string, AgentWorkspace[]>>(stored());
const reading = new Map<string, Promise<AgentWorkspace[]>>();
const busy = $state<Record<string, boolean>>({});

/** The workspaces of the project in root as last read (null: not yet). */
export function cachedWorkspaces(root: string): AgentWorkspace[] | null {
  return lists[root] ?? null;
}

/** Whether they are being read again now (the tab's spinner). */
export function readingWorkspaces(root: string): boolean {
  return !!busy[root];
}

/** Reads the project's workspaces again (one read at a time per project). */
export function refreshWorkspaces(root: string): Promise<AgentWorkspace[]> {
  const going = reading.get(root);
  if (going) return going;
  busy[root] = true;
  const p = api.AgentWorkspaces(root)
    .then((l) => {
      lists[root] = l ?? [];
      try { localStorage.setItem(KEY, JSON.stringify(lists)); } catch { /* read again next time */ }
      return lists[root];
    })
    .finally(() => { reading.delete(root); delete busy[root]; });
  reading.set(root, p);
  return p;
}

// Projects whose Workspaces tab was open since the app started: the tab is
// remembered for them; after a start, a project opens on its Versions.
export const workspacesSeen = new Set<string>();

/** Forgets what was read (tests). */
export function forgetWorkspaces() {
  for (const k of Object.keys(lists)) delete lists[k];
  reading.clear();
  workspacesSeen.clear();
  try { localStorage.removeItem(KEY); } catch { /* nothing kept */ }
}
