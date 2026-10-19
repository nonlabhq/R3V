import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// Archiving a project for the team (docs/design/archive.md): the project's
// settings offer it (not its menu), the confirmation says
// what happens and what isn't shared here, the panel leaves an archived
// project out, and the team's home lists it under Archived, folded, with
// Unarchive. Go bindings mocked.
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
Element.prototype.animate ??= function () {
  let done: (() => void) | null = null;
  return { cancel() {}, playState: "finished", effect: null,
    get onfinish() { return done; }, set onfinish(f: (() => void) | null) { done = f; if (f) setTimeout(f); } } as unknown as Animation;
};

import App from "./App.svelte";
import TeamHome from "./lib/TeamHome.svelte";
import ProjectSettings from "./lib/ProjectSettings.svelte";
import ArchiveProjectDialog from "./lib/ArchiveProjectDialog.svelte";
const { api } = mocks;

const project = (name: string, archived = false) =>
  ({ id: name.toLowerCase(), name, root: `C:/${name}`, status: "downloaded", branch: "main", archived });
const overview = (archived = false) => ({
  teams: [{ id: "t", name: "Band", memberName: "Yi", canArchive: true }], currentTeam: "t", archive: true,
  projects: [project("Song"), project("Beat", archived)], teamChecked: true, teamError: "",
});
const info = (over = {}) => ({ canArchive: true, archived: false, onTeam: true, changes: 0, unshared: false, ...over });

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  localStorage.clear();
  api.LocalOverview.mockResolvedValue(overview());
  api.Overview.mockResolvedValue(overview());
  api.UpdateStatus.mockResolvedValue({ auto: false, available: null, progress: null });
  api.ArchiveInfo.mockResolvedValue(info());
});
afterEach(() => cleanup());

const tabs = () => screen.queryAllByRole("tab").map((t) => t.textContent?.trim());
const rowOf = (name: string) => screen.queryAllByRole("button", { name: new RegExp(`^${name}`) })[0];

// (from a project's right-click menu, which has no Archive itself)
async function settingsOf(name: string) {
  await fireEvent.contextMenu(rowOf(name), { clientX: 20, clientY: 40 });
  const menu = await screen.findByRole("menu", { name });
  expect(within(menu).queryByRole("menuitem", { name: "Archive project…" })).toBeNull();
  await fireEvent.click(within(menu).getByRole("menuitem", { name: "Settings…" }));
  return screen.findByRole("dialog");
}

describe("Archiving from the left panel", () => {
  it("offers Archive in the project's settings, not its menu, confirms, and leaves the project out of the panel", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(document.querySelector("aside")!.textContent).toContain("Beat");
    const settings = await settingsOf("Beat");
    await fireEvent.click(within(settings).getByRole("button", { name: "Archive…" }));
    const dialog = await screen.findByRole("dialog", { name: "Archive “Beat”?" });
    expect(dialog.textContent).toContain("hidden for everyone in the team");
    expect(dialog.textContent).toContain("Nothing is deleted");
    api.Overview.mockResolvedValue(overview(true));
    await fireEvent.click(await within(dialog).findByRole("button", { name: "Archive" }));
    await waitFor(() => expect(api.ArchiveProject).toHaveBeenCalledWith("t", "beat", false));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(document.querySelector("aside")!.textContent).not.toContain("Beat"));
    expect(document.querySelector("aside")!.textContent).toContain("Song");
  });

  it("has no Archive in a build without it", async () => {
    api.Overview.mockResolvedValue({ ...overview(), archive: false });
    api.LocalOverview.mockResolvedValue({ ...overview(), archive: false });
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(within(await settingsOf("Beat")).queryByRole("button", { name: "Archive…" })).toBeNull();
  });

  it("has no Archive for a member of an R3V Cloud team", async () => {
    const member = { ...overview(), teams: [{ id: "t", name: "Band", memberName: "Yi", hosted: true, canArchive: false }] };
    api.Overview.mockResolvedValue(member);
    api.LocalOverview.mockResolvedValue(member);
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    expect(within(await settingsOf("Beat")).queryByRole("button", { name: "Archive…" })).toBeNull();
  });
});

describe("ArchiveProjectDialog", () => {
  it("says what isn't shared here, and archives all the same once confirmed", async () => {
    api.ArchiveInfo.mockResolvedValue(info({ changes: 3, unshared: true }));
    const onarchived = vi.fn();
    render(ArchiveProjectDialog, { teamId: "t", p: project("Song") as never, onarchived, onclose: vi.fn() });
    expect(await screen.findByText("3 changes here aren't committed yet.")).toBeTruthy();
    expect(screen.getByText("Some versions here aren't shared with the team yet.")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Archive anyway" }));
    await waitFor(() => expect(api.ArchiveProject).toHaveBeenCalledWith("t", "song", true));
    expect(onarchived).toHaveBeenCalled();
  });

  it("can't archive for a member of an R3V Cloud team", async () => {
    api.ArchiveInfo.mockResolvedValue(info({ canArchive: false }));
    render(ArchiveProjectDialog, { teamId: "t", p: project("Song") as never, onarchived: vi.fn(), onclose: vi.fn() });
    expect(await screen.findByText("Only the team's admins can archive projects.")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Archive" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("says a project not shared yet isn't on the team", async () => {
    api.ArchiveInfo.mockResolvedValue(info({ onTeam: false }));
    render(ArchiveProjectDialog, { teamId: "t", p: project("Song") as never, onarchived: vi.fn(), onclose: vi.fn() });
    expect(await screen.findByText("It isn't on the team yet: commit and share it first.")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Archive" }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("TeamHome's Archived section", () => {
  const home = (canArchive: boolean, over = {}) => ({
    overview: { teams: [], currentTeam: "t", projects: [], author: "", teamChecked: true, teamError: "" } as never,
    team: { id: "t", name: "Band", address: "s3+https://x/b", memberId: "m1", canArchive } as never,
    projects: [project("Song"), project("Beat", true)] as never, open: () => false, onopen: vi.fn(), onadd: vi.fn(),
    onsettings: vi.fn(), reload: vi.fn(async () => {}), ...over });

  it("lists archived projects folded, opens one, and brings one back", async () => {
    const reload = vi.fn(async () => {});
    const onopen = vi.fn();
    const beat = project("Beat", true);
    render(TeamHome, home(true, { projects: [project("Song"), beat], onopen, reload }));
    const fold = screen.getByRole("button", { name: /Archived/ });
    expect(fold.textContent).toContain("1");
    expect(fold.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByTitle("C:/Beat")).toBeNull();
    expect(document.querySelector(".sub")!.textContent).toContain("1 project");
    await fireEvent.click(fold);
    await fireEvent.click(screen.getByTitle("C:/Beat"));
    expect(onopen).toHaveBeenCalledWith(beat);
    await fireEvent.click(screen.getByRole("button", { name: "Unarchive" }));
    await waitFor(() => expect(api.UnarchiveProject).toHaveBeenCalledWith("t", "beat"));
    expect(reload).toHaveBeenCalled();
  });

  it("has no Unarchive for a member of an R3V Cloud team", async () => {
    render(TeamHome, home(false));
    await fireEvent.click(screen.getByRole("button", { name: /Archived/ }));
    expect(screen.getByTitle("C:/Beat")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Unarchive" })).toBeNull();
  });

  it("has no Archive project… in a card's ⋯ menu (it's in the project's settings)", async () => {
    render(TeamHome, home(true));
    await fireEvent.click(screen.getAllByRole("button", { name: "More" })[0]);
    expect(await screen.findByRole("menuitem", { name: "Settings…" })).toBeTruthy();
    expect(screen.queryByRole("menuitem", { name: "Archive project…" })).toBeNull();
  });
});

describe("ProjectSettings: archive and unarchive", () => {
  const props = (p: object, over = {}) => ({ p: p as never, team: { id: "t", name: "Band" } as never, inline: true, onclose: vi.fn(),
    onrenamed: vi.fn(), oncheck: vi.fn(), ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn(), ...over });

  it("offers Archive… by the danger zone, or Unarchive when it is", async () => {
    const onarchive = vi.fn(), onunarchive = vi.fn();
    render(ProjectSettings, props(project("Song"), { onarchive, onunarchive }));
    await fireEvent.click(screen.getByRole("button", { name: "Archive…" }));
    expect(onarchive).toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Unarchive" })).toBeNull();
    cleanup();
    render(ProjectSettings, props(project("Song", true), { onarchive: undefined, onunarchive }));
    await fireEvent.click(screen.getByRole("button", { name: "Unarchive" }));
    expect(onunarchive).toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Archive…" })).toBeNull();
  });
});
