import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// A project's menu (the sidebar row's right click, a team home card's ⋯),
// with the bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import ProjectMenu from "./ProjectMenu.svelte";
const { api } = mocks;
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; document.body.innerHTML = ""; });

const project = (over = {}) => ({ id: "song", name: "Song", root: "C:/Song", status: "downloaded", branch: "main", ...over }) as never;
const labels = () => [...screen.getByRole("menu", { name: "Song" }).querySelectorAll(":scope > [role=menuitem], :scope > .sub > [role=menuitem]")]
  .map((b) => b.textContent!.replace("›", "").trim());
const props = (over = {}) => ({ p: project(), at: { left: 10, top: 10 }, onsettings: vi.fn(), onclose: vi.fn(), ...over });

describe("ProjectMenu", () => {
  it("offers a project's sets, its folder and its settings", async () => {
    api.ProjectInfo.mockResolvedValue({ openable: [".", "Live/Alt.als"], tool: "Ableton Live" });
    api.OpenPrograms.mockResolvedValue([{ id: "explorer", name: "Explorer" }, { id: "code", name: "VS Code" }]);
    const pr = props();
    render(ProjectMenu, pr);
    await waitFor(() => expect(labels()).toEqual(["Open in Live", "Open folder", "Open the folder in", "Settings…"]));
    // the sets beside it
    await fireEvent.click(screen.getByRole("menuitem", { name: /Open in Live/ }));
    expect(screen.getAllByRole("menuitem").map((b) => b.textContent)).toContain("Live/Alt.als");
    await fireEvent.click(screen.getByRole("menuitem", { name: "Live/Alt.als" }));
    expect(api.OpenInTool).toHaveBeenCalledWith("C:/Song", "Live/Alt.als");
    expect(pr.onclose).toHaveBeenCalled();
  });

  it("lists the programs for the folder (not Explorer: Open folder is it)", async () => {
    api.OpenPrograms.mockResolvedValue([{ id: "explorer", name: "Explorer" }, { id: "code", name: "VS Code" }]);
    render(ProjectMenu, props());
    await fireEvent.click(screen.getByRole("menuitem", { name: /Open the folder in/ }));
    const list = screen.getByRole("menu", { name: "Open the folder in" });
    await waitFor(() => expect([...list.querySelectorAll("[role=menuitem]")].map((b) => b.textContent)).toEqual(["VS Code", "Choose a program…"]));
  });

  it("has the settings only for a project not on this computer, and none for a workspace", () => {
    const pr = props({ p: project({ root: "", status: "remote" }) });
    render(ProjectMenu, pr);
    expect(labels()).toEqual(["Settings…"]);
    expect(api.ProjectInfo).not.toHaveBeenCalled();
    cleanup();
    render(ProjectMenu, props({ p: project({ workspace: true }) }));
    expect(labels()).toEqual(["Open folder", "Open the folder in"]);
  });

  it("goes through its items with the arrows, and Esc gives the focus back to its opener", async () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    const pr = props({ opener });
    render(ProjectMenu, pr);
    await waitFor(() => expect(document.activeElement?.textContent).toBe("Open folder"));
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowDown" });
    expect(document.activeElement?.textContent).toContain("Open the folder in");
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowRight" });
    await waitFor(() => expect(document.activeElement?.textContent).toBe("Choose a program…"));
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowLeft" });
    expect(document.activeElement?.textContent).toContain("Open the folder in");
    expect(screen.queryByRole("menu", { name: "Open the folder in" })).toBeNull();
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowUp" });
    expect(document.activeElement?.textContent).toBe("Open folder");
    await fireEvent.keyDown(document.activeElement!, { key: "ArrowUp" });
    expect(document.activeElement?.textContent).toBe("Settings…");
    await fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(pr.onclose).toHaveBeenCalled();
    expect(document.activeElement).toBe(opener);
  });

  it("closes on a click elsewhere, not on its opener (which toggles it)", async () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    const pr = props({ opener });
    render(ProjectMenu, pr);
    await fireEvent.pointerDown(opener);
    await fireEvent.pointerDown(screen.getByRole("menuitem", { name: "Open folder" }));
    expect(pr.onclose).not.toHaveBeenCalled();
    await fireEvent.pointerDown(document.body);
    expect(pr.onclose).toHaveBeenCalled();
  });
});
