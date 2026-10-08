// The Files tab's model: the folder tree (folders only, with how many
// changed files each holds), a folder's files, the look a folder starts
// with, the program a file belongs to, and selecting several files the way
// a file explorer does.
import type { ProjectFile } from "./api";

export type Folder = {
  path: string; // "" the project folder
  name: string;
  folders: Folder[]; // sorted by name
  changed: number; // changed files inside, at any depth
  files: number; // files right in it
};

export const isChanged = (f: ProjectFile) => f.status !== "unchanged" && f.status !== "ignored";
export const baseName = (p: string) => p.slice(p.lastIndexOf("/") + 1);
export const dirOf = (p: string) => (p.includes("/") ? p.slice(0, p.lastIndexOf("/")) : "");

const byName = (a: string, b: string) => a.localeCompare(b, undefined, { numeric: true, sensitivity: "base" });

/** The folders of the project (from its files), the project folder at the top. */
export function folderTree(files: ProjectFile[], name = ""): Folder {
  const top: Folder = { path: "", name, folders: [], changed: 0, files: 0 };
  const index = new Map<string, Folder>([["", top]]);
  const get = (path: string): Folder => {
    let f = index.get(path);
    if (f) return f;
    const parent = get(dirOf(path));
    f = { path, name: baseName(path), folders: [], changed: 0, files: 0 };
    parent.folders.push(f);
    index.set(path, f);
    return f;
  };
  for (const f of files) {
    const dir = dirOf(f.path);
    get(dir).files++;
    if (isChanged(f)) {
      for (let d: string | null = dir; d !== null; d = d === "" ? null : dirOf(d)) get(d).changed++;
    }
  }
  const sort = (f: Folder) => { f.folders.sort((a, b) => byName(a.name, b.name)); f.folders.forEach(sort); };
  sort(top);
  return top;
}

/** The folder at path, or null when there is none (any more). */
export function findFolder(top: Folder, path: string): Folder | null {
  if (path === "") return top;
  let at: Folder | undefined = top;
  for (const part of path.split("/")) {
    at = at?.folders.find((f) => f.name === part);
    if (!at) return null;
  }
  return at;
}

/** The files right in a folder. */
export const filesIn = (files: ProjectFile[], dir: string) => files.filter((f) => dirOf(f.path) === dir);

// The programs files are made with: a short badge, and the name "Open in"
// says (only for files a program owns; others open in what Windows picks).
const tools: [RegExp, string, string][] = [
  [/\.als$/i, "Lv", "Live"], [/\.(alc|adg|adv|agr|ams)$/i, "Lv", "Live"],
  [/\.(unity|prefab|asset|mat|anim|controller)$/i, "Un", "Unity"],
  [/\.(uasset|umap|uproject)$/i, "Ue", "Unreal"],
  [/\.(tscn|tres|godot|gd)$/i, "Gd", "Godot"],
  [/\.blend$/i, "Bl", "Blender"], [/\.psd$/i, "Ps", "Photoshop"], [/\.c4d$/i, "C4", "Cinema 4D"],
  [/\.m[ab]$/i, "My", "Maya"], [/\.hip(nc|lc)?$/i, "Hd", "Houdini"], [/\.kra$/i, "Kr", "Krita"],
  [/\.(afphoto|afdesign)$/i, "Af", "Affinity"],
];

/** The file's program badge (else its extension) and the program's name ("" when none). */
export function toolOf(path: string): { badge: string; name: string } {
  const hit = tools.find(([re]) => re.test(path));
  if (hit) return { badge: hit[1], name: hit[2] };
  const name = baseName(path);
  const ext = name.includes(".") ? name.slice(name.lastIndexOf(".") + 1) : "";
  return { badge: ext.slice(0, 3).toUpperCase() || "—", name: "" };
}

export type Selection = { picked: string[]; anchor: string };

/** A click on key: alone, Ctrl toggles it, Shift picks the range from the anchor. */
export function clickPick(sel: Selection, order: string[], key: string, mods: { ctrl?: boolean; shift?: boolean }): Selection {
  if (mods.shift && sel.anchor && order.includes(sel.anchor)) {
    const a = order.indexOf(sel.anchor), b = order.indexOf(key);
    const range = order.slice(Math.min(a, b), Math.max(a, b) + 1);
    const picked = mods.ctrl ? [...new Set([...sel.picked, ...range])] : range;
    return { picked, anchor: sel.anchor };
  }
  if (mods.ctrl) {
    const picked = sel.picked.includes(key) ? sel.picked.filter((k) => k !== key) : [...sel.picked, key];
    return { picked, anchor: key };
  }
  return { picked: [key], anchor: key };
}

export type Rect = { left: number; top: number; right: number; bottom: number };

/** The keys whose boxes the marquee touches (with Ctrl, added to what was picked). */
export function marqueePick(box: Rect, items: { key: string; rect: Rect }[], before: string[] = []): string[] {
  const hit = items.filter(({ rect: r }) => r.left < box.right && r.right > box.left && r.top < box.bottom && r.bottom > box.top)
    .map((i) => i.key);
  return before.length ? [...new Set([...before, ...hit])] : hit;
}
