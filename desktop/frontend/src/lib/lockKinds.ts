// File locks in words: the kinds of file a team auto-locks (ids from Go's
// profile.LockKinds), and what a project's .r3v.yaml makes of the team's
// settings (file_locks:), now and between two states of the file.
import { t } from "./i18n.svelte";

/** A kind of file that can lock by itself, in words. */
export function lockKindName(id: string): string {
  return ({
    unreal: t("Unreal levels and assets"), unity: t("Unity scenes, prefabs and assets"),
    godot: t("Godot binary scenes and resources"), blender: t("Blender files"),
    "source-art": t("Source art (Photoshop, Krita, Substance, Maya, 3ds Max, Cinema 4D)"),
    "godot-text": t("Godot text scenes and resources"), images: t("Images"), models: t("3D models"), audio: t("Audio"), video: t("Video"),
  } as Record<string, string>)[id] ?? id;
}

/** A pattern in words: "everything under Content/Maps/", ".uasset files". */
export function patternText(p: string): string {
  const s = p.trim().replace(/^\//, "");
  const dir = s.match(/^(.*?)\/(\*\*(\/\*)?)?$/);
  if (dir && dir[1] && !/[*?[]/.test(dir[1])) return t("everything under {folder}", { folder: dir[1] + "/" });
  const ext = s.match(/^\*(\.[A-Za-z0-9_-]+)$/);
  if (ext) return t("{ext} files", { ext: ext[1] });
  return s;
}

export type TeamLockSide = { on: boolean; kinds: string[] };
export type FileLocks = { disabled: boolean; autoOff: boolean; add: string[]; remove: string[] };

export const fileLocksSet = (f: FileLocks | undefined | null) =>
  !!f && (f.disabled || f.autoOff || f.add.length > 0 || f.remove.length > 0);

/** What locks do in the project, line by line: the team's settings with
 * the project's own on top (it only narrows them). Nothing when this R3V
 * has no locks (team null), or when they're off and the file says nothing. */
export function lockingLines(team: TeamLockSide | null | undefined, f: FileLocks | undefined | null): string[] {
  if (!team) return [];
  const set = fileLocksSet(f);
  if (!team.on) return set ? [t("The team hasn't turned file locking on: this file's lock settings do nothing until it does.")] : [];
  if (f?.disabled) return [t("File locking: off for this project")];
  if (f?.autoOff || (!team.kinds.length && !f?.add.length)) return [t("Manual locks only: files lock when someone locks them (right-click › Lock)")];
  const out: string[] = [];
  if (team.kinds.length) out.push(t("Auto-lock (the team's): {kinds}", { kinds: team.kinds.map(lockKindName).join(", ") }));
  if (f?.add.length) out.push(t("Auto-lock also: {what}", { what: f.add.map(patternText).join(", ") }));
  if (f?.remove.length) out.push(t("Not auto-locked: {what}", { what: f.remove.map(patternText).join(", ") }));
  return out;
}

/** What changed in a file's lock settings, in words. */
export function fileLocksChanges(now: FileLocks | undefined | null, before: FileLocks | undefined | null) {
  const out: { kind: "add" | "del" | "mod"; text: string }[] = [];
  const a = now ?? { disabled: false, autoOff: false, add: [], remove: [] };
  const b = before ?? { disabled: false, autoOff: false, add: [], remove: [] };
  if (a.disabled !== b.disabled) out.push(a.disabled ? { kind: "add", text: t("File locking: off for this project") }
    : { kind: "del", text: t("File locking: no longer off for this project") });
  if (a.autoOff !== b.autoOff) out.push(a.autoOff ? { kind: "add", text: t("Manual locks only (no auto-lock)") }
    : { kind: "del", text: t("Auto-lock no longer off") });
  for (const p of a.add) if (!b.add.includes(p)) out.push({ kind: "add", text: t("Auto-lock also: {what}", { what: patternText(p) }) });
  for (const p of b.add) if (!a.add.includes(p)) out.push({ kind: "del", text: t("No longer auto-locked too: {what}", { what: patternText(p) }) });
  for (const p of a.remove) if (!b.remove.includes(p)) out.push({ kind: "add", text: t("Not auto-locked: {what}", { what: patternText(p) }) });
  for (const p of b.remove) if (!a.remove.includes(p)) out.push({ kind: "del", text: t("Auto-locked again: {what}", { what: patternText(p) }) });
  return out;
}
