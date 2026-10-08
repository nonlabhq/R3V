import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The window around the projects: tabs and their shortcuts, with the Go
// bindings mocked (every one resolves to null unless set) and the project
// page replaced by a stub.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./lib/api", async (orig) => ({ ...(await orig<typeof import("./lib/api")>()), api: mocks.api }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: () => () => {} },
  Window: { Minimise: async () => {}, ToggleMaximise: async () => {}, Close: async () => {} } }));
vi.mock("./lib/ProjectView.svelte", async () => ({ default: (await import("./test/Stub.svelte")).default }));

import App from "./App.svelte";
const { api } = mocks;

const project = (name: string) => ({ id: name.toLowerCase(), name, root: `C:/${name}`, status: "downloaded", branch: "main" });
const overview = () => ({
  teams: [{ id: "t", name: "Band", memberName: "Yi" }], currentTeam: "t",
  projects: [project("Song"), project("Beat")], teamChecked: true, teamError: "",
});

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  localStorage.clear();
  api.LocalOverview.mockResolvedValue(overview());
  api.Overview.mockResolvedValue(overview());
  api.UpdateStatus.mockResolvedValue({ auto: false, available: null, progress: null });
});
afterEach(() => cleanup());

const tabs = () => screen.queryAllByRole("tab").map((t) => t.textContent?.trim());

describe("App: tabs", () => {
  it("shows the team's home once the last tab is closed, also when the projects are read again", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.click(screen.getByRole("button", { name: "Close Song" }));
    await screen.findByRole("button", { name: "Team settings" });
    const reads = api.Overview.mock.calls.length;
    await fireEvent.keyDown(window, { key: "F5" }); // reads the overview again
    await waitFor(() => expect(api.Overview.mock.calls.length).toBeGreaterThan(reads));
    await new Promise((r) => setTimeout(r, 20));
    expect(tabs()).toEqual([]);
    expect(screen.getByRole("heading", { name: "Band" })).toBeTruthy();
  });

  it("shows the team's home from the team, and its settings from there", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.click(document.querySelector<HTMLElement>(".team-menu .switch")!);
    expect(await screen.findByRole("heading", { name: "Band" })).toBeTruthy();
    expect(screen.getAllByRole("tab").some((x) => x.getAttribute("aria-selected") === "true")).toBe(false);
    await fireEvent.click(screen.getByRole("button", { name: "Team settings" }));
    expect(await screen.findByRole("textbox", { name: "Team name" })).toBeTruthy();
    // a project from the home opens in its tab
    await fireEvent.click(screen.getByRole("button", { name: "Projects" }));
    await fireEvent.click(screen.getAllByRole("button", { name: /^Beat/ }).at(-1)!);
    await waitFor(() => expect(tabs()).toHaveLength(2));
    expect(screen.queryByRole("heading", { name: "Band" })).toBeNull();
  });

  it("opens the quick launcher with Ctrl+T, but not held down, nor while typing", async () => {
    api.AllProjects.mockResolvedValue([{ team: "t", projects: overview().projects }]);
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.keyDown(window, { key: "t", ctrlKey: true, repeat: true });
    const field = document.createElement("input");
    document.body.append(field);
    await fireEvent.keyDown(field, { key: "t", ctrlKey: true });
    field.remove();
    expect(screen.queryByRole("dialog")).toBeNull();
    await fireEvent.keyDown(window, { key: "t", ctrlKey: true });
    const search = await screen.findByRole("combobox", { name: "Open a project or team…" });
    // a project found by its name: Enter opens it in a tab
    await waitFor(() => expect(api.AllProjects).toHaveBeenCalled());
    await fireEvent.input(search, { target: { value: "bea" } });
    await waitFor(() => expect(screen.getByRole("option", { selected: true }).textContent).toContain("Beat"));
    await fireEvent.keyDown(search, { key: "Enter" });
    await waitFor(() => expect(tabs()).toHaveLength(2));
    expect(screen.queryByRole("dialog")).toBeNull();
    // Esc closes it
    await fireEvent.keyDown(window, { key: "t", ctrlKey: true });
    await fireEvent.keyDown(await screen.findByRole("combobox"), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("goes through, closes and reopens tabs, and folds the sidebar, with the keyboard", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.click(screen.getAllByRole("button", { name: /^Beat/ })[0]); // a second tab
    await waitFor(() => expect(tabs()).toHaveLength(2));
    const on = () => screen.getAllByRole("tab").find((t) => t.getAttribute("aria-selected") === "true")?.textContent?.trim();
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "Tab", ctrlKey: true });
    expect(on()).toContain("Song"); // (round to the first)
    await fireEvent.keyDown(window, { key: "Tab", ctrlKey: true, shiftKey: true });
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "1", ctrlKey: true });
    expect(on()).toContain("Song");
    await fireEvent.keyDown(window, { key: "9", ctrlKey: true });
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.keyDown(window, { key: "T", ctrlKey: true, shiftKey: true });
    await waitFor(() => expect(tabs()).toHaveLength(2));
    expect(on()).toContain("Beat");
    expect(screen.getByRole("button", { name: "Hide the sidebar" }).title).toBe("Hide the sidebar (Ctrl+\\)");
    const shell = document.querySelector(".shell")!;
    expect(shell.classList.contains("folded")).toBe(false);
    await fireEvent.keyDown(window, { key: "\\", code: "Backslash", ctrlKey: true });
    expect(shell.classList.contains("folded")).toBe(true);
    await fireEvent.keyDown(window, { key: "\\", code: "Backslash", ctrlKey: true });
    expect(shell.classList.contains("folded")).toBe(false);
  });

  it("lists the shortcuts (Ctrl+/), and Esc closes the list", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.keyDown(window, { key: "/", code: "Slash", ctrlKey: true });
    expect(await screen.findByRole("dialog", { name: "Keyboard shortcuts" })).toBeTruthy();
    // (tabs don't move under a dialog)
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    expect(tabs()).toHaveLength(1);
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("shows the version short, says Nightly, and copies it in full", async () => {
    api.Version.mockResolvedValue("0.1.3-nightly.202610070525");
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(App);
    const v = await screen.findByTitle(/0\.1\.3-nightly\.202610070525/);
    expect(v.firstChild?.textContent?.trim()).toBe("v0.1.3");
    expect(v.querySelector(".channel")?.textContent).toBe("Nightly");
    await fireEvent.click(v);
    expect(writeText).toHaveBeenCalledWith("0.1.3-nightly.202610070525");
  });
});

describe("App: tabs of more than one team", () => {
  // Two teams; the overview is the selected one's.
  const teams = [{ id: "a", name: "Band", memberName: "Yi" }, { id: "b", name: "Duo", memberName: "Yi" }];
  const lists: Record<string, ReturnType<typeof project>[]> = { a: [project("Song"), project("Beat")], b: [project("Loop")] };
  let currentTeam = "a";
  const teamOverview = () => ({ teams, currentTeam, projects: lists[currentTeam], teamChecked: true, teamError: "" });
  beforeEach(() => {
    currentTeam = "a";
    api.LocalOverview.mockImplementation(async () => teamOverview());
    api.Overview.mockImplementation(async () => teamOverview());
    api.SelectTeam.mockImplementation(async (id: string) => { currentTeam = id; });
  });
  const on = () => screen.getAllByRole("tab").find((t) => t.getAttribute("aria-selected") === "true")?.textContent?.trim();
  const teamMenu = () => document.querySelector<HTMLElement>(".team-menu .caret")!;
  const teamShown = () => document.querySelector<HTMLElement>(".team-menu .switch")!.textContent;

  it("keeps a team's tabs when another team is picked, and goes back to it from its tab", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.click(teamMenu());
    await fireEvent.click(screen.getByRole("button", { name: /Duo/ }));
    await waitFor(() => expect(teamShown()).toContain("Duo"));
    // team b's home shows; its project opens in a tab of its own, and team a's stays
    expect(await screen.findByRole("heading", { name: "Duo" })).toBeTruthy();
    await fireEvent.click(screen.getAllByRole("button", { name: /^Loop/ })[0]);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Loop")]));
    expect(on()).toContain("Loop");
    expect(screen.getAllByRole("tab")[0].title).toBe("Band · Song\nC:/Song");
    // tabs of two teams: each says which
    expect(document.querySelectorAll(".tab-team")).toHaveLength(2);
    // remembered as one list
    expect(JSON.parse(localStorage.getItem("r3v.tabs")!).map((x: { team: string; key: string }) => `${x.team}:${x.key}`))
      .toEqual(["a:C:/Song", "b:C:/Loop"]);

    // team a's tab: the sidebar goes to team a, and its project shows
    await fireEvent.click(screen.getAllByRole("tab")[0]);
    await waitFor(() => expect(teamShown()).toContain("Band"));
    expect(api.SelectTeam).toHaveBeenLastCalledWith("a");
    await waitFor(() => expect(on()).toContain("Song"));
    expect(screen.getAllByRole("button", { name: /^Beat/ }).length).toBeGreaterThan(0); // team a's list
    expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Loop")]);
  });

  it("closes the shown tab to the next one, on its team", async () => {
    localStorage.setItem("r3v.tabs", JSON.stringify([
      { team: "a", key: "C:/Song", p: { id: "song", root: "C:/Song", name: "Song", icon: "", color: "", status: "downloaded" } },
      { team: "b", key: "C:/Loop", p: { id: "loop", root: "C:/Loop", name: "Loop", icon: "", color: "", status: "downloaded" } },
    ]));
    render(App);
    await waitFor(() => expect(on()).toContain("Song"));
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    await waitFor(() => expect(teamShown()).toContain("Duo"));
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Loop")]));
    expect(on()).toContain("Loop");
    // and back again
    await fireEvent.keyDown(window, { key: "T", ctrlKey: true, shiftKey: true });
    await waitFor(() => expect(teamShown()).toContain("Band"));
    await waitFor(() => expect(on()).toContain("Song"));
  });

  it("starts the one list from the team's old one, and drops tabs of teams gone", async () => {
    localStorage.setItem("r3v.tabs:a", JSON.stringify(["C:/Beat", "C:/Gone"]));
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Beat")]));
    localStorage.setItem("r3v.tabs", JSON.stringify([{ team: "x", key: "C:/Old", p: { name: "Old" } }, { team: "a", key: "C:/Song" }]));
    cleanup();
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    expect(localStorage.getItem("r3v.tabs")).not.toContain("C:/Old");
  });
});

describe("App: the user", () => {
  it("opens User settings from the user area (Nightly), and shows their picture there", async () => {
    api.Profile.mockResolvedValue({ available: true, name: "Yi", memberId: "m1", color: "b2",
      picture: "data:image/png;base64,AAAA", notShared: [] });
    render(App);
    const who = await screen.findByRole("button", { name: /Yi/ });
    expect(who.querySelector("img")?.getAttribute("src")).toBe("data:image/png;base64,AAAA");
    await fireEvent.click(who);
    expect(await screen.findByRole("dialog", { name: "User settings" })).toBeTruthy();
  });

  it("keeps the user area as it was where looks aren't (Stable)", async () => {
    api.Profile.mockResolvedValue({ available: false, name: "Yi", memberId: "m1", color: "", picture: "", notShared: [] });
    render(App);
    await screen.findByText("Yi");
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.queryByRole("button", { name: /Yi/ })).toBeNull();
  });
});
