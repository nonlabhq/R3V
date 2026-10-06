import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The new tab and the project's Settings tab, with the bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import NewTab from "./NewTab.svelte";
import ProjectSettings from "./ProjectSettings.svelte";
const { api } = mocks;
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const project = (name: string, status = "downloaded") => ({ id: name.toLowerCase(), name, root: status === "remote" ? "" : `C:/${name}`, status, branch: "main" });

describe("NewTab", () => {
  const overview = (teams: { id: string; name: string }[]) => ({ teams, currentTeam: "t1", projects: [], teamChecked: true, teamError: "" });

  it("opens a project, says which are open, and changes team", async () => {
    const onopen = vi.fn(), reload = vi.fn(async () => {});
    const song = project("Song"), beat = project("Beat", "remote");
    render(NewTab, { overview: overview([{ id: "t1", name: "Band" }, { id: "t2", name: "Duo" }]) as never, projects: [song, beat] as never,
      open: (p: { name: string }) => p.name === "Song", reload, onopen, onadd: vi.fn() });
    expect(screen.getByText("open")).toBeTruthy(); // Song only
    await fireEvent.click(screen.getByTitle("Beat"));
    expect(onopen).toHaveBeenCalledWith(beat);
    await fireEvent.click(screen.getByRole("tab", { name: "Band" })); // the team it's on: nothing
    expect(api.SelectTeam).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("tab", { name: "Duo" }));
    await waitFor(() => expect(reload).toHaveBeenCalled());
    expect(api.SelectTeam).toHaveBeenCalledWith("t2");
  });

  it("shows no team picker for one team", () => {
    render(NewTab, { overview: overview([{ id: "t1", name: "Band" }]) as never, projects: [], open: () => false,
      reload: vi.fn(), onopen: vi.fn(), onadd: vi.fn() });
    expect(screen.queryByRole("tablist")).toBeNull();
  });
});

describe("ProjectSettings in its tab", () => {
  it("is part of the page, renames on Enter, and unlinks in two steps", async () => {
    const onunlink = vi.fn(), onrenamed = vi.fn();
    const p = project("Song");
    render(ProjectSettings, { p: p as never, inline: true, onclose: vi.fn(), onrenamed, oncheck: vi.fn(), ondelete: vi.fn(), onunlink, onlocate: vi.fn() });
    expect(screen.queryByRole("dialog")).toBeNull();
    const name = screen.getByLabelText("Project name");
    await fireEvent.input(name, { target: { value: "Song 2" } });
    await fireEvent.keyDown(name, { key: "Enter" });
    await waitFor(() => expect(api.RenameProject).toHaveBeenCalledWith("", "song", "C:/Song", "Song 2"));
    await fireEvent.click(screen.getByRole("button", { name: "Unlink…" }));
    expect(onunlink).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("button", { name: "Unlink" }));
    expect(onunlink).toHaveBeenCalled();
  });
});
