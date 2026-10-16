import { describe, expect, it } from "vitest";
import { Actor, Version } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";
import { branchGraph } from "$shared/branchGraph";
import { segmentRuns } from "$shared/graphSegments";
import type { GraphVersion } from "$shared/types";

// The app's history (State.History) goes to the graph as it is: what Go
// fills in (files, check, step) is what the graph's segments go by.

const agent = new Actor({ tool: "claude-code", model: "opus" });
const at = (min: number) => new Date(Date.UTC(2026, 9, 6, 10, min)).toISOString();

// lane: v10 … v1 off m1, an agent's, a minute apart; main: m1.
function history(over: Record<string, Partial<Version>> = {}): Version[] {
  const lane = Array.from({ length: 10 }, (_, i) => 10 - i).map((n) => new Version({
    id: `v${n}`, short: `v${n}`, author: "Robin", time: at(n), message: `v${n}`, parents: [n === 1 ? "m1" : `v${n - 1}`],
    branch: "lane", actor: agent, files: 2, step: "s1", ...over[`v${n}`] }));
  return [...lane, new Version({ id: "m1", short: "m1", author: "Robin", time: at(0), message: "m1", parents: [], branch: "main" })];
}
function runs(vs: GraphVersion[]) {
  const bs = [{ name: "main", latest: "m1" }, { name: "lane", latest: "v10" }];
  return segmentRuns(vs, branchGraph(vs, bs, "main", "main", "m1"), new Set());
}

describe("the app's history in the graph", () => {
  it("sums the files the versions changed", () => {
    const [s, ...rest] = runs(history());
    expect(rest).toEqual([]);
    expect(s.ids).toEqual(["v9", "v8", "v7", "v6", "v5", "v4", "v3", "v2"]);
    expect(s.files).toBe(16);
  });

  it("breaks a run at a failed check and where the agent's session changes", () => {
    expect(runs(history({ v6: { check: "fail" } })).map((s) => s.ids)).toEqual([["v9", "v8", "v7"], ["v5", "v4", "v3", "v2"]]);
    const later = { step: "s2" };
    expect(runs(history({ v9: later, v8: later, v7: later, v6: later })).map((s) => s.ids))
      .toEqual([["v9", "v8", "v7", "v6"], ["v5", "v4", "v3", "v2"]]);
  });
});
