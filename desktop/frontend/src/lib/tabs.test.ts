import { describe, expect, it } from "vitest";
import { closeTab, loadTabs, moveTab, oldTabs, pruneTabs, refreshSnaps, renameTab, snapOf, tabId, teamsOf, toSave, type TabRef } from "./tabs";

const tab = (team: string, key: string): TabRef => ({ team, key });
const keys = (tabs: TabRef[]) => tabs.map((x) => `${x.team}:${x.key}`);
const snap = (name: string) => snapOf({ id: name.toLowerCase(), root: `C:/${name}`, name, status: "downloaded" });

describe("tabs", () => {
  it("loads what was saved, whatever it is, with the new tabs open now", () => {
    expect(keys(loadTabs([tab("A", "a"), tab("B", "a")], [tab("", "new:1"), tab("A", "c")]))).toEqual(["A:a", "B:a", ":new:1"]);
    expect(loadTabs({}, [tab("", "new:1")])).toEqual([tab("", "new:1")]);
    expect(loadTabs(null, [])).toEqual([]);
    expect(keys(loadTabs([tab("A", "a"), "a", 3, tab("A", "new:9"), tab("A", "a"), { key: "x" }, tab("", "y")], []))).toEqual(["A:a"]);
    // a picture kept, a broken one dropped
    expect(loadTabs([{ team: "A", key: "a", p: { name: "Song", status: 1 } }], [])).toEqual([{ team: "A", key: "a", p: snapOf({ name: "Song" }) }]);
    expect(loadTabs([{ team: "A", key: "a", p: "Song" }], [])).toEqual([tab("A", "a")]);
    expect(keys(toSave([tab("A", "a"), tab("", "new:1"), tab("B", "b")]))).toEqual(["A:a", "B:b"]);
  });

  it("takes a team's tabs from the list kept before", () => {
    expect(oldTabs("A", ["a", "new:1", "b", "a", 3])).toEqual([tab("A", "a"), tab("A", "b")]);
    expect(oldTabs("A", "x")).toEqual([]);
  });

  it("tells tabs of the same project key on two teams apart", () => {
    expect(tabId(tab("A", "a"))).not.toBe(tabId(tab("B", "a")));
    expect(tabId(tab("", "new:1"))).toBe("new:1");
  });

  it("refreshes the pictures of the shown team's tabs only", () => {
    const tabs = [tab("A", "C:/Song"), tab("B", "C:/Song")];
    const got = refreshSnaps(tabs, "A", new Map([["C:/Song", snap("Song")]]));
    expect(got).toEqual([{ team: "A", key: "C:/Song", p: snap("Song") }, tab("B", "C:/Song")]);
    // nothing new: the same list (no save, no loop)
    expect(refreshSnaps(got, "A", new Map([["C:/Song", snap("Song")]]))).toBe(got);
    expect(refreshSnaps(got, "A", new Map([["C:/Song", { ...snap("Song"), status: "missing" }]]))[0].p?.status).toBe("missing");
  });

  it("keeps a tab in its place when its project's key changes on its team", () => {
    // downloaded: id -> folder
    expect(keys(renameTab([tab("A", "x"), tab("A", "id1"), tab("A", "y")], "A", "id1", "C:/p"))).toEqual(["A:x", "A:C:/p", "A:y"]);
    // already open under the new key: one tab
    expect(keys(renameTab([tab("A", "C:/p"), tab("A", "id1")], "A", "id1", "C:/p"))).toEqual(["A:C:/p"]);
    expect(keys(renameTab([tab("A", "x")], "A", "id1", "C:/p"))).toEqual(["A:x"]);
    // another team's tab of the same key stays as it is
    expect(keys(renameTab([tab("B", "id1"), tab("A", "id1")], "A", "id1", "C:/p"))).toEqual(["B:id1", "A:C:/p"]);
  });

  it("drops tabs of projects gone from the shown team, and of teams gone, not new tabs", () => {
    const tabs = [tab("A", "a"), tab("A", "gone"), tab("B", "b"), tab("C", "c"), tab("", "new:1")];
    expect(keys(pruneTabs(tabs, new Set(["A", "B", "C"]), "A", new Set(["a"])))).toEqual(["A:a", "B:b", "C:c", ":new:1"]);
    // the list not whole: only teams gone
    expect(keys(pruneTabs(tabs, new Set(["A", "B"]), "A", null))).toEqual(["A:a", "A:gone", "B:b", ":new:1"]);
  });

  it("counts the teams tabs are on", () => {
    expect(teamsOf([tab("A", "a"), tab("A", "b"), tab("", "new:1")])).toBe(1);
    expect(teamsOf([tab("A", "a"), tab("B", "b")])).toBe(2);
  });

  it("shows the next tab, else the one before, when the shown one closes", () => {
    expect(closeTab(["a", "b", "c"], "b")).toEqual({ ids: ["a", "c"], next: "c" });
    expect(closeTab(["a", "b", "c"], "c")).toEqual({ ids: ["a", "b"], next: "b" });
    expect(closeTab(["a"], "a")).toEqual({ ids: [], next: "" });
  });

  it("moves a tab among the shown ones, whatever else is listed", () => {
    const [a, b, c] = [tab("A", "a"), tab("B", "b"), tab("A", "c")];
    const ids = [a, b, c].map(tabId);
    expect(moveTab([a, b, c], ids, tabId(a), 3)).toEqual([b, c, a]);
    expect(moveTab([a, b, c], ids, tabId(c), 0)).toEqual([c, a, b]);
    expect(moveTab([a, b, c], ids, tabId(b), 2)).toEqual([a, b, c]);
    // a tab not drawn (its project not listed yet) doesn't throw the place off
    const hidden = tab("A", "hidden");
    expect(moveTab([hidden, a, b], [tabId(a), tabId(b)], tabId(b), 0)).toEqual([b, a, hidden]);
  });
});
