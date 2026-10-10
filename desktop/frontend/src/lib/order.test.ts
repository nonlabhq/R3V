import { describe, expect, it } from "vitest";
import { besideMain, moveKey, readOrder, sortByOrder } from "./order";

describe("project order", () => {
  it("sorts by the order dragged, the rest after as they came", () => {
    const id = (x: string) => x;
    expect(sortByOrder(["a", "b", "c", "d"], id, ["c", "a"])).toEqual(["c", "a", "b", "d"]);
    expect(sortByOrder(["a", "b"], id, [])).toEqual(["a", "b"]);
    expect(sortByOrder(["a", "b"], id, ["gone", "b"])).toEqual(["b", "a"]);
  });

  it("moves a key up or down", () => {
    expect(moveKey(["a", "b", "c", "d"], "a", 3)).toEqual(["b", "c", "a", "d"]);
    expect(moveKey(["a", "b", "c", "d"], "a", 4)).toEqual(["b", "c", "d", "a"]);
    expect(moveKey(["a", "b", "c", "d"], "d", 0)).toEqual(["d", "a", "b", "c"]);
    expect(moveKey(["a", "b", "c"], "b", 1)).toEqual(["a", "b", "c"]);
    expect(moveKey(["a", "b", "c"], "b", 2)).toEqual(["a", "b", "c"]);
    expect(moveKey(["a", "b"], "x", 0)).toEqual(["a", "b"]);
  });

  it("reads only strings back", () => {
    expect(readOrder(["a", 3, null, "b"])).toEqual(["a", "b"]);
    expect(readOrder({})).toEqual([]);
  });

  it("puts an agent's workspace right after its project", () => {
    const p = (root: string, mainRoot = "") => ({ root, workspace: !!mainRoot, mainRoot });
    const song = p("C:\\Song"), beat = p("C:\\Beat"), ws1 = p("C:\\Song (a)", "c:\\song"), ws2 = p("C:\\Other (b)", "C:\\Other");
    expect(besideMain([ws1, beat, ws2, song])).toEqual([beat, song, ws1, ws2]);
    expect(besideMain([song, beat])).toEqual([song, beat]);
  });
});
