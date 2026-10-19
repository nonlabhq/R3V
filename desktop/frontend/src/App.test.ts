import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

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

// jsdom has no Web Animations, which the sidebar's rows slide with: each
// one ends at once.
Element.prototype.getAnimations ??= () => [];
Element.prototype.animate ??= function () {
  let done: (() => void) | null = null;
  return { cancel() {}, playState: "finished", effect: null,
    get onfinish() { return done; }, set onfinish(f: (() => void) | null) { done = f; if (f) setTimeout(f); } } as unknown as Animation;
};

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
    // in the tab shown, as a click in the sidebar does: a tab of the team's home
    expect(tabs()).toEqual([expect.stringContaining("Band")]);
    expect(screen.getByRole("tab").getAttribute("aria-selected")).toBe("true");
    await fireEvent.click(screen.getByRole("button", { name: "Team settings" }));
    expect(await screen.findByRole("textbox", { name: "Team name" })).toBeTruthy();
    // a project from the home opens in its tab, the home's stays
    await fireEvent.click(screen.getByRole("button", { name: "Projects" }));
    await fireEvent.click(screen.getAllByRole("button", { name: /^Beat/ }).at(-1)!);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Band"), expect.stringContaining("Beat")]));
    expect(screen.queryByRole("heading", { name: "Band" })).toBeNull();
  });

  const middleClick = (el: Element) => fireEvent(el, new MouseEvent("auxclick", { button: 1, bubbles: true, cancelable: true }));
  const on = () => screen.getAllByRole("tab").find((t) => t.getAttribute("aria-selected") === "true")?.textContent?.trim();
  const inSidebar = (name: string) => screen.getAllByRole("button", { name: new RegExp(`^${name}`) })[0];

  it("shows a project clicked in the sidebar in the tab shown, or goes to its tab", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.click(inSidebar("Beat"));
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Beat")]));
    expect(on()).toContain("Beat");
    expect(JSON.parse(localStorage.getItem("r3v.tabs")!).map((x: { key: string }) => x.key)).toEqual(["C:/Beat"]);
    // open already in another tab: that tab is gone to, none replaced
    await middleClick(inSidebar("Song"));
    await waitFor(() => expect(tabs()).toHaveLength(2));
    await fireEvent.click(inSidebar("Song"));
    expect(tabs()).toEqual([expect.stringContaining("Beat"), expect.stringContaining("Song")]);
    expect(on()).toContain("Song");
  });

  it("opens a project in a tab behind with a middle click or Ctrl+click, and closes a tab with a middle click", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await middleClick(inSidebar("Beat"));
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Beat")]));
    expect(on()).toContain("Song"); // (behind: the one shown stays)
    // the middle click on a tab closes it, as × does
    await middleClick(screen.getAllByRole("tab")[0].closest(".tab")!);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Beat")]));
    expect(on()).toContain("Beat");
    await fireEvent.click(inSidebar("Song"), { ctrlKey: true });
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Beat"), expect.stringContaining("Song")]));
    expect(on()).toContain("Beat");
  });

  it("opens the team's home as a tab of its own, drawn with the team's mark, and keeps it", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await middleClick(document.querySelector(".team-menu .switch")!);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Band")]));
    expect(on()).toContain("Song");
    const [song, band] = screen.getAllByRole("tab");
    expect(band.title).toBe("Band · Team home");
    expect(band.querySelector(".pi")!.classList.contains("solid")).toBe(true); // a team's mark
    expect(song.querySelector(".pi")!.classList.contains("solid")).toBe(false); // a project's icon
    await fireEvent.click(band);
    expect(await screen.findByRole("heading", { name: "Band" })).toBeTruthy();
    expect(on()).toContain("Band");
    expect(JSON.parse(localStorage.getItem("r3v.tabs")!).map((x: { key: string }) => x.key)).toEqual(["C:/Song", "home:"]);
    // kept: R3V opened again starts on its first tab, the home on its own
    cleanup();
    localStorage.setItem("r3v.tabs", JSON.stringify([{ team: "t", key: "home:" }, { team: "t", key: "C:/Song" }]));
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Band"), expect.stringContaining("Song")]));
    expect(on()).toContain("Band");
    expect(await screen.findByRole("heading", { name: "Band" })).toBeTruthy();
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
    await fireEvent.click(screen.getAllByRole("button", { name: /^Beat/ })[0], { ctrlKey: true }); // a second tab
    await waitFor(() => expect(tabs()).toHaveLength(2));
    expect(on()).toContain("Song");
    await fireEvent.keyDown(window, { key: "Tab", ctrlKey: true });
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

  it("says the team is being asked by the turning refresh only, no line that moves the projects", async () => {
    api.LocalOverview.mockResolvedValue({ ...overview(), teamChecked: false });
    api.Overview.mockReturnValue(new Promise(() => {})); // the team never answers
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(screen.queryByText(/checking…/)).toBeNull();
    expect(screen.getByRole("button", { name: "Check the team for new projects" }).classList.contains("spin")).toBe(true);
  });

  it("moves a dragged project among the others as it goes, following the pointer, and keeps the order", async () => {
    // (jsdom lays nothing out: rows 40px high, one under the other)
    const rowOf = (el: HTMLElement) => (el.matches("[data-key]") ? [...el.parentElement!.children].indexOf(el) : 0);
    const top = vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockImplementation(function (this: HTMLElement) { return rowOf(this) * 40; });
    const high = vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(40);
    try {
      render(App);
      await waitFor(() => expect(document.querySelectorAll("li[data-key]")).toHaveLength(2));
      const keysShown = () => [...document.querySelectorAll<HTMLElement>("li[data-key]")].map((el) => el.dataset.key);
      expect(keysShown()).toEqual(["song", "beat"]);
      const song = document.querySelector<HTMLElement>("li[data-key='song']")!;
      await fireEvent.pointerDown(song, { button: 0, clientY: 20 });
      await fireEvent.pointerMove(window, { buttons: 1, clientY: 70 }); // past Beat's middle
      await waitFor(() => expect(keysShown()).toEqual(["beat", "song"]));
      expect(song.classList.contains("lifted")).toBe(true);
      // where the pointer is: 50px down from where it took it, its place 40px down
      await waitFor(() => expect(song.style.transform).toBe("translateY(10px)"));
      await fireEvent.pointerUp(window);
      expect(song.classList.contains("lifted")).toBe(false);
      expect(song.style.transform).toBe("");
      expect(JSON.parse(localStorage.getItem("r3v.order:t")!)).toEqual(["beat", "song"]);
    } finally {
      top.mockRestore();
      high.mockRestore();
    }
  });

  it("lists the shortcuts (Ctrl+/), and Esc closes the list", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.keyDown(window, { key: "/", code: "Slash", ctrlKey: true });
    const help = await screen.findByRole("dialog", { name: "Keyboard shortcuts" });
    // and what the mouse does: a new tab from the sidebar, a tab closed
    const lines = [...help.querySelectorAll("dt")].map((d) => `${d.textContent} = ${d.nextElementSibling?.textContent}`);
    expect(lines).toContain("Middle click · Ctrl+click = In the sidebar: open a project or team in a new tab");
    expect(lines).toContain("Middle click on a tab = Close the tab");
    // (tabs don't move under a dialog)
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    expect(tabs()).toHaveLength(1);
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("sizes the sidebar by its edge, folds it past its narrowest, and opens it again", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    const shell = document.querySelector<HTMLElement>(".shell")!;
    const edge = document.querySelector<HTMLElement>(".side-split")!;
    const drag = async (to: number) => {
      await fireEvent.pointerDown(edge, { button: 0, clientX: 256 });
      await fireEvent.pointerMove(window, { buttons: 1, clientX: to });
      await fireEvent.pointerUp(window);
    };
    await drag(320);
    expect(shell.style.gridTemplateColumns).toBe("320px minmax(0, 1fr)");
    expect(localStorage.getItem("r3v.sidebarWidth")).toBe("320");
    await drag(170); // narrowest, not folded yet
    expect(shell.classList.contains("folded")).toBe(false);
    expect(shell.style.gridTemplateColumns).toBe("200px minmax(0, 1fr)");
    await drag(100);
    expect(shell.classList.contains("folded")).toBe(true);
    await drag(240);
    expect(shell.classList.contains("folded")).toBe(false);
    expect(shell.style.gridTemplateColumns).toBe("240px minmax(0, 1fr)");
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
  const teamItem = (name: string) => within(document.querySelector<HTMLElement>(".team-menu .menu")!).getByRole("button", { name: new RegExp(name) });

  const snap = (name: string) => ({ id: name.toLowerCase(), root: `C:/${name}`, name, icon: "", color: "", status: "downloaded" });

  it("shows a team picked in the team menu in the tab shown, keeps the other tabs, and goes back from them", async () => {
    localStorage.setItem("r3v.tabs", JSON.stringify([{ team: "a", key: "C:/Song", p: snap("Song") }, { team: "a", key: "C:/Beat", p: snap("Beat") }]));
    render(App);
    await waitFor(() => expect(on()).toContain("Song"));
    await fireEvent.click(teamMenu());
    await fireEvent.click(teamItem("Duo"));
    await waitFor(() => expect(teamShown()).toContain("Duo"));
    // team b's home shows in the tab that showed Song; Beat's tab stays
    expect(await screen.findByRole("heading", { name: "Duo" })).toBeTruthy();
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Duo"), expect.stringContaining("Beat")]));
    expect(on()).toContain("Duo");
    // its project shows in that tab too
    await fireEvent.click(screen.getAllByRole("button", { name: /^Loop/ })[0]);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Loop"), expect.stringContaining("Beat")]));
    expect(on()).toContain("Loop");
    expect(screen.getAllByRole("tab")[1].title).toBe("Band · Beat\nC:/Beat");
    // tabs of two teams: each says which
    expect(document.querySelectorAll(".tab-team")).toHaveLength(2);
    // remembered as one list
    expect(JSON.parse(localStorage.getItem("r3v.tabs")!).map((x: { team: string; key: string }) => `${x.team}:${x.key}`))
      .toEqual(["b:C:/Loop", "a:C:/Beat"]);

    // team a's tab: the sidebar goes to team a, and its project shows
    await fireEvent.click(screen.getAllByRole("tab")[1]);
    await waitFor(() => expect(teamShown()).toContain("Band"));
    expect(api.SelectTeam).toHaveBeenLastCalledWith("a");
    await waitFor(() => expect(on()).toContain("Beat"));
    expect(screen.getAllByRole("button", { name: /^Song/ }).length).toBeGreaterThan(0); // team a's list
    expect(tabs()).toEqual([expect.stringContaining("Loop"), expect.stringContaining("Beat")]);
  });

  it("goes to the team's home tab when it is open already, from the team menu", async () => {
    localStorage.setItem("r3v.tabs", JSON.stringify([{ team: "a", key: "C:/Song", p: snap("Song") }, { team: "b", key: "home:" }]));
    render(App);
    await waitFor(() => expect(on()).toContain("Song"));
    await fireEvent.click(teamMenu());
    await fireEvent.click(teamItem("Duo"));
    await waitFor(() => expect(teamShown()).toContain("Duo"));
    await waitFor(() => expect(on()).toContain("Duo"));
    expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Duo")]);
  });

  it("opens a team's home in a tab behind with a middle click or Ctrl+click in the team menu", async () => {
    localStorage.setItem("r3v.tabs", JSON.stringify([{ team: "a", key: "C:/Song", p: snap("Song") }]));
    render(App);
    await waitFor(() => expect(on()).toContain("Song"));
    await fireEvent.click(teamMenu());
    await fireEvent.click(teamItem("Duo"), { ctrlKey: true });
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Duo")]));
    expect(on()).toContain("Song");
    expect(teamShown()).toContain("Band");
    expect(api.SelectTeam).not.toHaveBeenCalled();
    // (closed again; a middle click opens it the same way)
    await fireEvent.click(screen.getAllByRole("tab")[1].parentElement!.querySelector(".tab-x")!);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.click(teamMenu());
    const duo = teamItem("Duo");
    await fireEvent.mouseDown(duo, { button: 1 });
    await fireEvent(duo, new MouseEvent("auxclick", { button: 1, bubbles: true }));
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Duo")]));
    expect(on()).toContain("Song");
    expect(api.SelectTeam).not.toHaveBeenCalled();
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

  it("goes to another team's home from its tab, the sidebar on that team", async () => {
    localStorage.setItem("r3v.tabs", JSON.stringify([
      { team: "a", key: "C:/Song", p: { id: "song", root: "C:/Song", name: "Song", icon: "", color: "", status: "downloaded" } },
      { team: "b", key: "home:" },
    ]));
    render(App);
    await waitFor(() => expect(on()).toContain("Song"));
    await fireEvent.click(screen.getAllByRole("tab")[1]);
    await waitFor(() => expect(teamShown()).toContain("Duo"));
    expect(await screen.findByRole("heading", { name: "Duo" })).toBeTruthy();
    expect(on()).toContain("Duo");
    // a click on a project in the sidebar shows it in that tab
    await fireEvent.click(screen.getAllByRole("button", { name: /^Loop/ })[0]);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song"), expect.stringContaining("Loop")]));
    expect(on()).toContain("Loop");
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

describe("App: a project's right-click menu", () => {
  const found = [{ id: "explorer", name: "Explorer", path: "", chosen: false }, { id: "vscode", name: "VS Code", path: "C:/Code.exe", chosen: false },
    { id: "D:/Apps/My Editor.exe", name: "My Editor", path: "D:/Apps/My Editor.exe", chosen: true }];
  const rowOf = (name: string) => screen.getAllByRole("button", { name: new RegExp(`^${name}`) })[0];
  async function menuOf(name: string) {
    await fireEvent.contextMenu(rowOf(name), { clientX: 20, clientY: 40 });
    return document.querySelector<HTMLElement>(".row-menu")!;
  }

  it("opens the folder in the programs the header's Open in lists, or one chosen", async () => {
    api.ProjectInfo.mockResolvedValue({ tool: "Ableton Live", openable: ["."] });
    api.OpenPrograms.mockResolvedValue(found);
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    const menu = await menuOf("Song");
    const sub = within(menu).getByRole("button", { name: /Open the folder in/ });
    await fireEvent.mouseEnter(sub.parentElement!);
    const list = await screen.findByRole("menu", { name: "Open the folder in" });
    await waitFor(() => expect(within(list).getAllByRole("menuitem").map((b) => b.textContent)).toEqual(
      ["VS Code", "My Editor", "Choose a program…"])); // (not Explorer: "Open folder" is in the menu)
    expect(api.OpenPrograms).toHaveBeenCalledWith("C:/Song");
    await fireEvent.click(within(list).getByRole("menuitem", { name: "My Editor" }));
    expect(api.OpenFolderWith).toHaveBeenCalledWith("C:/Song", "D:/Apps/My Editor.exe");
    expect(document.querySelector(".row-menu")).toBeNull();

    // Choose a program…: kept for the project by R3V, and the folder opened in it
    api.ChooseProgram.mockResolvedValue({ id: "D:/Other.exe", name: "Other", path: "D:/Other.exe", chosen: true });
    const again = await menuOf("Beat");
    await fireEvent.mouseEnter(within(again).getByRole("button", { name: /Open the folder in/ }).parentElement!);
    await fireEvent.click(await screen.findByRole("menuitem", { name: "Choose a program…" }));
    expect(api.ChooseProgram).toHaveBeenCalledWith("C:/Beat", expect.any(String));
    await waitFor(() => expect(api.OpenFolderWith).toHaveBeenLastCalledWith("C:/Beat", "D:/Other.exe"));
  });
});

describe("App: what Open in kept in the page before", () => {
  it("sends the programs the page kept for projects to R3V once, and forgets them", async () => {
    localStorage.setItem("r3v.openWith:C:/Song", JSON.stringify({ last: { id: "vscode", name: "VS Code" }, chosen: ["D:/Apps/My Editor.exe", 3] }));
    localStorage.setItem("r3v.openWith:C:/Beat", "{broken");
    render(App);
    await waitFor(() => expect(api.ImportOpenWith).toHaveBeenCalledWith({
      "C:/Song": { last: { id: "vscode", name: "VS Code" }, chosen: ["D:/Apps/My Editor.exe"] } }));
    await waitFor(() => expect(localStorage.getItem("r3v.openWith:C:/Song")).toBeNull());
    expect(localStorage.getItem("r3v.openWith:C:/Beat")).toBeNull();
    cleanup();
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(api.ImportOpenWith).toHaveBeenCalledTimes(1);
  });

  it("keeps them while R3V can't take them", async () => {
    localStorage.setItem("r3v.openWith:C:/Song", JSON.stringify({ chosen: ["D:/Apps/My Editor.exe"] }));
    api.ImportOpenWith.mockRejectedValue(new Error("busy"));
    render(App);
    await waitFor(() => expect(api.ImportOpenWith).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 10));
    expect(localStorage.getItem("r3v.openWith:C:/Song")).not.toBeNull();
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
