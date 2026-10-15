import { describe, expect, it } from "vitest";
import { gridKey, navKey, type NavRow } from "./keynav";

// Samples/ (open) > Drums/ (closed), kick.wav; notes.txt
const rows: NavRow[] = [
  { key: "dir:Samples", dir: true, open: true, depth: 0 },
  { key: "dir:Samples/Drums", dir: true, open: false, depth: 1 },
  { key: "Samples/kick.wav", depth: 1 },
  { key: "notes.txt", depth: 0 },
];

describe("navKey", () => {
  it("goes up and down, to the ends, and starts at the top", () => {
    expect(navKey(rows, "Samples/kick.wav", "ArrowDown")).toEqual({ to: "notes.txt" });
    expect(navKey(rows, "notes.txt", "ArrowDown")).toEqual({ to: "notes.txt" }); // (stays at the end)
    expect(navKey(rows, "dir:Samples", "ArrowUp")).toEqual({ to: "dir:Samples" });
    expect(navKey(rows, "", "ArrowDown")).toEqual({ to: "dir:Samples" });
    expect(navKey(rows, "notes.txt", "Home")).toEqual({ to: "dir:Samples" });
    expect(navKey(rows, "dir:Samples", "End")).toEqual({ to: "notes.txt" });
    expect(navKey(rows, "dir:Samples", "PageDown", 2)).toEqual({ to: "Samples/kick.wav" });
  });

  it("opens and closes folders, goes into them and up out of them", () => {
    expect(navKey(rows, "dir:Samples/Drums", "ArrowRight")).toEqual({ toggle: "dir:Samples/Drums" }); // closed: opens
    expect(navKey(rows, "dir:Samples", "ArrowRight")).toEqual({ to: "dir:Samples/Drums" }); // open: in
    expect(navKey(rows, "dir:Samples", "ArrowLeft")).toEqual({ toggle: "dir:Samples" }); // open: closes
    expect(navKey(rows, "Samples/kick.wav", "ArrowLeft")).toEqual({ to: "dir:Samples" }); // up
    expect(navKey(rows, "dir:Samples/Drums", "ArrowLeft")).toEqual({ to: "dir:Samples" });
    expect(navKey(rows, "notes.txt", "ArrowLeft")).toBeNull(); // (at the top)
    expect(navKey(rows, "notes.txt", "ArrowRight")).toBeNull(); // (a file)
    expect(navKey(rows, "notes.txt", "x")).toBeNull();
  });
});

describe("gridKey", () => {
  const keys = ["a", "b", "c", "d", "e"]; // 3 columns: a b c / d e
  it("moves along and across rows, staying in the grid", () => {
    expect(gridKey(keys, "a", "ArrowRight", 3)).toBe("b");
    expect(gridKey(keys, "b", "ArrowDown", 3)).toBe("e");
    expect(gridKey(keys, "c", "ArrowDown", 3)).toBe("c"); // nothing below
    expect(gridKey(keys, "d", "ArrowUp", 3)).toBe("a");
    expect(gridKey(keys, "a", "ArrowLeft", 3)).toBe("a");
    expect(gridKey(keys, "", "ArrowDown", 3)).toBe("a");
    expect(gridKey(keys, "c", "End", 3)).toBe("e");
  });
});
