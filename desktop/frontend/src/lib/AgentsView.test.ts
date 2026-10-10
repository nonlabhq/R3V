import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api, toast: vi.fn() };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));

import AgentsView from "./AgentsView.svelte";

const ROOT = "C:\\Projects\\Song Project";
const branches = [
  { name: "main", label: "", color: "", parent: "", current: true, latest: null, kind: "", owner: "" },
  { name: "b-r1", label: "renderer", color: "", parent: "main", current: false, latest: null, kind: "agent", owner: "" },
  { name: "idea", label: "", color: "", parent: "main", current: false, latest: null, kind: "", owner: "" },
];
const ws = (over: Record<string, unknown> = {}) => ({ folder: "C:\\Projects\\Song (renderer)", branch: "b-r1", label: "renderer",
  base: "main", baseLabel: "main", created: "2026-10-10T08:00:00Z", ahead: 3, behind: 2, changes: 1, parked: 0,
  lastActivity: new Date(Date.now() - 120000).toISOString(), missing: false, locks: [], tool: "claude-code", owner: "Yi",
  stale: false, opened: false, ...over });

function show(props: Record<string, unknown> = {}) {
  const p = { root: ROOT, name: "Song", branch: "main", branches, version: "h1", onmerge: vi.fn(), onopen: vi.fn(), onchanged: vi.fn(), ...props };
  render(AgentsView, p);
  return p;
}
const toasted = (text: RegExp) => waitFor(() => expect(toast.mock.calls.some(([t]) => text.test(t))).toBe(true));

beforeEach(() => { for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; toast.mockClear(); });
afterEach(() => cleanup());

describe("The Agents view", () => {
  it("says what workspaces are for when there are none, with the command for agents", async () => {
    api.AgentWorkspaces.mockResolvedValue([]);
    show();
    expect(await screen.findByRole("heading", { name: "Agents working side by side" })).toBeTruthy();
    expect(screen.getByText(/another copy of this project on this computer/)).toBeTruthy();
    expect(screen.getByText(/r3v workspace add ".*Song \(renderer\)" --branch renderer --agent claude-code/)).toBeTruthy();
    expect(api.AgentWorkspaces).toHaveBeenCalledWith(ROOT);
  });

  it("lists each workspace with what it has to merge, its changes, its locks and when it was last active", async () => {
    api.AgentWorkspaces.mockResolvedValue([ws({ locks: [{ path: "tex/rock.png", prefix: false, memberId: "", name: "", since: "",
      mine: false, agent: true, tool: "claude-code", branch: "b-r1", person: false, stale: false }] })]);
    show();
    const item = (await screen.findByText("renderer")).closest("li")!;
    const it = within(item);
    expect(it.getByText("3 versions to merge")).toBeTruthy();
    expect(it.getByText("2 versions behind “main”")).toBeTruthy();
    expect(it.getByText("1 change not committed")).toBeTruthy();
    expect(it.getByText(/^Active /)).toBeTruthy();
    expect(it.getByText("for Yi")).toBeTruthy();
    expect(it.getByRole("button", { name: "C:\\Projects\\Song (renderer)" })).toBeTruthy();
    expect(it.queryByText("tex/rock.png")).toBeNull();
    await fireEvent.click(it.getByRole("button", { name: /Holds 1 lock/ }));
    expect(it.getByText("tex/rock.png")).toBeTruthy();
  });

  it("says when a folder is gone, and a workspace with nothing to merge can't be merged", async () => {
    api.AgentWorkspaces.mockResolvedValue([ws({ folder: "C:\\gone", ahead: 0, behind: 0, changes: 0 }),
      ws({ folder: "C:\\x", branch: "b-2", label: "shell", missing: true })]);
    show();
    expect((await screen.findByRole("button", { name: "Merge into main" })).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("Nothing to merge")).toBeTruthy();
    expect(screen.getByText(/The folder is gone/)).toBeTruthy();
  });

  it("merges, and opens a workspace in the sidebar", async () => {
    api.AgentWorkspaces.mockResolvedValue([ws()]);
    const p = { workspace: true, root: "C:\\Projects\\Song (renderer)" };
    api.OpenWorkspace.mockResolvedValue(p);
    const props = show();
    await fireEvent.click(await screen.findByRole("button", { name: "Merge into main" }));
    expect(props.onmerge).toHaveBeenCalledWith(expect.objectContaining({ branch: "b-r1", base: "main" }));
    await fireEvent.click(screen.getByRole("button", { name: "Open" }));
    await waitFor(() => expect(props.onopen).toHaveBeenCalledWith(p));
    expect(api.OpenWorkspace).toHaveBeenCalledWith(ROOT, "C:\\Projects\\Song (renderer)");
  });

  it("makes a new workspace beside the project, set up for Claude Code", async () => {
    api.AgentWorkspaces.mockResolvedValue([]);
    api.AddWorkspace.mockResolvedValue({ folder: "C:\\Projects\\Song (shell)", branch: "b-s", label: "shell", agent: "claude-code", setupError: "" });
    const props = show();
    await fireEvent.click(await screen.findByRole("button", { name: /New workspace/ }));
    const dialog = screen.getByRole("dialog", { name: "New workspace" });
    const d = within(dialog);
    expect(d.getByRole("button", { name: "Create" }).hasAttribute("disabled")).toBe(true);
    await fireEvent.input(d.getByLabelText("Branch name"), { target: { value: "shell" } });
    expect((d.getByLabelText("Folder") as HTMLInputElement).value).toBe("C:\\Projects\\Song (shell)");
    expect((d.getByLabelText("Set it up for Claude Code") as HTMLInputElement).checked).toBe(true);
    expect((d.getByLabelText("Start from") as HTMLSelectElement).value).toBe("main");
    expect(d.getAllByRole("option").map((o) => o.textContent)).toEqual(["main (you are here)", "idea"]); // (not agents' branches)
    await fireEvent.click(d.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(api.AddWorkspace).toHaveBeenCalledWith(ROOT, "C:\\Projects\\Song (shell)", "shell", "", "claude-code"));
    await toasted(/The workspace “shell” is ready/);
    expect(props.onchanged).toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("makes one from another branch, in a folder typed in, without an agent set up; says what went wrong", async () => {
    api.AgentWorkspaces.mockResolvedValue([]);
    api.AddWorkspace.mockRejectedValue(new Error("D:\\ws is not empty"));
    show();
    await fireEvent.click(await screen.findByRole("button", { name: /New workspace/ }));
    const d = within(screen.getByRole("dialog", { name: "New workspace" }));
    await fireEvent.input(d.getByLabelText("Folder"), { target: { value: "D:\\ws" } });
    await fireEvent.input(d.getByLabelText("Branch name"), { target: { value: "textures" } });
    expect((d.getByLabelText("Folder") as HTMLInputElement).value).toBe("D:\\ws"); // (typed: kept)
    await fireEvent.change(d.getByLabelText("Start from"), { target: { value: "idea" } });
    await fireEvent.click(d.getByLabelText("Set it up for Claude Code"));
    await fireEvent.click(d.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(api.AddWorkspace).toHaveBeenCalledWith(ROOT, "D:\\ws", "textures", "idea", ""));
    expect((await d.findByRole("alert")).textContent).toMatch(/not empty/);
  });

  it("asks before removing; refused, says what's left, and removes anyway when told", async () => {
    api.AgentWorkspaces.mockResolvedValue([ws()]);
    api.RemoveWorkspace.mockResolvedValueOnce({ removed: false, left: { unmerged: 3, changes: 1, parked: 0, base: "main" } })
      .mockResolvedValueOnce({ removed: true, left: null });
    const props = show();
    await fireEvent.click(await screen.findByRole("button", { name: "Remove…" }));
    const ask = within(screen.getByRole("dialog", { name: "Remove the workspace “renderer”?" }));
    expect(ask.getByText(/is deleted from this computer and its branch is archived/)).toBeTruthy();
    await fireEvent.click(ask.getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.RemoveWorkspace).toHaveBeenCalledWith(ROOT, "C:\\Projects\\Song (renderer)", false));
    const left = within(await screen.findByRole("dialog", { name: "“renderer” has work that isn't merged" }));
    expect(left.getByText("3 versions not merged into “main”")).toBeTruthy();
    expect(left.getByText("1 change not committed")).toBeTruthy();
    expect(left.getByText(/for good/)).toBeTruthy();
    expect(props.onchanged).not.toHaveBeenCalled();
    await fireEvent.click(left.getByRole("button", { name: "Remove anyway" }));
    await waitFor(() => expect(api.RemoveWorkspace).toHaveBeenLastCalledWith(ROOT, "C:\\Projects\\Song (renderer)", true));
    await toasted(/Removed the workspace “renderer”/);
    expect(props.onchanged).toHaveBeenCalled();
  });

  it("keeps a refused workspace when asked", async () => {
    api.AgentWorkspaces.mockResolvedValue([ws()]);
    api.RemoveWorkspace.mockResolvedValue({ removed: false, left: { unmerged: 0, changes: 0, parked: 2, base: "main" } });
    show();
    await fireEvent.click(await screen.findByRole("button", { name: "Remove…" }));
    await fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    const left = within(await screen.findByRole("dialog", { name: /has work that isn't merged/ }));
    expect(left.getByText("2 sets of parked changes")).toBeTruthy();
    await fireEvent.click(left.getByRole("button", { name: "Keep the workspace" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(api.RemoveWorkspace).toHaveBeenCalledTimes(1);
  });
});
