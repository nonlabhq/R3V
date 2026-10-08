import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// File locks on the page: what a lock holds, what locks say in words (the
// .r3v.yaml viewer's too), and the Files tab's padlocks and menu. The Go
// bindings are mocks.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api, toast: vi.fn() };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));

import FileExplorer from "./FileExplorer.svelte";
import { loadLocks, lockAt, lockText, type LocksView } from "./locks.svelte";
import { fileLocksChanges, lockingLines, patternText } from "./lockKinds";
import { rulesChanges } from "./rules";
import type { State } from "./api";
import type { RulesState } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;

const ROOT = "C:/Game";
const KAI = "0000000000000000000000000000000b", ME = "0000000000000000000000000000000a";
const since = new Date(Date.now() - 3 * 86400000).toISOString();
const view = (items: Partial<LocksView["items"][number]>[], extra: Partial<LocksView> = {}): LocksView => ({
  on: true, me: ME, admin: false, autoLock: true, waiting: [], offline: "",
  items: items.map((l) => ({ path: "", prefix: false, memberId: KAI, name: "Kai", since, mine: false, ...l })), ...extra,
});

describe("locks", () => {
  it("finds the lock holding a path: its own, or a folder's over it", () => {
    const v = view([{ path: "Maps/Harbor.umap" }, { path: "Art/", prefix: true, memberId: ME, name: "Yi", mine: true }]);
    expect(lockAt(v, "Maps/Harbor.umap")?.name).toBe("Kai");
    expect(lockAt(v, "Maps/Harbor2.umap")).toBeUndefined();
    expect(lockAt(v, "Art/ship.blend")?.mine).toBe(true);
    expect(lockAt(v, "Art/")?.path).toBe("Art/");
    expect(lockAt(v, "Artwork/x.psd")).toBeUndefined();
    expect(lockAt({ ...v, on: false }, "Maps/Harbor.umap")).toBeUndefined(); // off: none
    expect(lockText(v.items[0])).toBe("Kai is editing this file (locked 3 days ago): changes to it can't be shared until it's unlocked");
    expect(lockText(v.items[1])).toBe("You locked this folder 3 days ago");
  });

  it("says what the project's lock settings make of the team's", () => {
    const none = { disabled: false, autoOff: false, add: [], remove: [] };
    const team = { on: true, kinds: ["unreal", "blender"] };
    expect(patternText("Content/Maps/**")).toBe("everything under Content/Maps/");
    expect(patternText("Content/Dev/")).toBe("everything under Content/Dev/");
    expect(patternText("*.uasset")).toBe(".uasset files");
    expect(lockingLines(null, { ...none, disabled: true })).toEqual([]); // no locks in this R3V
    expect(lockingLines({ on: false, kinds: [] }, none)).toEqual([]); // off and the file says nothing
    expect(lockingLines({ on: false, kinds: [] }, { ...none, disabled: true })[0]).toMatch(/hasn't turned file locking on/);
    expect(lockingLines(team, { ...none, disabled: true })).toEqual(["File locking: off for this project"]);
    expect(lockingLines(team, { ...none, autoOff: true })[0]).toMatch(/^Manual locks only/);
    expect(lockingLines({ on: true, kinds: [] }, none)[0]).toMatch(/^Manual locks only/);
    expect(lockingLines(team, { ...none, add: ["Content/Maps/**"], remove: ["*.uasset"] })).toEqual([
      "Auto-lock (the team's): Unreal levels and assets, Blender files",
      "Auto-lock also: everything under Content/Maps/",
      "Not auto-locked: .uasset files",
    ]);
  });

  it("compares two states of the rules file's lock settings", () => {
    const none = { disabled: false, autoOff: false, add: [], remove: [] };
    expect(fileLocksChanges({ ...none, add: ["Content/Maps/**"] }, { ...none, remove: ["*.uasset"] })).toEqual([
      { kind: "add", text: "Auto-lock also: everything under Content/Maps/" },
      { kind: "del", text: "Auto-locked again: .uasset files" },
    ]);
    const rs = (fileLocks: typeof none) => ({ exists: true, error: "", requires: "", gitignore: false, presets: [], rules: [], options: [],
      fileLocks, teamLocks: { on: true, kinds: [] } }) as unknown as RulesState;
    expect(rulesChanges(rs({ ...none, disabled: true }), rs(none))).toEqual([{ kind: "add", text: "File locking: off for this project" }]);
    expect(rulesChanges(rs(none), rs(none))).toEqual([]);
  });
});

describe("the Files tab's locks", () => {
  const file = (path: string, status = "unchanged") =>
    ({ path, status, size: 10, kind: "other", live: "", from: "", edited: false, preview: false, video: false, model: false, modified: "2026-10-06T10:00:00Z" });
  const st = { name: "Game", head: "h1", branch: "main", branches: [], history: [], changes: [] } as unknown as State;

  beforeEach(() => {
    for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
    toast.mockClear();
    localStorage.clear();
    api.ProjectFiles.mockResolvedValue([file("Harbor.umap", "modified"), file("Hero.uasset"), file("ship.blend"), file("Art/a.psd")]);
    api.LastChanges.mockResolvedValue({});
    api.FileHistory.mockResolvedValue([]);
  });
  afterEach(() => cleanup());

  async function show(v: LocksView) {
    api.ProjectLocks.mockResolvedValue(v);
    await loadLocks(ROOT);
    render(FileExplorer, { root: ROOT, st, onrestore: vi.fn(), ondiscard: vi.fn() });
    await waitFor(() => expect(document.querySelectorAll("[data-path]").length).toBeGreaterThan(0));
  }
  const rowOf = (name: string) => [...document.querySelectorAll<HTMLElement>("[data-path]")].find((r) => r.dataset.path === name)!;
  const menuItems = () => [...document.querySelectorAll(".ctx .item")].map((b) => b.textContent?.trim());

  it("shows who holds a file and a folder, in the list and the grid", async () => {
    await show(view([{ path: "Harbor.umap" }, { path: "Art/", prefix: true, memberId: ME, name: "Yi", mine: true }]));
    expect(rowOf("Harbor.umap").querySelector(".lock")?.getAttribute("title")).toMatch(/^Kai is editing this file/);
    expect(rowOf("Hero.uasset").querySelector(".lock")).toBeNull();
    const art = [...document.querySelectorAll(".tree .folder")].find((b) => b.textContent?.includes("Art"))!;
    expect(art.querySelector(".lock.mine")).not.toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Grid view" }));
    expect(rowOf("Harbor.umap").querySelector(".lock-corner .lock")).not.toBeNull();
  });

  it("shows nothing when locks are off", async () => {
    await show(view([{ path: "Harbor.umap" }], { on: false }));
    expect(document.querySelector(".lock")).toBeNull();
    await fireEvent.contextMenu(rowOf("Hero.uasset"));
    expect(menuItems()).not.toContain("Lock");
  });

  it("locks and unlocks from the menu; an admin breaks someone's lock after asking", async () => {
    await show(view([{ path: "Harbor.umap" }, { path: "ship.blend", memberId: ME, name: "Yi", mine: true }], { admin: true }));
    api.LockFiles.mockResolvedValue({ locked: ["Hero.uasset"], refused: [] });
    await fireEvent.contextMenu(rowOf("Hero.uasset"));
    await fireEvent.click(screen.getByText("Lock"));
    expect(api.LockFiles).toHaveBeenCalledWith(ROOT, ["Hero.uasset"]);

    await fireEvent.contextMenu(rowOf("ship.blend"));
    await fireEvent.click(screen.getByText("Unlock"));
    expect(api.UnlockFiles).toHaveBeenCalledWith(ROOT, ["ship.blend"]);

    await fireEvent.contextMenu(rowOf("Harbor.umap"));
    await fireEvent.click(screen.getByText("Break lock…"));
    expect(api.BreakLock).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("button", { name: "Break lock" }));
    expect(api.BreakLock).toHaveBeenCalledWith(ROOT, "Harbor.umap");
  });

  it("says who holds a file when someone else does (not an admin)", async () => {
    await show(view([{ path: "Harbor.umap" }]));
    await fireEvent.contextMenu(rowOf("Harbor.umap"));
    expect(menuItems()).not.toContain("Lock");
    expect(menuItems()).not.toContain("Break lock…");
    expect(screen.getByText("Locked by Kai")).toBeTruthy();
  });
});
