// A project's rules (.r3v.yaml) for people: preset names, what a preset
// leaves out, and what changed between two states of the file. The file is
// read in Go (RulesAt), by the parser R3V follows; this only words it.
import { t } from "./i18n.svelte";
import { api, errorText } from "./api";
import { toast } from "./notify.svelte";
import { fileLocksChanges } from "./lockKinds";
import type { RulesState } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

export const presetName = (p: string) => ({ ableton: "Ableton Live", unity: "Unity", unreal: "Unreal", godot: "Godot",
  design: t("Design files"), code: t("Code"), none: t("No preset") } as Record<string, string>)[p] ?? p;

// What a preset leaves out, for people: "Library, Temp, Obj and 9 more".
export const leftOutText = (pats: string[]) => {
  const names = [...new Set(pats.map((p) => p.replace(/^\/|\/$/g, "")))];
  return names.length > 5 ? t("{names} and {n} more", { names: names.slice(0, 5).join(", "), n: names.length - 5 }) : names.join(", ");
};

// A folder of presets: ("" the project folder).
export const folderName = (f: string) => (f ? `${f}/` : t("The project folder"));

// Opens .r3v.yaml in a text editor.
export async function editRulesText(root: string) {
  try {
    await api.OpenRules(root);
    toast(t("Save the file, then R3V follows the new rules"), "info");
  } catch (e) {
    toast(errorText(e), "error");
  }
}

// One change to the rules: added (add), no longer there (del), changed (mod).
export type RulesChange = { kind: "add" | "del" | "mod"; text: string };

// What changed in the rules from `before` to `now` (null or missing: no
// file), rule by rule, in plain words. Comments and layout aren't rules.
export function rulesChanges(now: RulesState | null, before: RulesState | null): RulesChange[] {
  const out: RulesChange[] = [];
  const a = now?.exists ? now : null, b = before?.exists ? before : null;
  if (a && !b) out.push({ kind: "add", text: t("New rules file") });
  if (b && !a) out.push({ kind: "del", text: t("The rules file was deleted") });

  // Presets, by folder.
  const pa = new Map((a?.presets ?? []).map((e) => [e.folder.toLowerCase(), e]));
  const pb = new Map((b?.presets ?? []).map((e) => [e.folder.toLowerCase(), e]));
  for (const [k, e] of pa) {
    const old = pb.get(k);
    if (!old) out.push({ kind: "add", text: t("{folder}: {tool} preset", { tool: presetName(e.preset), folder: folderName(e.folder) }) });
    else if (old.preset !== e.preset) out.push({ kind: "mod", text: t("{folder}: preset changed from {from} to {to}",
      { folder: folderName(e.folder), from: presetName(old.preset), to: presetName(e.preset) }) });
  }
  for (const [k, e] of pb) {
    if (!pa.has(k)) out.push({ kind: "del", text: t("{folder}: no preset named any more (was {tool})", { folder: folderName(e.folder), tool: presetName(e.preset) }) });
  }

  // Rules: a pattern added, gone, or switched between left out and kept.
  const key = (r: { kind: string; pattern: string }) => `${r.kind}\u0000${r.pattern}`;
  const ra = a?.rules ?? [], rb = b?.rules ?? [];
  const ka = new Set(ra.map(key)), kb = new Set(rb.map(key));
  const switched = new Set<string>(); // patterns whose rule changed kind
  for (const r of ra) {
    if (kb.has(key(r))) continue;
    const other = { kind: r.kind === "ignore" ? "track" : "ignore", pattern: r.pattern };
    if (kb.has(key(other)) && !ka.has(key(other))) {
      switched.add(r.pattern);
      out.push({ kind: "mod", text: r.kind === "ignore" ? t("Now left out (was kept): {pattern}", { pattern: r.pattern })
        : t("Now kept (was left out): {pattern}", { pattern: r.pattern }) });
    } else {
      out.push({ kind: "add", text: r.kind === "ignore" ? t("Left out: {pattern}", { pattern: r.pattern })
        : t("Kept: {pattern}", { pattern: r.pattern }) });
    }
  }
  for (const r of rb) {
    if (ka.has(key(r)) || switched.has(r.pattern)) continue;
    out.push({ kind: "del", text: r.kind === "ignore" ? t("No longer left out: {pattern}", { pattern: r.pattern })
      : t("No longer kept by a rule: {pattern}", { pattern: r.pattern }) });
  }
  // The same rules in another order: later rules win, so it can matter.
  if (a && b && ra.length === rb.length && ra.length > 1 && ra.every((r) => kb.has(key(r))) &&
    ra.some((r, i) => key(r) !== key(rb[i]))) {
    out.push({ kind: "mod", text: t("The rules' order changed (later rules win)") });
  }

  // Other settings.
  const reqA = a?.requires ?? "", reqB = b?.requires ?? "";
  if (reqA && reqA !== reqB) out.push({ kind: reqB ? "mod" : "add", text: reqB
    ? t("Needs R3V {version} or newer (was {old})", { version: reqA, old: reqB })
    : t("Needs R3V {version} or newer", { version: reqA }) });
  if (!reqA && reqB) out.push({ kind: "del", text: t("No longer says which R3V it needs (was {old})", { old: reqB }) });
  if (!!a?.gitignore !== !!b?.gitignore) out.push(a?.gitignore
    ? { kind: "add", text: t("Follows the project's .gitignore files") }
    : { kind: "del", text: t("No longer follows the project's .gitignore files") });
  // File locks (file_locks:).
  if (a || b) out.push(...fileLocksChanges(a?.fileLocks, b?.fileLocks));
  return out;
}
