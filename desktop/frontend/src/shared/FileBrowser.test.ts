import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import FileBrowser from "./FileBrowser.svelte";

// The Files explorer as R3V Cloud's web app uses it: files and their last
// versions only, nothing of the App's (no renaming, no menu, no dragging).

globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;
afterEach(() => { cleanup(); localStorage.clear(); });

const files = [
  { path: "Game.uproject", status: "unchanged", size: 300 },
  { path: "Content/Maps/Harbor.umap", status: "modified", size: 9000 },
  { path: "Content/Art/ship.png", status: "unchanged", size: 4000, preview: true },
];
const panel = createRawSnippet(() => ({ render: () => `<p class="shown"></p>` }));

describe("FileBrowser", () => {
  it("lists a folder's files and folders, opens a folder, shows the file picked, and offers no renaming", async () => {
    render(FileBrowser, { files, loaded: true, name: "Game", remember: "p1",
      last: { "Game.uproject": { message: "First\n\nmore", author: "Robin", authorId: "m1", time: "2026-10-10T10:00:00Z" } }, panel });
    expect(screen.getByText("Game.uproject")).toBeTruthy();
    expect(screen.getByText("First")).toBeTruthy(); // (its last version's title)
    const content = [...document.querySelectorAll<HTMLElement>(".row.sub")].find((r) => r.title === "Content")!;
    await fireEvent.dblClick(content);
    await fireEvent.dblClick([...document.querySelectorAll<HTMLElement>(".row.sub")].find((r) => r.title === "Content/Maps")!);
    const map = document.querySelector<HTMLElement>('[data-path="Content/Maps/Harbor.umap"]')!;
    await fireEvent.click(map);
    expect(map.classList.contains("on")).toBe(true);
    await fireEvent.keyDown(screen.getByRole("grid", { name: "Files" }), { key: "F2" });
    expect(document.querySelector("input.rename")).toBeNull();
    expect(document.querySelector("[data-file-drop-target]")).toBeNull();
  });
});
