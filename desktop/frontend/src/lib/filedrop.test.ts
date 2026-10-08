import { describe, expect, it, vi } from "vitest";
import { clearDropMarks, delivered, landed, onDroppedFiles } from "./filedrop";

describe("files dropped on the page", () => {
  it("go to where they landed, as the page saw it", () => {
    const fn = vi.fn();
    const off = onDroppedFiles(fn);
    document.body.innerHTML = `<div data-file-drop-target data-root="C:/Song" data-dir="Samples" class="file-drop-target-active"><span id="in"></span></div><p id="out"></p>`;
    landed(document.getElementById("in"));
    expect(delivered(["D:/pad.wav"])).toBe(true);
    expect(fn).toHaveBeenCalledWith({ files: ["D:/pad.wav"], root: "C:/Song", dir: "Samples" });
    expect(document.querySelector(".file-drop-target-active")).toBeNull();
    // once only; and a drop on no target is left to Wails
    expect(delivered(["D:/pad.wav"])).toBe(false);
    landed(document.getElementById("out"));
    expect(delivered(["D:/pad.wav"])).toBe(false);
    off();
  });

  it("clears marks left on", () => {
    document.body.innerHTML = `<div class="file-drop-target-active"></div><div class="file-drop-target-active"></div>`;
    clearDropMarks();
    expect(document.querySelectorAll(".file-drop-target-active")).toHaveLength(0);
  });
});
