import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// A project's settings in its Settings tab: in tabs down the left (General,
// Branches, Rules, Workflow) as a team's are; the archived branches folded
// to their count; each branch's versions. Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: vi.fn() }));

import ProjectSettings from "./ProjectSettings.svelte";
import BranchSettings from "./BranchSettings.svelte";
import type { Branch, TeamSummary } from "./api";

const team = { id: "t1", name: "Band", memberId: "m1", memberName: "Yi", hosted: true } as TeamSummary;
const p = { id: "p1", name: "Song", root: "C:/Song", status: "downloaded", branch: "main" };
const props = () => ({ p: p as never, team, inline: true, onclose: vi.fn(), onrenamed: vi.fn(), oncheck: vi.fn(),
  ondelete: vi.fn(), onunlink: vi.fn(), onlocate: vi.fn() });
const branch = (name: string, versions: number, over: Partial<Branch> = {}) =>
  ({ name, label: "", color: "", parent: "", current: false, latest: null, kind: "", owner: "", versions, ...over }) as Branch;

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  localStorage.clear();
  api.ProjectInfo.mockResolvedValue({ branch: "main", tool: "Unity", rules: { applied: [], error: "" } });
  api.BranchList.mockResolvedValue({ names: true, branches: [branch("main", 12, { current: true }), branch("b-1", 1, { label: "lighting" })] });
  api.ArchivedBranches.mockResolvedValue([
    { name: "b-2", label: "old idea", color: "", by: "Mia", time: new Date().toISOString(), latest: null },
    { name: "b-3", label: "sketch", color: "", by: "", time: new Date().toISOString(), latest: null }]);
  api.ProjectWorkflow.mockResolvedValue({ team: true, agentsHere: false, canEdit: true, locks: true, teamLocksOn: false, teamKinds: [],
    ownLocks: null, all: [], teamAgents: { steps: true, prompts: true, agentLocks: true }, steps: null, prompts: null, agentLocks: null,
    mine: {}, rulesLocks: false });
});
afterEach(() => cleanup());

const nav = () => screen.getByRole("navigation", { name: "Project settings" });

describe("ProjectSettings in tabs", () => {
  it("groups its sections in tabs, and remembers the one picked", async () => {
    render(ProjectSettings, props());
    await waitFor(() => expect([...nav().querySelectorAll("button")].map((b) => b.textContent))
      .toEqual(["General", "Branches", "Rules", "Workflow"]));
    // General: the name, where it is, the checks and the danger zone
    expect(screen.getByLabelText("Project name")).toBeTruthy();
    expect(screen.getByText("Danger zone")).toBeTruthy();
    expect(screen.getByText("Check project…")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Rules…" })).toBeNull();
    await fireEvent.click(within(nav()).getByRole("button", { name: "Rules" }));
    expect(screen.getByRole("button", { name: "Rules…" })).toBeTruthy();
    expect(screen.queryByLabelText("Project name")).toBeNull();
    expect(localStorage.getItem("r3v.projectSettingsTab")).toBe("rules");
    await fireEvent.click(within(nav()).getByRole("button", { name: "Workflow" }));
    expect(await screen.findByRole("combobox", { name: "File locking" })).toBeTruthy();
  });

  it("opens on the tab picked last, once it's there", async () => {
    localStorage.setItem("r3v.projectSettingsTab", "branches");
    render(ProjectSettings, props());
    expect(await screen.findByText("lighting")).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "Branches" }).getAttribute("aria-current")).toBe("page");
  });

  it("has only General for a project not on this computer", () => {
    render(ProjectSettings, { ...props(), p: { ...p, root: "", status: "remote" } as never });
    expect([...nav().querySelectorAll("button")].map((b) => b.textContent)).toEqual(["General"]);
  });

  it("lists the branches with their versions, the archived ones folded to their count", async () => {
    render(ProjectSettings, props());
    await fireEvent.click(await within(nav()).findByRole("button", { name: "Branches" }));
    expect((await screen.findByText("lighting")).closest("button")!.textContent).toContain("1 version");
    expect(screen.getAllByTitle("Branch settings")[0].textContent).toContain("12 versions");
    const fold = await screen.findByRole("button", { name: /Archived \(2\)/ });
    expect(fold.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText("old idea")).toBeNull();
    await fireEvent.click(fold);
    expect(fold.getAttribute("aria-expanded")).toBe("true");
    const list = screen.getByRole("list", { name: "Archived branches" });
    expect(within(list).getByText("sketch")).toBeTruthy();
    await fireEvent.click(within(list).getAllByRole("button", { name: "Unarchive" })[0]);
    await waitFor(() => expect(api.UnarchiveBranch).toHaveBeenCalledWith("C:/Song", "b-2"));
    await fireEvent.click(within(list).getAllByRole("button", { name: "Delete…" })[1]);
    expect(await screen.findByRole("dialog", { name: /Delete “sketch” for good/ })).toBeTruthy();
  });
});

describe("BranchSettings", () => {
  it("says how many versions the branch has", () => {
    render(BranchSettings, { root: "C:/Song", branch: "b-1", branches: [branch("main", 12), branch("b-1", 3, { label: "lighting" })],
      onchanged: vi.fn(), onclose: vi.fn() });
    expect(screen.getByText("3 versions")).toBeTruthy();
  });
});
