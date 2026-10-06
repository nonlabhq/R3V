import { describe, expect, it } from "vitest";
import { branchGraph, short } from "./branchGraph";

// main: m3 (merges a2) - m2 - m1; alt: a2 - a1 from m1 (merged into m3);
// mine: y1 from m2, the branch you're on.
const versions = [
  { id: "y1", parents: ["m2"] },
  { id: "m3", parents: ["m2", "a2"] },
  { id: "a2", parents: ["a1"] },
  { id: "m2", parents: ["m1"] },
  { id: "a1", parents: ["m1"] },
  { id: "m1", parents: [] },
];
const branches = [{ name: "main", latest: "m3" }, { name: "alt", latest: "a2" }, { name: "mine", latest: "y1" }];

describe("branch graph", () => {
  const g = branchGraph(versions, branches, "mine", "main", "y1");
  const col = (id: string) => g.chainOf.get(id)!.col;
  it("puts main in the middle", () => expect([col("m1"), col("m2"), col("m3")]).toEqual([0, 0, 0]));
  it("puts the branch you're on to the right", () => expect(col("y1")).toBe(1));
  it("puts other branches to the left", () => expect(col("a1")).toBe(-1));
  it("follows a branch down to where it split", () => expect(g.chainOf.get("a2")!.ids).toEqual(["a2", "a1"]));
  it("tells forks and merges from lines", () => {
    expect(g.edges).toContainEqual({ from: "a1", to: "m1", kind: "fork" });
    expect(g.edges).toContainEqual({ from: "m3", to: "a2", kind: "merge" });
    expect(g.edges).toContainEqual({ from: "m3", to: "m2", kind: "line" });
  });
  it("counts the columns on each side", () => expect([g.left, g.right]).toEqual([1, 1]));

  it("uses a column again below a branch that ended", () => {
    // b: b1 from m1 into m2 (merged); c: c1 from m3, later, also on the left.
    const g2 = branchGraph([
      { id: "c1", parents: ["m3"] }, { id: "m3", parents: ["m2"] }, { id: "x", parents: ["m3"] },
      { id: "m2", parents: ["m1", "b1"] }, { id: "b1", parents: ["m1"] }, { id: "m1", parents: [] },
    ], [{ name: "main", latest: "m3" }, { name: "c", latest: "c1" }, { name: "b", latest: "b1" }, { name: "x", latest: "x" }], "main");
    expect(g2.chainOf.get("b1")!.col).toBe(-1);
    expect(g2.chainOf.get("c1")!.col).toBe(-1);
  });

  it("draws a branch just made, with no versions of its own, from where it starts", () => {
    // idea: made from m2 (on main), nothing committed on it yet; you are on it.
    const g3 = branchGraph(versions, [...branches, { name: "idea", latest: "m2" }], "idea", "main", "m2");
    const idea = g3.chains.find((c) => c.name === "idea")!;
    expect(idea).toMatchObject({ empty: true, tip: "m2", ids: [], mine: true });
    expect(idea.col).toBeGreaterThan(0);
    expect(g3.chainOf.get("m2")!.main).toBe(true); // m2 stays on main
  });

  it("keeps a version of another branch on that branch when you go to it", () => {
    // x: x2 - x1 from m1; you are on main and went to x1.
    const g4 = branchGraph([
      { id: "x2", parents: ["x1"] }, { id: "m2", parents: ["m1"] }, { id: "x1", parents: ["m1"] }, { id: "m1", parents: [] },
    ], [{ name: "main", latest: "m2" }, { name: "x", latest: "x2" }], "main", "main", "x1");
    expect(g4.chainOf.get("x1")!.name).toBe("x");
    expect(g4.chainOf.get("x1")!.col).not.toBe(0);
    expect(g4.chains.filter((c) => c.name === "main")).toHaveLength(1);
  });

  it("puts your versions not shared yet on your branch's line", () => {
    // main on the team is m1; you committed m2 and m3 here.
    const g5 = branchGraph([{ id: "m3", parents: ["m2"] }, { id: "m2", parents: ["m1"] }, { id: "m1", parents: [] }],
      [{ name: "main", latest: "m1" }], "main", "main", "m3");
    expect(g5.chains).toHaveLength(1);
    expect(g5.chains[0].ids).toEqual(["m3", "m2", "m1"]);
  });

  it("draws two branches on the same newest version, each once", () => {
    const g6 = branchGraph(versions, [...branches, { name: "idea", latest: "m3" }], "idea", "main", "m3");
    expect(g6.chains.filter((c) => c.name === "main")).toHaveLength(1);
    expect(g6.chains.filter((c) => c.name === "idea")).toHaveLength(1);
    expect(g6.chainOf.get("m3")!.name).toBe("main");
  });

  it("copes with a branch the team doesn't list yet, an empty history, loops and missing parents", () => {
    const g7 = branchGraph(versions, branches.filter((b) => b.name !== "mine"), "mine", "main", "y1");
    expect(g7.chainOf.get("y1")).toBeDefined();
    expect(branchGraph([], [], "main").chains).toEqual([]);
    const loop = branchGraph([{ id: "a", parents: ["b"] }, { id: "b", parents: ["a"] }, { id: "c", parents: ["gone"] }],
      [{ name: "main", latest: "a" }], "main", "main", "a");
    expect(loop.chainOf.size).toBe(3);
  });

  it("cuts long titles", () => {
    expect(short("Arrangement: intro and breakdown", 16)).toBe("Arrangement: in…");
    expect(short("Short")).toBe("Short");
  });
});
