import { describe, expect, it } from "vitest";
import { splitPx, splits } from "./splits.svelte";

describe("splits", () => {
  it("gives the left pane its share of the width, within both panes' smallest", () => {
    expect(splitPx("t1", 0.5, 1000, 200, 300)).toBe(500);
    splits.t2 = 0.25;
    expect(splitPx("t2", 0.5, 1000, 200, 300)).toBe(250);
    expect(splitPx("t1", 0.1, 1000, 200, 300)).toBe(200); // not under the left's smallest
    expect(splitPx("t1", 0.9, 1000, 200, 300)).toBe(700); // nor over what the right needs
    expect(splitPx("t1", 0.5, 0, 200, 300)).toBe(200); // not laid out yet: no NaN
    expect(splitPx("t1", 0.5, 400, 200, 300)).toBe(200); // too narrow for both: the left keeps its smallest
    splits.t3 = -2;
    expect(splitPx("t3", 0.5, 1000, 200, 300)).toBe(200);
  });
});
