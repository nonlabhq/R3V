import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// Sections (docs/design/sections.md): the left panel groups the projects
// by the team's sections (the ungrouped first), each folding; those who may
// drag a project onto a section and change sections from a header's menu
// or the team's menu; the team's home groups its cards the same way; a
// project's settings pick its section; its header says it; Activity says
// what changed. Go bindings mocked.
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

Element.prototype.getAnimations ??= () => [];
document.elementFromPoint ??= () => null; // (jsdom has none: the tests say what is under the pointer)
Element.prototype.animate ??= function () {
  let done: (() => void) | null = null;
  return { cancel() {}, playState: "finished", effect: null,
    get onfinish() { return done; }, set onfinish(f: (() => void) | null) { done = f; if (f) setTimeout(f); } } as unknown as Animation;
};

import App from "./App.svelte";
import TeamHome from "./lib/TeamHome.svelte";
import ProjectSettings from "./lib/ProjectSettings.svelte";
import ProjectHeader from "./lib/ProjectHeader.svelte";
import TeamActivity from "./shared/team/TeamActivity.svelte";
import { sectionFolds } from "./lib/sections.svelte";
const { api } = mocks;

const project = (name: string, section = "", sectionName = "") =>
  ({ id: name.toLowerCase(), name, root: `C:/${name}`, status: "downloaded", branch: "main", section, sectionName });
const engine = { id: "s1", name: "Engine", projects: ["beat"] };
const tools = { id: "s2", name: "Tools", projects: [] as string[] };
const overview = (canEditSections = true, sections = [engine, tools]) => ({
  teams: [{ id: "t", name: "Band", memberName: "Yi", canEditSections }], currentTeam: "t", sectionsOn: true, sections,
  projects: [project("Song"), project("Beat", "s1", "Engine")], teamChecked: true, teamError: "",
});

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  for (const k of Object.keys(sectionFolds)) delete sectionFolds[k];
  localStorage.clear();
  api.LocalOverview.mockResolvedValue(overview());
  api.Overview.mockResolvedValue(overview());
  api.UpdateStatus.mockResolvedValue({ auto: false, available: null, progress: null });
});
afterEach(() => cleanup());

const tabs = () => screen.queryAllByRole("tab");
const aside = () => document.querySelector("aside")!;
const list = (name: string) => aside().querySelector<HTMLElement>(`ul[aria-label="${name}"]`)!;
const header = (name: string) => [...aside().querySelectorAll<HTMLElement>(".sect-h")].find((h) => h.textContent?.includes(name))!;

describe("Sections in the left panel", () => {
  it("lists the ungrouped projects first, then each section with its own, and folds a section", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    const order = [...aside().querySelectorAll(".sect-h .sect-name, li[data-key] .name")].map((e) => e.textContent);
    expect(order).toEqual(["Song", "Engine", "Beat", "Tools"]);
    expect(within(list("Engine")).getByText("Beat")).toBeTruthy();
    const fold = within(header("Engine")).getByRole("button", { name: /Engine/ });
    expect(fold.getAttribute("aria-expanded")).toBe("true");
    await fireEvent.click(fold);
    expect(fold.getAttribute("aria-expanded")).toBe("false");
    expect(within(list("Engine")).queryByText("Beat")).toBeNull();
    expect(JSON.parse(localStorage.getItem("r3v.sectionsFolded")!)).toEqual({ s1: true });
  });

  it("puts a project dragged onto a section in it, for the team", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    const song = aside().querySelector<HTMLElement>("li[data-key='song']")!;
    const target = header("Tools");
    const at = vi.spyOn(document, "elementFromPoint").mockReturnValue(target);
    try {
      await fireEvent.pointerDown(song, { button: 0, clientY: 20 });
      await fireEvent.pointerMove(window, { buttons: 1, clientY: 140 });
      await waitFor(() => expect(header("Tools").classList.contains("drop")).toBe(true));
      await fireEvent.pointerUp(window);
      await waitFor(() => expect(api.SetProjectSection).toHaveBeenCalledWith("t", "song", "s2"));
      // (shown there at once, before the team's list says so)
      await waitFor(() => expect(within(list("Tools")).getByText("Song")).toBeTruthy());
    } finally {
      at.mockRestore();
    }
  });

  it("takes a project out of its section when dragged onto the ungrouped ones", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    const beat = aside().querySelector<HTMLElement>("li[data-key='beat']")!;
    const at = vi.spyOn(document, "elementFromPoint").mockReturnValue(aside().querySelector<HTMLElement>("ul[data-section='']")!);
    try {
      await fireEvent.pointerDown(beat, { button: 0, clientY: 100 });
      await fireEvent.pointerMove(window, { buttons: 1, clientY: 20 });
      await fireEvent.pointerUp(window);
      await waitFor(() => expect(api.SetProjectSection).toHaveBeenCalledWith("t", "beat", ""));
    } finally {
      at.mockRestore();
    }
  });

  it("offers Rename, Move and Delete in a section's menu, and New section… in the team's menu", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.contextMenu(header("Engine"), { clientX: 20, clientY: 80 });
    let menu = await screen.findByRole("menu", { name: "Engine" });
    expect(within(menu).getAllByRole("menuitem").map((m) => m.textContent)).toEqual(["Rename…", "Move down", "Delete section…", "New section…"]);
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Move down" }));
    await waitFor(() => expect(api.MoveSection).toHaveBeenCalledWith("t", "s1", 1));

    await fireEvent.contextMenu(header("Engine"), { clientX: 20, clientY: 80 });
    menu = await screen.findByRole("menu", { name: "Engine" });
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Rename…" }));
    let dialog = await screen.findByRole("dialog", { name: "Rename the section" });
    const field = within(dialog).getByLabelText("Section name") as HTMLInputElement;
    expect(field.value).toBe("Engine");
    await fireEvent.input(field, { target: { value: "Game" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Rename" }));
    await waitFor(() => expect(api.RenameSection).toHaveBeenCalledWith("t", "s1", "Game"));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await fireEvent.click(within(header("Tools")).getByRole("button", { name: "Section menu" }));
    menu = await screen.findByRole("menu", { name: "Tools" });
    expect(within(menu).queryByRole("menuitem", { name: "Move down" })).toBeNull(); // (last)
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Delete section…" }));
    dialog = await screen.findByRole("dialog", { name: "Delete the section “Tools”?" });
    expect(dialog.textContent).toContain("ungrouped");
    await fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.DeleteSection).toHaveBeenCalledWith("t", "s2"));

    await fireEvent.click(screen.getAllByRole("button", { name: "Switch team" })[0]);
    await fireEvent.click(await screen.findByRole("button", { name: "New section…" }));
    dialog = await screen.findByRole("dialog", { name: "New section" });
    await fireEvent.input(within(dialog).getByLabelText("Section name"), { target: { value: " Art " } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Create" }));
    await waitFor(() => expect(api.NewSection).toHaveBeenCalledWith("t", "Art"));
  });

  it("shows a member of an R3V Cloud team the sections, with nothing to change them", async () => {
    api.LocalOverview.mockResolvedValue(overview(false));
    api.Overview.mockResolvedValue(overview(false));
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(header("Engine")).toBeTruthy();
    expect(within(header("Engine")).queryByRole("button", { name: "Section menu" })).toBeNull();
    await fireEvent.contextMenu(header("Engine"), { clientX: 20, clientY: 80 });
    expect(screen.queryByRole("menu", { name: "Engine" })).toBeNull();
    await fireEvent.click(screen.getAllByRole("button", { name: "Switch team" })[0]);
    expect(screen.queryByRole("button", { name: "New section…" })).toBeNull();
    // A drag stays among the ungrouped: nothing changes for the team.
    const at = vi.spyOn(document, "elementFromPoint").mockReturnValue(header("Engine"));
    try {
      const song = aside().querySelector<HTMLElement>("li[data-key='song']")!;
      await fireEvent.pointerDown(song, { button: 0, clientY: 20 });
      await fireEvent.pointerMove(window, { buttons: 1, clientY: 140 });
      await fireEvent.pointerUp(window);
      expect(api.SetProjectSection).not.toHaveBeenCalled();
    } finally {
      at.mockRestore();
    }
  });

  it("has no sections in a build without them", async () => {
    api.LocalOverview.mockResolvedValue({ ...overview(), sectionsOn: false });
    api.Overview.mockResolvedValue({ ...overview(), sectionsOn: false });
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(aside().querySelector(".sect-h")).toBeNull();
    expect(aside().textContent).toContain("Beat");
  });
});

describe("TeamHome's sections", () => {
  const home = (over = {}) => ({
    overview: { teams: [], currentTeam: "t", projects: [], author: "", teamChecked: true, teamError: "", sectionsOn: true,
      sections: [engine, tools] } as never,
    team: { id: "t", name: "Band", address: "s3+https://x/b", memberId: "m1" } as never,
    projects: [project("Song"), project("Beat", "s1", "Engine")] as never, open: () => false, onopen: vi.fn(), onadd: vi.fn(),
    onsettings: vi.fn(), reload: vi.fn(async () => {}), ...over });

  it("groups the cards by section after the ungrouped ones, each folding", async () => {
    render(TeamHome, home());
    const heads = [...document.querySelectorAll(".pane-in h3")].map((h) => h.textContent?.replace(/\s+/g, " ").trim());
    expect(heads).toEqual(["On this computer1", "▾Engine1", "▾Tools0"]);
    const cards = screen.getByRole("generic", { name: "Engine" });
    expect(within(cards).getByTitle("C:/Beat")).toBeTruthy();
    expect(screen.getByText("No projects in this section yet")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: /Engine/ }));
    expect(screen.queryByTitle("C:/Beat")).toBeNull();
    expect(screen.getByTitle("C:/Song")).toBeTruthy();
  });
});

describe("ProjectSettings: the section", () => {
  const props = (over = {}) => ({ p: project("Beat", "s1", "Engine") as never, team: { id: "t", name: "Band" } as never, inline: true,
    onclose: vi.fn(), onrenamed: vi.fn(), oncheck: vi.fn(), ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn(),
    sections: [engine, tools] as never, ...over });

  it("picks the project's section for those who may, or just says it", async () => {
    const onsection = vi.fn(async () => {});
    render(ProjectSettings, props({ onsection }));
    const pick = screen.getByRole("combobox", { name: "Section" }) as HTMLSelectElement;
    expect(pick.value).toBe("s1");
    expect([...pick.options].map((o) => o.textContent)).toEqual(["No section", "Engine", "Tools"]);
    await fireEvent.change(pick, { target: { value: "s2" } });
    await waitFor(() => expect(onsection).toHaveBeenCalledWith("s2"));
    cleanup();
    render(ProjectSettings, props());
    expect(screen.queryByRole("combobox", { name: "Section" })).toBeNull();
    expect(screen.getByText("Engine")).toBeTruthy();
    cleanup();
    render(ProjectSettings, props({ sections: undefined }));
    expect(screen.queryByText("Section")).toBeNull();
  });
});

describe("ProjectHeader: TEAM / Section", () => {
  const st = { root: "C:/Beat", name: "Beat", tool: "", openable: [] } as never;
  it("says the section after the team when the project is in one", () => {
    render(ProjectHeader, { st, team: { id: "t", name: "Band" } as never, entry: { id: "beat", sectionName: "Engine" } as never });
    expect(document.querySelector(".where")!.textContent?.replace(/\s+/g, "")).toBe("Band/Engine");
    cleanup();
    render(ProjectHeader, { st, team: { id: "t", name: "Band" } as never, entry: { id: "beat", sectionName: "" } as never });
    expect(document.querySelector(".where")!.textContent?.trim()).toBe("Band");
  });
});

describe("Activity: sections changed", () => {
  it("says what changed in the team's sections", async () => {
    const item = (change: string, over = {}) => ({ time: new Date().toISOString(), kind: "sections-changed", project: "", projectName: "",
      branch: "", version: "", title: "", by: "m1", byName: "Robin", tool: "", model: "", section: "Engine", change, was: "", ...over });
    const source = { activity: vi.fn(async () => [item("made"), item("renamed", { was: "Core" }), item("project-in", { project: "p1", projectName: "Game" }),
      item("project-out", { project: "p1", projectName: "Game" }), item("moved"), item("deleted")]) };
    render(TeamActivity, { source: source as never });
    expect(await screen.findByText("Robin made the section Engine")).toBeTruthy();
    for (const line of ["Robin renamed the section Core to Engine", "Robin put Game in the section Engine",
      "Robin took Game out of the section Engine", "Robin moved the section Engine", "Robin deleted the section Engine"]) {
      expect(screen.getByText(line)).toBeTruthy();
    }
  });
});
