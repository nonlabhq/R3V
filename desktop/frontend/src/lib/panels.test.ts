import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The quick launcher, a team's home and the project's Settings tab, with
// the bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import QuickLaunch from "./QuickLaunch.svelte";
import TeamHome from "./TeamHome.svelte";
import ProjectSettings from "./ProjectSettings.svelte";
const { api } = mocks;
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const project = (name: string, status = "downloaded") => ({ id: name.toLowerCase(), name, root: status === "remote" ? "" : `C:/${name}`, status, branch: "main" });

describe("QuickLaunch", () => {
  const teams = [{ id: "t1", name: "Band" }, { id: "t2", name: "Duo" }];
  const all = [{ team: "t1", projects: [project("Song")] }, { team: "t2", projects: [project("Loop"), project("Beat", "remote")] }];

  it("offers what was opened last and the teams, and finds projects of every team by name", async () => {
    api.AllProjects.mockResolvedValue(all);
    const onpick = vi.fn(), onclose = vi.fn();
    render(QuickLaunch, { teams: teams as never, current: "t1", recent: [{ team: "t2", key: "C:/Loop" }], onpick, onclose });
    const search = screen.getByRole("combobox");
    await waitFor(() => expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      expect.stringContaining("Loop"), expect.stringContaining("Band"), expect.stringContaining("Duo")]));
    await fireEvent.keyDown(search, { key: "ArrowDown" });
    await fireEvent.keyDown(search, { key: "ArrowDown" });
    await fireEvent.keyDown(search, { key: "Enter" });
    expect(onpick).toHaveBeenLastCalledWith({ team: "t2" }); // Duo's home
    await fireEvent.input(search, { target: { value: "BE" } });
    await waitFor(() => expect(screen.getAllByRole("option")).toHaveLength(1));
    await fireEvent.keyDown(search, { key: "Enter" });
    expect(onpick).toHaveBeenLastCalledWith({ team: "t2", project: expect.objectContaining({ name: "Beat" }) });
    await fireEvent.input(search, { target: { value: "zzz" } });
    expect(await screen.findByText("Nothing matches “zzz”")).toBeTruthy();
    await fireEvent.keyDown(search, { key: "Escape" });
    expect(onclose).toHaveBeenCalled();
  });
});

describe("TeamHome", () => {
  it("shows the team, its members and its projects, and opens one", async () => {
    api.TeamMembers.mockResolvedValue([{ id: "m1", name: "Yi" }, { id: "m2", name: "Mia" }]);
    const onopen = vi.fn();
    const song = project("Song"), beat = project("Beat", "remote");
    render(TeamHome, { overview: { teams: [], currentTeam: "t1", projects: [], author: "", teamChecked: true, teamError: "" } as never,
      team: { id: "t1", name: "Band", address: "s3+https://x.r2.cloudflarestorage.com/b", memberId: "m1" } as never,
      projects: [song, beat] as never, open: (p: { name: string }) => p.name === "Song", onopen, onadd: vi.fn(), reload: vi.fn(async () => {}) });
    expect(screen.getByRole("heading", { name: "Band" })).toBeTruthy();
    await waitFor(() => expect(screen.getByRole("list", { name: "Members" }).textContent).toContain("Mia"));
    expect(document.querySelector(".sub")!.textContent!.replace(/\s+/g, " ")).toContain("2 members · 2 projects · R2");
    expect(screen.getByText("open")).toBeTruthy(); // Song only
    await fireEvent.click(screen.getByTitle("Beat"));
    expect(onopen).toHaveBeenCalledWith(beat);
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
