import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// Sections (docs/design/sections.md): the left panel groups the projects
// by the team's sections (the ungrouped last), each in this person's order
// (a header dragged) and folding; those who may drag a project onto a
// section (or move it from its menu) and change sections from a header's
// menu or the +; the team's home groups its cards the same way; a
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
import { sectionFolds, sectionOrders, sectionShowAll, setSectionOrder } from "./lib/sections.svelte";
const { api } = mocks;

const project = (name: string, section = "", sectionName = "") =>
  ({ id: name.toLowerCase(), name, root: `C:/${name}`, status: "downloaded", branch: "main", section, sectionName, shared: true });
const engine = { id: "s1", name: "Engine", projects: ["beat"] };
const tools = { id: "s2", name: "Tools", projects: [] as string[] };
const overview = (canEditSections = true, sections = [engine, tools]) => ({
  teams: [{ id: "t", name: "Band", memberName: "Yi", canEditSections }], currentTeam: "t", sectionsOn: true, sections,
  projects: [project("Song"), project("Beat", "s1", "Engine")], teamChecked: true, teamError: "",
});

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  for (const k of Object.keys(sectionFolds)) delete sectionFolds[k];
  for (const k of Object.keys(sectionOrders)) delete sectionOrders[k];
  for (const k of Object.keys(sectionShowAll)) delete sectionShowAll[k];
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
    expect(order).toEqual(["Engine", "Beat", "Tools", "Song"]); // (the ungrouped last)
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

  it("offers Rename and Delete in a section's menu (no Move: it's dragged), and a new section at the Projects band's +", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.contextMenu(header("Engine"), { clientX: 20, clientY: 80 });
    let menu = await screen.findByRole("menu", { name: "Engine" });
    expect(within(menu).getAllByRole("menuitem").map((m) => m.textContent)).toEqual(["Rename…", "Delete section…", "New section…"]);
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
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Delete section…" }));
    dialog = await screen.findByRole("dialog", { name: "Delete the section “Tools”?" });
    expect(dialog.textContent).toContain("ungrouped");
    await fireEvent.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(api.DeleteSection).toHaveBeenCalledWith("t", "s2"));

    // (not in the team's menu: the Projects band's + has it)
    await fireEvent.click(screen.getAllByRole("button", { name: "Switch team" })[0]);
    expect(screen.queryByRole("button", { name: "New section…" })).toBeNull();
    await fireEvent.click(screen.getAllByRole("button", { name: "Switch team" })[0]);
    await fireEvent.click(within(aside()).getByRole("button", { name: "Add a project or a section" }));
    await fireEvent.click(within(await screen.findByRole("menu")).getByRole("menuitem", { name: "Section" }));
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

  it("asks at the Projects band's + whether to add a project or a new section, for those who may", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.click(within(aside()).getByRole("button", { name: "Add a project or a section" }));
    let menu = await screen.findByRole("menu");
    expect(within(menu).getAllByRole("menuitem").map((m) => m.textContent)).toEqual(["Project", "Section"]);
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Project" }));
    await waitFor(() => expect(api.ChooseFolder).toHaveBeenCalled());
    expect(screen.queryByRole("menu")).toBeNull();
    await fireEvent.click(within(aside()).getByRole("button", { name: "Add a project or a section" }));
    menu = await screen.findByRole("menu");
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Section" }));
    expect(await screen.findByRole("dialog", { name: "New section" })).toBeTruthy();
  });

  it("adds a project straight from the + for those who may not change sections, and without sections", async () => {
    for (const o of [overview(false), { ...overview(), sectionsOn: false }]) {
      api.LocalOverview.mockResolvedValue(o);
      api.Overview.mockResolvedValue(o);
      api.ChooseFolder.mockClear();
      render(App);
      await waitFor(() => expect(tabs()).toHaveLength(1));
      expect(within(aside()).queryByRole("button", { name: "Add a project or a section" })).toBeNull();
      await fireEvent.click(within(aside()).getByRole("button", { name: "Add project" }));
      await waitFor(() => expect(api.ChooseFolder).toHaveBeenCalled());
      expect(screen.queryByRole("menu")).toBeNull();
      cleanup();
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

describe("The sections' order: each person's, on this computer", () => {
  const names = () => [...aside().querySelectorAll(".sect-h .sect-name")].map((e) => e.textContent);

  it("moves a section dragged by its header, keeps the order here, and asks the team nothing", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(names()).toEqual(["Engine", "Tools"]);
    await fireEvent.pointerDown(header("Engine"), { button: 0, clientY: 80 });
    await fireEvent.pointerMove(window, { buttons: 1, clientY: 200 });
    await waitFor(() => expect(names()).toEqual(["Tools", "Engine"]));
    await fireEvent.pointerUp(window);
    expect(JSON.parse(localStorage.getItem("r3v.sectionOrder:t")!)).toEqual(["s2", "s1"]);
    // (a drag isn't a click: the section didn't fold)
    expect(within(header("Engine")).getByRole("button", { name: /Engine/ }).getAttribute("aria-expanded")).toBe("true");
    for (const call of Object.keys(mocks.fns).filter((k) => /Section/.test(k))) expect(mocks.fns[call]).not.toHaveBeenCalled();
  });

  it("lets a member who may not change sections order them too", async () => {
    api.LocalOverview.mockResolvedValue(overview(false));
    api.Overview.mockResolvedValue(overview(false));
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.pointerDown(header("Engine"), { button: 0, clientY: 80 });
    await fireEvent.pointerMove(window, { buttons: 1, clientY: 200 });
    await fireEvent.pointerUp(window);
    await waitFor(() => expect(names()).toEqual(["Tools", "Engine"]));
  });

  it("shows the order kept here, not the team record's; a section never placed comes last", async () => {
    const art = { id: "s3", name: "Art", projects: [] as string[] };
    api.LocalOverview.mockResolvedValue(overview(true, [art, engine, tools]));
    api.Overview.mockResolvedValue(overview(true, [art, engine, tools]));
    localStorage.setItem("r3v.sectionOrder:t", JSON.stringify(["s2", "s1"]));
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await waitFor(() => expect(names()).toEqual(["Tools", "Engine", "Art"]));
  });

  it("moves a project from its menu: Move to › None or a section, the one it's in checked; not for those who may not", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.contextMenu(aside().querySelector<HTMLElement>("li[data-key='beat']")!, { clientX: 20, clientY: 40 });
    let menu = await screen.findByRole("menu", { name: "Beat" });
    await fireEvent.click(within(menu).getByRole("menuitem", { name: /Move to/ }));
    const to = await screen.findByRole("menu", { name: "Move to" });
    expect(within(to).getAllByRole("menuitemradio").map((m) => [m.textContent, m.getAttribute("aria-checked")]))
      .toEqual([["None", "false"], ["Engine", "true"], ["Tools", "false"]]);
    await fireEvent.click(within(to).getByRole("menuitemradio", { name: "None" }));
    await waitFor(() => expect(api.SetProjectSection).toHaveBeenCalledWith("t", "beat", ""));
    cleanup();

    api.LocalOverview.mockResolvedValue(overview(false));
    api.Overview.mockResolvedValue(overview(false));
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.contextMenu(aside().querySelector<HTMLElement>("li[data-key='beat']")!, { clientX: 20, clientY: 40 });
    menu = await screen.findByRole("menu", { name: "Beat" });
    expect(within(menu).queryByRole("menuitem", { name: /Move to/ })).toBeNull();
  });
});

describe("TeamHome's sections", () => {
  const home = (over = {}) => ({
    overview: { teams: [], currentTeam: "t", projects: [], author: "", teamChecked: true, teamError: "", sectionsOn: true,
      sections: [engine, tools] } as never,
    team: { id: "t", name: "Band", address: "s3+https://x/b", memberId: "m1" } as never,
    projects: [project("Song"), project("Beat", "s1", "Engine")] as never, open: () => false, onopen: vi.fn(), onadd: vi.fn(),
    onsettings: vi.fn(), reload: vi.fn(async () => {}), ...over });

  it("packs the sections in one grid: side by side when they fit, a big one the whole row and two rows of it, in order", async () => {
    // (a grid 1000px wide: 4 columns of 240px, 12px apart)
    const width = vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1000);
    try {
      const many = Array.from({ length: 9 }, (_, i) => project(`Big${i}`, "s3", "Big"));
      const big = { id: "s3", name: "Big", projects: many.map((p) => p.id) };
      render(TeamHome, home({ overview: { teams: [], currentTeam: "t", projects: [], author: "", teamChecked: true, teamError: "",
        sectionsOn: true, sections: [engine, tools, big] }, projects: [project("Song"), project("Beat", "s1", "Engine"), ...many] }));
      const blocks = () => [...document.querySelectorAll<HTMLElement>(".blocks > .block")];
      await waitFor(() => expect(document.querySelector<HTMLElement>(".blocks")!.style.getPropertyValue("--cols")).toBe("4"));
      expect(blocks().map((b) => [b.dataset.section ?? "", b.style.getPropertyValue("--span")]))
        .toEqual([["s1", "1"], ["s2", "1"], ["s3", "4"], ["", "2"]]); // (in order; the ungrouped last: Song and Add project)
      const bigCards = () => within(screen.getByRole("generic", { name: "Big" })).queryAllByRole("button", { name: /^Big\d/ });
      expect(bigCards()).toHaveLength(8); // (two rows)
      await fireEvent.click(screen.getByRole("button", { name: "Show all (9)" }));
      expect(bigCards()).toHaveLength(9);
      expect(JSON.parse(localStorage.getItem("r3v.sectionsShowAll")!)).toEqual({ s3: true });
      await fireEvent.click(screen.getByRole("button", { name: "Show less" }));
      expect(bigCards()).toHaveLength(8);
    } finally {
      width.mockRestore();
    }
  });

  it("orders the sections as this person did in the left panel", () => {
    setSectionOrder("t", ["s2", "s1"]);
    render(TeamHome, home());
    expect([...document.querySelectorAll(".block h3 .fold")].map((b) => b.textContent?.replace(/[▾▸\d]/g, "").trim())).toEqual(["Tools", "Engine"]);
  });

  it("offers Move to › in a card's ⋯ menu for those who may change sections", async () => {
    const onsection = vi.fn();
    render(TeamHome, home({ onsection }));
    await fireEvent.click(within(screen.getByTitle("C:/Beat").parentElement!).getByRole("button", { name: "More" }));
    await fireEvent.click(await screen.findByRole("menuitem", { name: /Move to/ }));
    await fireEvent.click(within(await screen.findByRole("menu", { name: "Move to" })).getByRole("menuitemradio", { name: "Tools" }));
    expect(onsection).toHaveBeenCalledWith(expect.objectContaining({ id: "beat" }), "s2");
    cleanup();
    render(TeamHome, home());
    await fireEvent.click(within(screen.getByTitle("C:/Beat").parentElement!).getByRole("button", { name: "More" }));
    await screen.findByRole("menuitem", { name: "Settings…" });
    expect(screen.queryByRole("menuitem", { name: /Move to/ })).toBeNull();
  });

  it("groups the cards by section, the ungrouped ones last, each folding", async () => {
    render(TeamHome, home());
    const heads = [...document.querySelectorAll(".pane-in h3")].map((h) => h.textContent?.replace(/\s+/g, " ").trim());
    expect(heads).toEqual(["▾Engine1", "▾Tools0", "On this computer1"]); // (the ungrouped last)
    const cards = screen.getByRole("generic", { name: "Engine" });
    expect(within(cards).getByTitle("C:/Beat")).toBeTruthy();
    expect(screen.getByText("No projects in this section yet")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: /Engine/ }));
    expect(screen.queryByTitle("C:/Beat")).toBeNull();
    expect(screen.getByTitle("C:/Song")).toBeTruthy();
  });

  it("puts a + at the end of the Projects band: a choice with sections to change, else a project", async () => {
    const onadd = vi.fn(), onnewsection = vi.fn();
    render(TeamHome, home({ onadd, onnewsection }));
    const band = document.querySelector<HTMLElement>(".projects h2.band")!;
    expect(band.lastElementChild?.textContent).toBe("+");
    await fireEvent.click(within(band).getByRole("button", { name: "Add a project or a section" }));
    let menu = await screen.findByRole("menu");
    expect(within(menu).getAllByRole("menuitem").map((m) => m.textContent)).toEqual(["Project", "Section"]);
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Section" }));
    expect(onnewsection).toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
    await fireEvent.click(within(band).getByRole("button", { name: "Add a project or a section" }));
    menu = await screen.findByRole("menu");
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Project" }));
    expect(onadd).toHaveBeenCalledTimes(1);
    cleanup();

    const add = vi.fn();
    render(TeamHome, home({ onadd: add }));
    await fireEvent.click(within(document.querySelector<HTMLElement>(".projects h2.band")!).getByRole("button", { name: "Add project" }));
    expect(add).toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
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

describe("The team's home Activity, as it updates", () => {
  const item = (version: string, title: string, at: string) => ({ time: at, kind: "share", project: "p1", projectName: "Game",
    branch: "main", version, title, by: "m1", byName: "Robin", tool: "", model: "" });

  it("keeps the lines there as they are when a new one comes (only it is drawn), and its room from the start", async () => {
    let changed: () => void = () => {};
    let list = [item("v2", "Two", "2026-10-19T10:00:00Z"), item("v1", "One", "2026-10-19T09:00:00Z")];
    const source = { activity: vi.fn(async () => list), activityChanges: (fn: () => void) => { changed = fn; return () => {}; } };
    render(TeamActivity, { source: source as never, latest: 5 });
    const two = await screen.findByText("Robin shared “Two”");
    expect(document.querySelector<HTMLElement>("section.latest")!.style.getPropertyValue("--rows")).toBe("5");
    list = [item("v3", "Three", "2026-10-19T11:00:00Z"), ...list];
    changed();
    await screen.findByText("Robin shared “Three”");
    expect(screen.getByText("Robin shared “Two”")).toBe(two); // (the same line, not drawn again)
  });

  it("isn't read again from nothing when the team's list is read again (a new team object, the same team)", async () => {
    const team = { id: "t", name: "Band", address: "s3+https://x/b", memberId: "m1" };
    api.TeamActivity.mockResolvedValue([]);
    api.TeamMembers.mockResolvedValue([]);
    const props = { overview: { teams: [], currentTeam: "t", projects: [], author: "", teamChecked: true, teamError: "" } as never,
      team: team as never, projects: [] as never, open: () => false, onopen: vi.fn(), onadd: vi.fn(), onsettings: vi.fn(), reload: vi.fn(async () => {}) };
    const { rerender } = render(TeamHome, props);
    await waitFor(() => expect(api.TeamActivity).toHaveBeenCalledTimes(1));
    await rerender({ ...props, team: { ...team } as never });
    await rerender({ ...props, team: { ...team, name: "Band!" } as never });
    await new Promise((r) => setTimeout(r, 10));
    expect(api.TeamActivity).toHaveBeenCalledTimes(1);
    expect(api.TeamMembers).toHaveBeenCalledTimes(1);
  });
});
