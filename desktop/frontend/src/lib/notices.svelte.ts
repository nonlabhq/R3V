import type { State } from "./api";

// What a project's page has to say that stays until it's dealt with (the
// title bar's notice slot shows them: NoticeSlot, ProjectBanners). Each is
// a key, most pressing first.

// Put aside by the user: the sync-service warning once understood (per
// project, kept), missing samples said "Later" to (until their count changes).
export const putAside = $state({ cloud: {} as Record<string, boolean>, samples: {} as Record<string, number> });

export function cloudUnderstood(root: string): boolean {
  if (putAside.cloud[root]) return true;
  try { return localStorage.getItem(`r3v.cloudOk:${root}`) === "1"; } catch { return false; }
}

export function understandCloud(root: string) {
  putAside.cloud[root] = true;
  try { localStorage.setItem(`r3v.cloudOk:${root}`, "1"); } catch { /* shown again next time */ }
}

// These stop the work at hand: they come before the team's new versions.
export const pressing = new Set(["loadError", "unfinished", "notDownloaded", "branchGone", "older"]);

export function issuesOf(st: State, o: { working: boolean; restorable: number; loadError: string; locks: boolean }): string[] {
  const k: string[] = [];
  const team = !!st.remoteUrl;
  if (o.loadError) k.push("loadError");
  if (st.unfinished) k.push("unfinished");
  // A download that didn't finish (the connection lost, R3V closed): no
  // version here yet, the team has some.
  if (team && !st.head && st.incoming.length > 0 && !o.working) k.push("notDownloaded");
  if (st.branchGone && !o.working) k.push("branchGone");
  if (st.olderVersion) k.push("older");
  if (st.rules.error) k.push("rulesError");
  if (o.restorable && putAside.samples[st.root] !== o.restorable) k.push("samples");
  for (const s of st.rules.suggestions) k.push(`rule:${s.folder}\n${s.preset}`);
  if (st.cloudFolder && !cloudUnderstood(st.root)) k.push("cloud");
  if (team && !st.head && !st.incoming.length && !o.working) k.push("notShared");
  else if (team && st.head && st.unshared && !o.working) k.push("unshared");
  if (st.inUse?.length) k.push("inUse");
  if (st.locksOffer && o.locks && !o.working) k.push("locks");
  return k;
}
