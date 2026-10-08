// File locks (docs/design/locks.md) on the page: a project's locks as Go
// reads them (ProjectLocks), kept per project and read again when they
// change ("locks" events: yours, someone's notice, a share), and what they
// say in words. Off (the team's switch, the project's .r3v.yaml, a team on
// its own storage), view.on is false and nothing shows.
import { Events } from "@wailsio/runtime";
import { api, errorText } from "./api";
import { language, t } from "./i18n.svelte";
import { toast } from "./notify.svelte";
import type { LockItem, LocksView } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

export type { LockItem, LocksView };

const views = $state<Record<string, LocksView>>({});

/** A project's locks as last read (undefined before the first read). */
export const locksOf = (root: string): LocksView | undefined => views[root];

/** Reads a project's locks again. */
export async function loadLocks(root: string) {
  try {
    const v = await api.ProjectLocks(root);
    if (v) views[root] = v;
  } catch {
    // (shown as they were)
  }
}

/** The lock holding a path: the file's own, or a folder's over it. A
 * folder is asked about as "dir/". */
export function lockAt(v: LocksView | undefined, path: string): LockItem | undefined {
  if (!v?.on) return undefined;
  return v.items.find((l) => l.path === path) ?? v.items.find((l) => l.prefix && path.startsWith(l.path) && path !== l.path);
}

/** How long ago a lock was taken: "3 days ago", "5 min ago". */
export function lockAge(iso: string): string {
  const s = Math.max(0, (Date.now() - Date.parse(iso)) / 1000);
  if (isNaN(s)) return "";
  const rel = new Intl.RelativeTimeFormat(language(), { numeric: "auto", style: "long" });
  if (s < 60) return rel.format(0, "second");
  if (s < 3600) return rel.format(-Math.floor(s / 60), "minute");
  if (s < 86400) return rel.format(-Math.floor(s / 3600), "hour");
  return rel.format(-Math.floor(s / 86400), "day");
}

/** Who holds it, in words. */
export const holderName = (l: { name: string; mine?: boolean }) => (l.mine ? t("you") : l.name || t("Someone"));

/** A lock in a sentence, for its tooltip. */
export function lockText(l: LockItem): string {
  const when = lockAge(l.since);
  if (l.mine) return l.prefix ? t("You locked this folder {when}", { when }) : t("Locked by you {when}", { when });
  return l.prefix ? t("{name} locked this folder {when}: changes in it can't be shared until it's unlocked", { name: holderName(l), when })
    : t("{name} is editing this file (locked {when}): changes to it can't be shared until it's unlocked", { name: holderName(l), when });
}

type HeldEvent = { root: string; path: string; file: string; name: string; meanwhile: boolean };
type LocksEvent = { root: string; path: string; name: string; locked: boolean; shared: boolean; mine: boolean };

/** Keeps a project's locks current while its page is shown, and says at
 * once when a change can't be shared (someone else holds the file) or can
 * again (it was unlocked). changed: the paths changed here. */
export function watchLocks(root: () => string, changed: () => string[]) {
  $effect(() => {
    const r = root();
    loadLocks(r);
    const offLocks = Events.On("locks", (ev: { data: LocksEvent }) => {
      const e = ev.data;
      if (e.root !== r) return;
      const was = lockAt(views[r], e.path);
      loadLocks(r);
      // Someone else's lock on a file you changed went: it can be shared.
      if (e.path && !e.locked && !e.mine && was && !was.mine && changed().includes(e.path)) {
        toast(t("{file} is unlocked: your change can be shared", { file: baseName(e.path) }), "ok", 7000);
      }
    });
    const offHeld = Events.On("lock-held", (ev: { data: HeldEvent }) => {
      const e = ev.data;
      if (e.root !== r) return;
      const name = e.name === "Someone" ? t("Someone") : e.name;
      toast(e.meanwhile
        ? t("{name} locked {file} while you were offline: your change can't be shared until it's unlocked", { name, file: e.file })
        : t("{name} is editing {file}: your change can't be shared until it's unlocked", { name, file: e.file }), "warn", 9000);
    });
    // The team's side read again (a share, someone's version): so are the locks.
    const offTeam = Events.On("team-watch", (ev: { data: { root: string } }) => { if (ev.data.root === r) loadLocks(r); });
    return () => { offLocks(); offHeld(); offTeam(); };
  });
}

const baseName = (p: string) => p.replace(/\/$/, "").slice(p.replace(/\/$/, "").lastIndexOf("/") + 1);

/** Locks paths (a folder: "dir/"), saying what someone else holds. */
export async function lockPaths(root: string, paths: string[]) {
  try {
    const res = await api.LockFiles(root, paths);
    for (const h of res?.refused ?? []) {
      toast(t("{name} already holds {file}", { name: h.name || t("Someone"), file: baseName(h.path) }), "warn", 7000);
    }
    if (res?.locked.length) {
      toast(res.locked.length === 1 ? t("Locked {file}: your teammates see you're working on it", { file: baseName(res.locked[0]) })
        : t("Locked {n} items: your teammates see you're working on them", { n: res.locked.length }), "ok");
    }
  } catch (e) {
    toast(errorText(e), "error", 9000);
  }
  await loadLocks(root);
}

/** Frees your locks on paths. */
export async function unlockPaths(root: string, paths: string[]) {
  try {
    await api.UnlockFiles(root, paths);
  } catch (e) {
    toast(errorText(e), "error", 9000);
  }
  await loadLocks(root);
}

/** Frees someone else's lock (admins). */
export async function breakLock(root: string, path: string) {
  try {
    await api.BreakLock(root, path);
    toast(t("Unlocked {file}", { file: baseName(path) }), "ok");
  } catch (e) {
    toast(errorText(e), "error", 9000);
  }
  await loadLocks(root);
}
