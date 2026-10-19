// What "Open in" remembers for a project, on this computer only (never the
// team's, nor in .r3v.yaml): the program last picked to open its folder
// with, and the programs chosen for it with "Choose a program…" (their
// .exe paths).
export type OpenWith = { last?: { id: string; name: string }; chosen: string[] };

const key = (root: string) => `r3v.openWith:${root}`;

export function readOpenWith(root: string): OpenWith {
  try {
    const v = JSON.parse(localStorage.getItem(key(root)) ?? "{}");
    const chosen = Array.isArray(v?.chosen) ? v.chosen.filter((c: unknown) => typeof c === "string") : [];
    const last = typeof v?.last?.id === "string" && typeof v?.last?.name === "string" ? { id: v.last.id, name: v.last.name } : undefined;
    return { last, chosen };
  } catch {
    return { chosen: [] };
  }
}

export function keepOpenWith(root: string, v: OpenWith) {
  try { localStorage.setItem(key(root), JSON.stringify(v)); } catch { /* not remembered */ }
}

/** A chosen program's name: its file's, without .exe. */
export function programName(path: string): string {
  return (path.split(/[\\/]/).pop() ?? path).replace(/\.exe$/i, "");
}
