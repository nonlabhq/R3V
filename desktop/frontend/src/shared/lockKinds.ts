// The kinds of file a team auto-locks: Go's profile.LockKinds as they are
// (lockKinds.json, checked against Go by internal/profile's tests), and in
// words.
import { t } from "./i18n";
import kinds from "./lockKinds.json";

/** A kind of file that can lock by itself. default: auto-locked when a team turns locking on; new: a default since teams could pick their own (offered to a team that did). */
export type LockKind = { id: string; patterns: string[]; default: boolean; new?: boolean };

/** The kinds a team picks from, in the order offered. */
export const LOCK_KINDS: LockKind[] = kinds;

/** The kinds a team auto-locks when it turns locking on. */
export const defaultLockKinds = () => LOCK_KINDS.filter((k) => k.default).map((k) => k.id);

/** A kind of file that can lock by itself, in words. */
export function lockKindName(id: string): string {
  return ({
    unreal: t("Unreal levels and assets"), unity: t("Unity scenes, prefabs and assets"),
    godot: t("Godot binary scenes and resources"), blender: t("Blender files"),
    "source-art": t("Source art (Photoshop, Krita, Substance, Maya, 3ds Max, Cinema 4D)"),
    "godot-text": t("Godot text scenes and resources"), images: t("Images"), models: t("3D models"), audio: t("Audio"), video: t("Video"),
  } as Record<string, string>)[id] ?? id;
}
