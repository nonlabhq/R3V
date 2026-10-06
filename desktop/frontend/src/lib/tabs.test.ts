import { describe, expect, it } from "vitest";
import { closeTab, loadTabs, moveTab, pruneTabs, renameTab, toSave } from "./tabs";

describe("tabs", () => {
  it("loads what was saved, whatever it is, with the new tabs open now", () => {
    expect(loadTabs(["a", "b"], ["new:1", "c"])).toEqual(["a", "b", "new:1"]);
    expect(loadTabs({}, ["new:1"])).toEqual(["new:1"]);
    expect(loadTabs(null, [])).toEqual([]);
    expect(loadTabs(["a", 3, "new:9", "a"], [])).toEqual(["a"]);
    expect(toSave(["a", "new:1", "b"])).toEqual(["a", "b"]);
  });

  it("keeps a tab in its place when its project's key changes", () => {
    // downloaded: id -> folder
    expect(renameTab(["x", "id1", "y"], "id1", "C:/p")).toEqual(["x", "C:/p", "y"]);
    // already open under the new key: one tab
    expect(renameTab(["C:/p", "id1"], "id1", "C:/p")).toEqual(["C:/p"]);
    expect(renameTab(["x"], "id1", "C:/p")).toEqual(["x"]);
  });

  it("drops tabs of projects gone, not new tabs", () => {
    expect(pruneTabs(["a", "gone", "new:1"], new Set(["a"]))).toEqual(["a", "new:1"]);
  });

  it("shows the next tab, else the one before, when the shown one closes", () => {
    expect(closeTab(["a", "b", "c"], "b")).toEqual({ keys: ["a", "c"], next: "c" });
    expect(closeTab(["a", "b", "c"], "c")).toEqual({ keys: ["a", "b"], next: "b" });
    expect(closeTab(["a"], "a")).toEqual({ keys: [], next: "" });
  });

  it("moves a tab among the shown ones, whatever else is listed", () => {
    expect(moveTab(["a", "b", "c"], ["a", "b", "c"], "a", 3)).toEqual(["b", "c", "a"]);
    expect(moveTab(["a", "b", "c"], ["a", "b", "c"], "c", 0)).toEqual(["c", "a", "b"]);
    expect(moveTab(["a", "b", "c"], ["a", "b", "c"], "b", 2)).toEqual(["a", "b", "c"]);
    // a key not drawn (its project not listed yet) doesn't throw the place off
    expect(moveTab(["hidden", "a", "b"], ["a", "b"], "b", 0)).toEqual(["b", "a", "hidden"]);
  });
});
