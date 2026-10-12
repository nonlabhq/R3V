// A project's workspaces as last read, kept in the background: the
// Workspaces tab shows them at once and reads them again (reading them
// scans every workspace's files and asks the team, which takes a while).
import { api, type AgentWorkspace } from "./api";

const lists = $state<Record<string, AgentWorkspace[]>>({});
const reading = new Map<string, Promise<AgentWorkspace[]>>();

/** The workspaces of the project in root as last read (null: not yet). */
export function cachedWorkspaces(root: string): AgentWorkspace[] | null {
  return lists[root] ?? null;
}

/** Reads the project's workspaces again (one read at a time per project). */
export function refreshWorkspaces(root: string): Promise<AgentWorkspace[]> {
  const going = reading.get(root);
  if (going) return going;
  const p = api.AgentWorkspaces(root)
    .then((l) => (lists[root] = l ?? []))
    .finally(() => reading.delete(root));
  reading.set(root, p);
  return p;
}

/** Forgets what was read (tests). */
export function forgetWorkspaces() {
  for (const k of Object.keys(lists)) delete lists[k];
  reading.clear();
}
