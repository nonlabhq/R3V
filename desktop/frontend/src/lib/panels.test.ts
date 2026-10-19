import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
import ProjectHeader from "./ProjectHeader.svelte";
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
      projects: [song, beat, { ...song, root: "C:/Song (lamp)", workspace: true, mainRoot: "C:/Song" }] as never,
      open: (p: { name: string }) => p.name === "Song", onopen, onadd: vi.fn(), reload: vi.fn(async () => {}) });
    // Those on this computer, then those on the team only; a workspace counted on its project.
    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual(
      [expect.stringContaining("On this computer"), expect.stringContaining("On the team only")]);
    expect(screen.getAllByTitle(/Song/)).toHaveLength(1);
    expect(screen.getByTitle("C:/Song").textContent).toContain("1 workspace");
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

  it("sets Claude Code up to make steps in a folder that isn't a workspace", async () => {
    api.ProjectInfo.mockResolvedValue({ branch: "main", rules: { applied: [], error: "" } });
    api.Steps.mockResolvedValue({ available: true, on: true, settings: {}, hooked: false, session: null, sessions: [] });
    render(ProjectSettings, { p: project("Song") as never, inline: true, onclose: vi.fn(), onrenamed: vi.fn(), oncheck: vi.fn(), ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn() });
    await fireEvent.click(await screen.findByRole("button", { name: "Workflow" })); // (its tab)
    await fireEvent.click(await screen.findByRole("button", { name: "Set up" }));
    await waitFor(() => expect(api.SetUpSteps).toHaveBeenCalledWith("C:/Song"));
  });

  it("opens the project check (Live version, samples, plugins) of a Live project; other kinds have none", async () => {
    api.ProjectCheck.mockReturnValue(new Promise(() => {}));
    api.ProjectInfo.mockResolvedValue({ branch: "main", tool: "Unity", rules: { applied: [], error: "" } });
    render(ProjectSettings, { p: project("Game") as never, inline: true, onclose: vi.fn(), onrenamed: vi.fn(), oncheck: vi.fn(), ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn() });
    await screen.findByText("Check project…");
    expect(screen.queryByText("Project check")).toBeNull();
    cleanup();
    api.ProjectInfo.mockResolvedValue({ branch: "main", tool: "Ableton Live", rules: { applied: [], error: "" } });
    render(ProjectSettings, { p: project("Song") as never, inline: true, onclose: vi.fn(), onrenamed: vi.fn(), oncheck: vi.fn(), ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn() });
    await fireEvent.click((await screen.findAllByRole("button", { name: "Check…" }))[0]);
    expect(await screen.findByRole("dialog", { name: "Project check" })).toBeTruthy();
    await waitFor(() => expect(api.ProjectCheck).toHaveBeenCalledWith("C:/Song"));
  });
});

describe("ProjectHeader", () => {
  const st = (openable: string[]) => ({ root: "C:/Song", name: "Song", tool: "Ableton Live", openable }) as never;
  const code = { root: "C:/Code", name: "Code", tool: "", openable: [] } as never; // a kind with no tool
  const found = [{ id: "explorer", name: "Explorer", path: "", chosen: false }, { id: "terminal", name: "Terminal", path: "C:/wt.exe", chosen: false },
    { id: "vscode", name: "VS Code", path: "C:/Code.exe", chosen: false }];
  const main = () => document.querySelector<HTMLButtonElement>(".split .main")!;
  const more = () => screen.getByRole("button", { name: "More ways to open the project" });

  it("opens the project in its program with F12, at the far right", async () => {
    render(ProjectHeader, { st: st(["."]) });
    const buttons = [...document.querySelectorAll(".actions > button, .actions > .open-wrap")];
    expect(buttons.at(-1)?.textContent).toContain("Open in Live");
    await fireEvent.keyDown(window, { key: "F12" });
    expect(api.OpenInTool).toHaveBeenCalledWith("C:/Song", ".");
  });

  it("lists the sets with F12 when there are several, above the programs for the folder", async () => {
    api.OpenPrograms.mockResolvedValue(found);
    render(ProjectHeader, { st: st(["A.als", "B.als"]) });
    await fireEvent.keyDown(window, { key: "F12" });
    const menu = screen.getByRole("menu");
    expect(menu.textContent).toContain("B.als");
    expect(api.OpenInTool).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.getAllByRole("menuitem").map((b) => b.textContent)).toEqual(
      ["A.als", "B.als", "Explorer", "Terminal", "VS Code", "Choose a program…"]));
    expect([...menu.querySelectorAll(".menu-h")].map((h) => h.textContent)).toEqual(["Project files", "Open the folder in"]);
    await fireEvent.click(screen.getByRole("menuitem", { name: "B.als" }));
    expect(api.OpenInTool).toHaveBeenCalledWith("C:/Song", "B.als");
  });

  it("with no tool and nothing picked yet, is “Open in…” and opens the menu; the program picked becomes its main action", async () => {
    api.OpenPrograms.mockResolvedValue(found);
    render(ProjectHeader, { st: code });
    expect(main().textContent).toBe("Open in…");
    await fireEvent.click(main());
    expect(api.OpenPrograms).toHaveBeenCalledWith("C:/Code");
    expect(screen.getByRole("menu").textContent).not.toContain("Project files");
    await fireEvent.click(await screen.findByRole("menuitem", { name: "VS Code" }));
    expect(api.OpenFolderWith).toHaveBeenCalledWith("C:/Code", "vscode");
    expect(screen.queryByRole("menu")).toBeNull();
    await waitFor(() => expect(main().textContent).toContain("Open in VS Code"));
    await fireEvent.keyDown(window, { key: "F12" });
    expect(api.OpenFolderWith).toHaveBeenCalledTimes(2);
  });

  it("opens the folder in the program R3V kept for the project", async () => {
    api.OpenedWith.mockImplementation(async (root: string) => (root === "C:/Code" ? { id: "cursor", name: "Cursor" } : null));
    render(ProjectHeader, { st: code });
    await waitFor(() => expect(main().textContent).toContain("Open in Cursor"));
    await fireEvent.click(main());
    expect(api.OpenFolderWith).toHaveBeenCalledWith("C:/Code", "cursor");
    cleanup();
    render(ProjectHeader, { st: { root: "C:/Other", name: "Other", tool: "", openable: [] } as never });
    expect(main().textContent).toBe("Open in…");
  });

  it("opens the folder with a program chosen, which then stays in the menu", async () => {
    const mine = { id: "D:/Apps/My Editor.exe", name: "My Editor", path: "D:/Apps/My Editor.exe", chosen: true };
    api.ChooseProgram.mockResolvedValue(mine);
    api.OpenPrograms.mockResolvedValue(found);
    render(ProjectHeader, { st: code });
    await fireEvent.click(more());
    await fireEvent.click(await screen.findByRole("menuitem", { name: "Choose a program…" }));
    await waitFor(() => expect(api.OpenFolderWith).toHaveBeenCalledWith("C:/Code", "D:/Apps/My Editor.exe"));
    expect(api.ChooseProgram).toHaveBeenCalledWith("C:/Code", expect.any(String));
    expect(main().textContent).toContain("Open in My Editor");
    api.OpenPrograms.mockResolvedValue([...found, mine]);
    await fireEvent.click(more());
    expect(api.OpenPrograms).toHaveBeenLastCalledWith("C:/Code");
    expect(await screen.findByRole("menuitem", { name: "My Editor" })).toBeTruthy();
  });

  it("does nothing when choosing a program is cancelled", async () => {
    api.ChooseProgram.mockResolvedValue(null);
    render(ProjectHeader, { st: code });
    await fireEvent.click(more());
    await fireEvent.click(screen.getByRole("menuitem", { name: "Choose a program…" }));
    await waitFor(() => expect(api.ChooseProgram).toHaveBeenCalled());
    expect(api.OpenFolderWith).not.toHaveBeenCalled();
    expect(main().textContent).toBe("Open in…");
  });

  it("drops a chosen program that is no longer there", async () => {
    // (R3V forgets it once it lists the programs)
    let last: unknown = { id: "D:/Gone.exe", name: "Gone" };
    api.OpenedWith.mockImplementation(async () => last);
    api.OpenPrograms.mockImplementation(async () => { last = null; return found; });
    render(ProjectHeader, { st: code });
    await waitFor(() => expect(main().textContent).toContain("Open in Gone"));
    await fireEvent.click(more());
    await waitFor(() => expect(main().textContent).toBe("Open in…"));
    expect(screen.queryByRole("menuitem", { name: "Gone" })).toBeNull();
  });

  it("moves through the menu with the keyboard; Esc goes back to the ▾", async () => {
    api.OpenPrograms.mockResolvedValue(found);
    render(ProjectHeader, { st: code });
    await fireEvent.click(more());
    expect(more().getAttribute("aria-expanded")).toBe("true");
    await screen.findByRole("menuitem", { name: "VS Code" });
    await waitFor(() => expect(document.activeElement?.textContent).toBe("Explorer"));
    const menu = screen.getByRole("menu");
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    expect(document.activeElement?.textContent).toBe("Terminal");
    await fireEvent.keyDown(menu, { key: "End" });
    expect(document.activeElement?.textContent).toBe("Choose a program…");
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    expect(document.activeElement?.textContent).toBe("Explorer");
    await fireEvent.keyDown(menu, { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(more());
  });
});
