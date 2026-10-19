import { t } from "./i18n.svelte";
import { api, errorText, type Program } from "./api";
import { toast } from "./notify.svelte";

// "Open the folder in": what the header's "Open in" menu and the sidebar's
// right-click menu both offer. R3V keeps, on this computer only, the
// programs chosen for each project and the one last picked (the Go side:
// it runs only those, or the ones it detected).

/** A program as the menus name it. */
export const programLabel = (p: { id: string; name: string }) =>
  p.id === "explorer" ? t("Explorer") : p.name === "Terminal" ? t("Terminal") : p.name;

/** What the folder can be opened with (none when it can't be asked: said). */
export async function loadPrograms(root: string): Promise<Program[]> {
  try {
    return (await api.OpenPrograms(root)) ?? [];
  } catch (e) {
    toast(errorText(e), "error");
    return [];
  }
}

/** Opens the folder with p; false when it couldn't (said). */
export async function openFolderWith(root: string, p: { id: string }): Promise<boolean> {
  try {
    await api.OpenFolderWith(root, p.id);
    return true;
  } catch (e) {
    toast(errorText(e), "error");
    return false;
  }
}

/** Asks for a program, keeps it for the project and opens the folder with
 *  it; null when cancelled or it couldn't. */
export async function chooseProgram(root: string): Promise<Program | null> {
  let p: Program | null;
  try {
    p = await api.ChooseProgram(root, t("Choose a program to open the project with"));
  } catch (e) {
    toast(errorText(e), "error");
    return null;
  }
  return p && (await openFolderWith(root, p)) ? p : null;
}

const OLD_KEY = "r3v.openWith:";

/** Before R3V kept them, the page did (localStorage, per project): sent to
 *  R3V once, which keeps what is still there, then forgotten here. */
export async function migrateOpenWith() {
  const kept: Record<string, { last: { id: string; name: string } | null; chosen: string[] }> = {};
  const keys: string[] = [];
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (!k?.startsWith(OLD_KEY)) continue;
      keys.push(k);
      try {
        const v = JSON.parse(localStorage.getItem(k) ?? "{}");
        const chosen = Array.isArray(v?.chosen) ? v.chosen.filter((c: unknown) => typeof c === "string") : [];
        const last = typeof v?.last?.id === "string" && typeof v?.last?.name === "string" ? { id: v.last.id, name: v.last.name } : null;
        kept[k.slice(OLD_KEY.length)] = { last, chosen };
      } catch { /* broken: forgotten */ }
    }
  } catch {
    return; // no storage
  }
  if (!keys.length) return;
  try {
    await api.ImportOpenWith(kept as never);
  } catch {
    return; // tried again next start
  }
  for (const k of keys) try { localStorage.removeItem(k); } catch { /* left */ }
}
