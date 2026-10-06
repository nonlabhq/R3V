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

  it("cuts long titles", () => {
    expect(short("Arrangement: intro and breakdown", 16)).toBe("Arrangement: in…");
    expect(short("Short")).toBe("Short");
  });
});
