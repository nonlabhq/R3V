import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import FileExplorer from "./FileExplorer.svelte";

// The Files tab's explorer: folders one at a time, sorting, kinds, columns.

const file = (path: string, kind: string, size: number, modified = "2026-10-06T10:00:00Z", status = "unchanged") =>
  ({ path, status, size, kind, live: "", from: "", edited: false, preview: false, video: false, model: false, modified });
const files = [
  file("Song.als", "set", 300),
  file("notes.txt", "other", 20, "2026-10-01T10:00:00Z"),
  file("Samples/Kick.wav", "audio", 1000),
  file("Samples/Loops/Loop 2.wav", "audio", 500),
  file("Samples/Loops/Loop 10.wav", "audio", 700, "2026-10-06T10:00:00Z", "added"),
];
const names = () => [...document.querySelectorAll(".row:not(.head) .nm")].map((n) => n.textContent);

beforeEach(() => localStorage.clear());
afterEach(() => cleanup());

function show() {
  const onselect = vi.fn();
  render(FileExplorer, { root: "C:/Song", files, selected: "", onselect });
  return { onselect };
}

describe("FileExplorer", () => {
  it("lists a folder at a time, folders first, and goes in and back up", async () => {
    show();
    expect(names()).toEqual(["Samples", "notes.txt", "Song.als"]);
    expect(screen.getByText("3 files")).toBeTruthy(); // in Samples
    await fireEvent.click(screen.getByTitle("Samples"));
    expect(names()).toEqual(["Loops", "Kick.wav"]);
    await fireEvent.click(screen.getByRole("button", { name: "Project" }));
    expect(names()).toEqual(["Samples", "notes.txt", "Song.als"]);
  });

  it("goes through the files with the keyboard, into folders and back up", async () => {
    const { onselect } = show();
    const list = screen.getByRole("grid", { name: "Files" });
    const on = () => document.querySelector(".row.on .nm")?.textContent;
    await fireEvent.keyDown(list, { key: "ArrowDown" }); // the first: a folder
    expect(on()).toBe("Samples");
    expect(onselect).not.toHaveBeenCalled();
    await fireEvent.keyDown(list, { key: "ArrowDown" });
    expect(onselect).toHaveBeenLastCalledWith("notes.txt");
    await fireEvent.keyDown(list, { key: "End" });
    expect(onselect).toHaveBeenLastCalledWith("Song.als");
    await fireEvent.keyDown(list, { key: "Home" });
    await fireEvent.keyDown(list, { key: "ArrowRight" }); // into Samples
    expect(names()).toEqual(["Loops", "Kick.wav"]);
    await fireEvent.keyDown(list, { key: "ArrowDown" });
    await fireEvent.keyDown(list, { key: "Enter" }); // into Loops
    expect(names()).toEqual(["Loop 2.wav", "Loop 10.wav"]);
    await fireEvent.keyDown(list, { key: "Backspace" });
    expect(names()).toEqual(["Loops", "Kick.wav"]);
    expect(on()).toBe("Loops"); // (on the folder just left)
    await fireEvent.keyDown(list, { key: "ArrowLeft" });
    expect(names()).toEqual(["Samples", "notes.txt", "Song.als"]);
  });

  it("enters one folder on a double click, not two", async () => {
    show();
    const samples = screen.getByTitle("Samples");
    await fireEvent.click(samples, { detail: 1 });
    // The second click of the double click lands on the row now there.
    await fireEvent.click(screen.getByTitle("Samples/Loops"), { detail: 2 });
    expect(names()).toEqual(["Loops", "Kick.wav"]);
  });

  it("sorts numbers naturally, by size, and shows only some kinds", async () => {
    const { onselect } = show();
    await fireEvent.click(screen.getByTitle("Samples"));
    await fireEvent.click(screen.getByTitle("Samples/Loops"));
    expect(names()).toEqual(["Loop 2.wav", "Loop 10.wav"]);
    await fireEvent.click(screen.getByRole("columnheader", { name: /^Size/ }));
    expect(names()).toEqual(["Loop 10.wav", "Loop 2.wav"]); // biggest first
    await fireEvent.click(screen.getByTitle("Samples/Loops/Loop 2.wav"));
    expect(onselect).toHaveBeenCalledWith("Samples/Loops/Loop 2.wav");
    await fireEvent.click(screen.getByRole("button", { name: "Project" }));
    await fireEvent.click(screen.getByTitle("Show only some kinds of file"));
    await fireEvent.click(screen.getByRole("checkbox", { name: /Live Set/ }));
    expect(names()).toEqual(["Song.als"]);
    // The filter's button: the kinds picked, as icons (just the funnel when none).
    const filter = screen.getByRole("button", { name: "Show only some kinds of file" });
    expect(filter.title).toBe("Showing only: Live Set");
    expect(filter.querySelector("svg path")?.getAttribute("d")).not.toBe("M2.5 3h11L9.5 8.5v4l-3 1.5V8.5z");
    // Sorting says how in its tooltip only.
    expect(screen.getByRole("button", { name: "Sort" }).title).toMatch(/^Sort: Size/);
  });

  it("remembers how it looks", async () => {
    show();
    await fireEvent.click(screen.getByRole("button", { name: "Grid" }));
    cleanup();
    show();
    expect(document.querySelector(".grid")).not.toBeNull();
  });
});
