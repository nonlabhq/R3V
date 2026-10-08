import { describe, expect, it } from "vitest";
import { clickPick, folderTree, findFolder, marqueePick } from "./files";

const file = (path: string, status = "unchanged", kind = "other", preview = false) =>
  ({ path, status, size: 1, kind, live: "", from: "", edited: false, preview, video: false, model: false, modified: "" });

describe("files", () => {
  it("makes the folder tree with the changed files inside each", () => {
    const top = folderTree([file("a.txt", "modified"), file("B/c.txt", "added"), file("B/D/e.txt", "deleted"), file("B/f.txt"),
      file("x/y.txt", "ignored")], "Song");
    expect(top.name).toBe("Song");
    expect(top.changed).toBe(3);
    expect(top.folders.map((f) => f.name)).toEqual(["B", "x"]);
    expect(findFolder(top, "B")!.changed).toBe(2);
    expect(findFolder(top, "B")!.files).toBe(2);
    expect(findFolder(top, "B/D")!.changed).toBe(1);
    expect(findFolder(top, "x")!.changed).toBe(0);
    expect(findFolder(top, "B/Z")).toBeNull();
  });

  it("picks with a click, Ctrl and Shift", () => {
    const order = ["a", "b", "c", "d"];
    let s = clickPick({ picked: [], anchor: "" }, order, "b", {});
    expect(s).toEqual({ picked: ["b"], anchor: "b" });
    s = clickPick(s, order, "d", { shift: true });
    expect(s.picked).toEqual(["b", "c", "d"]);
    s = clickPick(s, order, "a", { shift: true }); // the range from the same anchor
    expect(s.picked).toEqual(["a", "b"]);
    s = clickPick(s, order, "d", { ctrl: true });
    expect(s).toEqual({ picked: ["a", "b", "d"], anchor: "d" });
    s = clickPick(s, order, "a", { ctrl: true });
    expect(s.picked).toEqual(["b", "d"]);
    s = clickPick(s, order, "b", { ctrl: true, shift: true }); // Ctrl+Shift adds a range
    expect(s.picked).toEqual(["b", "d", "a"]); // (from the last Ctrl+click)
  });

  it("picks what a box touches", () => {
    const at = (key: string, top: number) => ({ key, rect: { left: 0, right: 100, top, bottom: top + 40 } });
    const items = [at("a", 0), at("b", 50), at("c", 100)];
    expect(marqueePick({ left: 10, right: 20, top: 45, bottom: 60 }, items)).toEqual(["b"]);
    expect(marqueePick({ left: 10, right: 20, top: 30, bottom: 120 }, items)).toEqual(["a", "b", "c"]);
    expect(marqueePick({ left: 200, right: 220, top: 0, bottom: 120 }, items)).toEqual([]);
    expect(marqueePick({ left: 10, right: 20, top: 45, bottom: 48 }, items, ["a"])).toEqual(["a"]); // between rows: kept
  });
});
