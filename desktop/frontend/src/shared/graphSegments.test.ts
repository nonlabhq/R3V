import { describe, expect, it } from "vitest";
import { branchGraph } from "./branchGraph";
import { collapse, segmentRuns, spanText } from "./graphSegments";
import type { GraphVersion } from "./types";

// Which versions become one segment, and which always stay their own node.

const agent = { tool: "claude-code", model: "opus" };
const at = (min: number) => new Date(Date.UTC(2026, 9, 6, 10, min)).toISOString();
const ver = (id: string, parents: string[], branch: string, over: Partial<GraphVersion> = {}): GraphVersion => ({
  id, short: id, author: "Robin", authorId: "", time: at(0), message: id, parents, branch, actor: agent, ...over });

// main: m2 - m1 (a person's); lane/render: r8 … r1 off m1, an agent's, a minute apart.
function history(over: Record<string, Partial<GraphVersion>> = {}) {
  const lane = Array.from({ length: 8 }, (_, i) => 8 - i).map((n) =>
    ver(`r${n}`, [n === 1 ? "m1" : `r${n - 1}`], "lane/render", { time: at(n), ...over[`r${n}`] }));
  return [ver("m2", ["m1"], "main", { actor: null, time: at(20) }), ...lane, ver("m1", [], "main", { actor: null })];
}
const branches = [{ name: "main", latest: "m2" }, { name: "lane/render", latest: "r8" }];
function runs(vs: GraphVersion[], keep: string[] = [], bs: { name: string; latest: string }[] = branches) {
  return segmentRuns(vs, branchGraph(vs, bs, "main", "main", "m2"), new Set(keep));
}

describe("segments", () => {
  it("makes a run of an agent's versions one segment, the line's newest and oldest left out", () => {
    const [s, ...rest] = runs(history());
    expect(rest).toEqual([]);
    expect(s.ids).toEqual(["r7", "r6", "r5", "r4", "r3", "r2"]);
    expect(s).toMatchObject({ id: "seg:r2", key: "r2", branch: "lane/render", from: at(2), to: at(7) });
    expect(s.files).toBeUndefined(); // (the versions don't say)
  });

  it("keeps a person's versions, checked ones, merges, milestones and what the caller keeps their own", () => {
    expect(runs(history({ r5: { actor: null } })).map((s) => s.ids)).toEqual([["r4", "r3", "r2"]]); // (r7, r6: two)
    expect(runs(history({ r5: { check: "fail" } })).map((s) => s.ids)).toEqual([["r4", "r3", "r2"]]);
    expect(runs(history({ r4: { check: "pass" } })).map((s) => s.ids)).toEqual([["r7", "r6", "r5"]]);
    expect(runs(history(), ["r5"]).map((s) => s.ids)).toEqual([["r4", "r3", "r2"]]);
    expect(runs(history(), ["r4", "r6"])).toEqual([]); // (runs of two or one stay as they are)
    // r6 merges main's m1 in: a merge, its own node.
    expect(runs(history({ r6: { parents: ["r5", "m1"] } })).map((s) => s.ids)).toEqual([["r5", "r4", "r3", "r2"]]);
  });

  it("keeps where another branch starts, and a version merged into another line", () => {
    // idea: i1 from r4; m2 merges r6 into main.
    const vs = [ver("i1", ["r4"], "idea", { time: at(30) }), ...history().map((v) => (v.id === "m2" ? { ...v, parents: ["m1", "r6"] } : v))];
    const got = runs(vs, [], [...branches, { name: "idea", latest: "i1" }]);
    expect(got).toEqual([]); // (r7, r5, then r3 and r2: none three long)
    expect(runs(vs.map((v) => (v.id === "i1" ? { ...v, parents: ["m1"] } : v)), [], [...branches, { name: "idea", latest: "i1" }])
      .map((s) => s.ids)).toEqual([["r5", "r4", "r3", "r2"]]);
  });

  it("cuts at the agent's steps, one segment per step", () => {
    const step = (s: string) => ({ step: s });
    const got = runs(history({ r7: step("b"), r6: step("b"), r5: step("b"), r4: step("a"), r3: step("a"), r2: step("a") }));
    expect(got.map((s) => s.ids)).toEqual([["r7", "r6", "r5"], ["r4", "r3", "r2"]]);
  });

  it("adds up the files changed when the versions say", () => {
    const [s] = runs(history({ r2: { files: 3 }, r3: { files: 2 } }));
    expect(s.files).toBe(5);
  });

  it("lays a closed segment out as one row, at its newest version's place, its child pointing at it", () => {
    const vs = history();
    const segs = runs(vs);
    const { rows, segs: closed } = collapse(vs, segs, new Set());
    expect(rows.map((r) => r.id)).toEqual(["m2", "r8", "seg:r2", "r1", "m1"]);
    expect(rows.find((r) => r.id === "r8")!.parents).toEqual(["seg:r2"]);
    expect(rows.find((r) => r.id === "seg:r2")!.parents).toEqual(["r1"]);
    expect(closed.has("seg:r2")).toBe(true);
    const g = branchGraph(rows, branches, "main", "main", "m2");
    expect(g.chainOf.get("seg:r2")!.name).toBe("lane/render");
    // opened: every version again, as it was given
    expect(collapse(vs, segs, new Set(["r2"])).rows).toBe(vs);
  });

  it("says how long a segment took, short", () => {
    expect(spanText(at(0), at(2), "en")).toBe("2m");
    expect(spanText(at(0), at(0), "en")).toBe("1m");
    expect(spanText("2026-10-06T10:00:00Z", "2026-10-06T12:10:00Z", "en")).toBe("2h");
    expect(spanText("2026-10-01T10:00:00Z", "2026-10-06T10:00:00Z", "en")).toBe("5d");
  });
});
