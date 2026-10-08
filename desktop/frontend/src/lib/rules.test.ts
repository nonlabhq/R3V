import { describe, expect, it } from "vitest";
import { rulesChanges } from "./rules";

const state = (over: Record<string, unknown> = {}) => ({ exists: true, error: "", requires: "", gitignore: false,
  presets: [], rules: [], options: [], ...over }) as unknown as Parameters<typeof rulesChanges>[0];
const lines = (now: ReturnType<typeof state> | null, before: ReturnType<typeof state> | null) =>
  rulesChanges(now, before).map((c) => `${c.kind} ${c.text}`);

// What changed in the rules, rule by rule, in plain words.
describe("rulesChanges", () => {
  it("names rules added, gone and switched", () => {
    const before = state({ rules: [{ kind: "ignore", pattern: "Renders/" }, { kind: "ignore", pattern: "*.tmp" }] });
    const now = state({ rules: [{ kind: "track", pattern: "*.tmp" }, { kind: "ignore", pattern: "*.wav" }] });
    expect(lines(now, before)).toEqual([
      "mod Now kept (was left out): *.tmp",
      "add Left out: *.wav",
      "del No longer left out: Renders/",
    ]);
  });

  it("names presets added, changed and gone, by folder", () => {
    const before = state({ presets: [{ folder: "", preset: "ableton", found: true }, { folder: "Game", preset: "unity", found: false }] });
    const now = state({ presets: [{ folder: "", preset: "none", found: false }, { folder: "Art", preset: "design", found: false }] });
    expect(lines(now, before)).toEqual([
      "mod The project folder: preset changed from Ableton Live to No preset",
      "add Art/: Design files preset",
      "del Game/: no preset named any more (was Unity)",
    ]);
  });

  it("names the other settings and a new order", () => {
    const r = [{ kind: "ignore", pattern: "a/" }, { kind: "track", pattern: "a/b" }];
    const before = state({ rules: r, requires: "0.1.0" });
    const now = state({ rules: [...r].reverse(), requires: "0.2.0", gitignore: true });
    expect(lines(now, before)).toEqual([
      "mod The rules' order changed (later rules win)",
      "mod Needs R3V 0.2.0 or newer (was 0.1.0)",
      "add Follows the project's .gitignore files",
    ]);
  });

  it("says when the file is new or gone, and nothing when only comments changed", () => {
    const s = state({ rules: [{ kind: "ignore", pattern: "x/" }] });
    expect(lines(s, null)).toEqual(["add New rules file", "add Left out: x/"]);
    expect(lines(state({ exists: false }), s)).toEqual(["del The rules file was deleted", "del No longer left out: x/"]);
    expect(lines(s, state({ rules: [{ kind: "ignore", pattern: "x/" }] }))).toEqual([]);
  });
});
