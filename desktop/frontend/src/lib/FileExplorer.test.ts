import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// The Files tab: folders on the left, a folder's files in the middle (a
// list or a grid), picking files like in Explorer, renaming, dropping files
// in and dragging them out. The Go bindings are mocks.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  const handlers: Record<string, ((ev: { data: unknown }) => void)[]> = {};
  return { fns, api, toast: vi.fn(), handlers };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: (name: string, fn: (ev: { data: unknown }) => void) => {
    (mocks.handlers[name] ??= []).push(fn);
    return () => { mocks.handlers[name] = mocks.handlers[name].filter((f) => f !== fn); };
  } } }));
const emit = (name: string, data: unknown) => (mocks.handlers[name] ?? []).forEach((fn) => fn({ data }));

import FileExplorer from "./FileExplorer.svelte";
import type { State } from "./api";

// jsdom has no ResizeObserver (bind:offsetWidth uses it).
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;

const ROOT = "C:/Song";
const file = (path: string, kind: string, size: number, status = "unchanged", preview = false) =>
  ({ path, status, size, kind, live: "", from: "", edited: false, preview, video: false, model: false, modified: "2026-10-06T10:00:00Z" });
const files = [
  file("Song.als", "set", 300, "modified"),
  file("notes.txt", "other", 20),
  file("Samples/Kick.wav", "audio", 1000),
  file("Samples/Loops/Loop 2.wav", "audio", 500),
  file("Samples/Loops/Loop 10.wav", "audio", 700, "added"),
  file("Art/cover.png", "image", 4000, "modified", true),
  file("Art/back.png", "image", 3000, "unchanged", true),
];
const version = (id: string, message: string, author: string, time: string) =>
  ({ id, short: id, author, authorId: author.toLowerCase(), time, message, parents: [], branches: [], inBranch: true, notHere: false });
const st = { name: "Song", head: "h2", branch: "main", branches: [], history: [], changes: [] } as unknown as State;

const treeNames = () => [...document.querySelectorAll(".tree .folder .fname")].map((n) => n.textContent);
const rowNames = () => [...document.querySelectorAll(".row.item .fname, .card .cname")].map((n) => n.textContent);
const row = (name: string) => [...document.querySelectorAll<HTMLElement>("[data-path]")].find((r) => r.dataset.path?.endsWith(name))!;

beforeEach(() => {
  localStorage.clear();
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  toast.mockClear();
  api.ProjectFiles.mockResolvedValue(files);
  api.LastChanges.mockResolvedValue({
    "Song.als": version("h2", "Bass line", "Mia", "2026-10-06T09:00:00Z"),
    "notes.txt": version("h1", "First", "Yi", "2026-10-01T09:00:00Z"),
  });
  api.FileHistory.mockResolvedValue([]);
});
afterEach(() => cleanup());

async function show() {
  const props = { root: ROOT, st, onrestore: vi.fn(), ondiscard: vi.fn() };
  render(FileExplorer, props);
  await waitFor(() => expect(rowNames().length).toBeGreaterThan(0));
  return props;
}
const folder = (name: string) => screen.getAllByRole("button").find((b) => b.classList.contains("folder") && b.textContent?.includes(name))!;

describe("FileExplorer", () => {
  it("lists only folders on the left, with the changed files inside each", async () => {
    await show();
    expect(treeNames()).toEqual(["Song", "Art", "Samples"]);
    expect(screen.getByText("3 changed")).toBeTruthy(); // Song.als, Loop 10.wav, cover.png
    expect(folder("Samples").querySelector(".pill")?.textContent).toBe("1");
    expect(folder("Art").querySelector(".pill")?.textContent).toBe("1");
    // the top folder's files in the middle, the newest change first
    expect(rowNames()).toEqual(["Song.als", "notes.txt"]);
    expect(screen.getByText("Bass line")).toBeTruthy(); // its last version
    expect(screen.getByText(/Mia ·/)).toBeTruthy();
  });

  it("shows a folder's files, a grid for pictures, and remembers each folder's look", async () => {
    await show();
    await fireEvent.click(folder("Art"));
    expect(document.querySelector(".area.grid")).not.toBeNull(); // mostly pictures
    expect(rowNames()).toEqual(["cover.png", "back.png"]);
    expect(document.querySelector(".card img")).not.toBeNull();
    await fireEvent.click(folder("Samples"));
    expect(document.querySelector(".area.grid")).not.toBeNull(); // sounds: a grid too
    await fireEvent.click(screen.getByRole("button", { name: "List view" }));
    expect(document.querySelector(".area.list")).not.toBeNull();
    await fireEvent.click(folder("Art"));
    expect(document.querySelector(".area.grid")).not.toBeNull(); // Art kept its own
    await fireEvent.click(folder("Samples"));
    expect(document.querySelector(".area.list")).not.toBeNull();
    // inner folders open in the tree; the path above the files goes back up
    expect(treeNames()).toContain("Loops");
    await fireEvent.click(folder("Loops"));
    expect(rowNames()).toEqual(["Loop 10.wav", "Loop 2.wav"]); // not committed yet: newest
    await fireEvent.click(screen.getByRole("button", { name: "Samples" }));
    expect(rowNames()).toEqual(["Kick.wav"]);
  });

  it("filters the folder's files and sorts by size", async () => {
    await show();
    await fireEvent.input(screen.getByPlaceholderText("Filter in Song"), { target: { value: "not" } });
    expect(rowNames()).toEqual(["notes.txt"]);
    await fireEvent.input(screen.getByPlaceholderText("Filter in Song"), { target: { value: "" } });
    await fireEvent.click(screen.getByRole("columnheader", { name: "Size" }));
    expect(rowNames()).toEqual(["Song.als", "notes.txt"]); // biggest first
    await fireEvent.click(screen.getByRole("columnheader", { name: "Size" }));
    expect(rowNames()).toEqual(["notes.txt", "Song.als"]);
  });

  it("picks files like Explorer: a click, Ctrl+click, Shift+click", async () => {
    await show();
    await fireEvent.click(folder("Art"));
    await fireEvent.click(screen.getByRole("button", { name: "List view" }));
    await fireEvent.click(folder("Song"));
    const picked = () => [...document.querySelectorAll<HTMLElement>("[data-path][aria-selected=true]")].map((r) => r.dataset.path);
    await fireEvent.click(row("Song.als"));
    expect(picked()).toEqual(["Song.als"]);
    await fireEvent.click(row("notes.txt"), { ctrlKey: true });
    expect(picked()).toEqual(["Song.als", "notes.txt"]);
    await fireEvent.click(row("Song.als"), { ctrlKey: true });
    expect(picked()).toEqual(["notes.txt"]);
    await fireEvent.click(row("notes.txt"), { shiftKey: true }); // from the last clicked (Song.als)
    expect(picked()).toEqual(["Song.als", "notes.txt"]);
    // the panel shows the file clicked last
    await waitFor(() => expect(api.FileHistory).toHaveBeenCalledWith(ROOT, "notes.txt"));
    expect(screen.getByText("History of this file")).toBeTruthy();
  });

  it("drags the picked files out of the app", async () => {
    await show();
    await fireEvent.click(row("Song.als"));
    await fireEvent.click(row("notes.txt"), { ctrlKey: true });
    const area = screen.getByRole("grid", { name: "Files" });
    await fireEvent.pointerDown(row("notes.txt"), { button: 0, clientX: 10, clientY: 10 });
    await fireEvent.pointerMove(area, { buttons: 1, clientX: 40, clientY: 12 });
    expect(api.StartDrag).toHaveBeenCalledWith(ROOT, ["Song.als", "notes.txt"]);
  });

  it("renames with F2, and says why it can't", async () => {
    await show();
    await fireEvent.click(row("notes.txt"));
    const area = screen.getByRole("grid", { name: "Files" });
    await fireEvent.keyDown(area, { key: "F2" });
    const input = screen.getByRole("textbox", { name: "New name" }) as HTMLInputElement;
    expect(input.value).toBe("notes.txt");
    expect(input.selectionEnd).toBe(5); // the name, not the extension
    api.RenameFile.mockResolvedValueOnce("todo.txt");
    await fireEvent.input(input, { target: { value: "todo.txt" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(api.RenameFile).toHaveBeenCalledWith(ROOT, "notes.txt", "todo.txt");
    await waitFor(() => expect(api.ProjectFiles).toHaveBeenCalledTimes(2));

    api.RenameFile.mockRejectedValueOnce(new Error("there is already a file or folder named Song.als here"));
    await fireEvent.click(row("notes.txt"));
    await fireEvent.keyDown(area, { key: "F2" });
    const again = screen.getByRole("textbox", { name: "New name" });
    await fireEvent.input(again, { target: { value: "Song.als" } });
    await fireEvent.keyDown(again, { key: "Enter" });
    await waitFor(() => expect(toast).toHaveBeenCalledWith(expect.stringMatching(/already a file/), "error", 9000));
    // Esc leaves the name as it was
    await fireEvent.keyDown(area, { key: "F2" });
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "New name" }), { key: "Escape" });
    expect(api.RenameFile).toHaveBeenCalledTimes(2);
  });

  it("renames a folder from its menu", async () => {
    await show();
    await fireEvent.contextMenu(folder("Samples"));
    await fireEvent.click(screen.getByRole("button", { name: /^Rename/ }));
    const input = screen.getByRole("textbox", { name: "New name" }) as HTMLInputElement;
    expect(input.selectionEnd).toBe("Samples".length);
    api.RenameFile.mockResolvedValueOnce("Sounds");
    await fireEvent.input(input, { target: { value: "Sounds" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(api.RenameFile).toHaveBeenCalledWith(ROOT, "Samples", "Sounds");
  });

  it("copies files dropped from Explorer into the folder, never over one there", async () => {
    await show();
    await fireEvent.click(folder("Samples"));
    const area = document.querySelector<HTMLElement>(".area")!;
    expect(area.dataset.dir).toBe("Samples");
    expect(area.hasAttribute("data-file-drop-target")).toBe(true);
    api.CopyIntoProject.mockResolvedValueOnce({ copied: ["Samples/pad.wav"], clashes: [] });
    emit("files-dropped", { root: ROOT, dir: "Samples", files: ["D:/Downloads/pad.wav"] });
    await waitFor(() => expect(api.CopyIntoProject).toHaveBeenCalledWith(ROOT, "Samples", ["D:/Downloads/pad.wav"]));
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Copied 1 item into Samples", "ok"));

    api.CopyIntoProject.mockResolvedValueOnce({ copied: [], clashes: ["Kick.wav"] });
    emit("files-dropped", { root: ROOT, dir: "Samples", files: ["D:/Downloads/Kick.wav"] });
    await waitFor(() => expect(toast).toHaveBeenCalledWith(expect.stringMatching(/^Nothing was copied: Kick\.wav is already in Samples/), "error", 10000));
    // another project's drop isn't this one's
    emit("files-dropped", { root: "C:/Other", dir: "", files: ["D:/x.wav"] });
    // its own files dragged out and let go over their folder: nothing to copy
    emit("files-dropped", { root: ROOT, dir: "Samples", files: ["C:\\Song\\Samples\\Kick.wav"] });
    expect(api.CopyIntoProject).toHaveBeenCalledTimes(2);
    // into another folder of the project: a copy
    emit("files-dropped", { root: ROOT, dir: "", files: ["C:\\Song\\Samples\\Kick.wav"] });
    expect(api.CopyIntoProject).toHaveBeenCalledTimes(3);
  });

  it("restores a file's version from its history", async () => {
    api.FileHistory.mockResolvedValue([
      { version: version("h2", "Bass line", "Mia", "2026-10-06T09:00:00Z"), status: "modified", path: "Song.als", from: "" },
      { version: version("h1", "First", "Yi", "2026-10-01T09:00:00Z"), status: "added", path: "Song.als", from: "" },
    ]);
    const props = await show();
    await fireEvent.click(row("Song.als"));
    await screen.findByText("current");
    const list = document.querySelector<HTMLElement>(".timeline")!;
    const items = within(list).getAllByRole("listitem");
    expect(items[0].textContent).toMatch(/current/);
    await fireEvent.click(within(items[1]).getByRole("button", { name: /Restore/ }));
    expect(props.onrestore).toHaveBeenCalledWith("Song.als", "h1", "First", "Song.als");
  });
});
