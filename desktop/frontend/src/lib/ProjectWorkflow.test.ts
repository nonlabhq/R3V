import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// A project's Workflow settings (docs/design/team-settings.md): each as the
// team's unless set for the project; read-only for those who can't change
// them; the .r3v.yaml file_locks offered to move. Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: vi.fn() }));

import ProjectWorkflow from "./ProjectWorkflow.svelte";

const view = (over = {}) => ({
  team: true, agentsHere: true, canEdit: true, locks: true, teamLocksOn: false, teamKinds: ["unreal"], ownLocks: null,
  all: [{ id: "unreal", patterns: ["*.umap"] }], teamAgents: { steps: true, prompts: true, agentLocks: true },
  steps: null, prompts: null, agentLocks: null, mine: {}, rulesLocks: false, ...over,
});

beforeEach(() => { for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });
afterEach(() => cleanup());

describe("ProjectWorkflow", () => {
  it("follows the team until the project sets its own", async () => {
    api.ProjectWorkflow.mockResolvedValue(view());
    render(ProjectWorkflow, { root: "C:/Game" });
    const prompts = (await screen.findByRole("combobox", { name: "Keep the prompts" })) as HTMLSelectElement;
    expect(prompts.value).toBe("team");
    expect(prompts.options[0].text).toBe("As the team: on");
    await fireEvent.change(prompts, { target: { value: "off" } });
    expect(api.SetProjectWorkflow).toHaveBeenCalledWith("C:/Game", expect.objectContaining({ prompts: false, steps: null }));
    const locks = screen.getByRole("combobox", { name: "File locking" }) as HTMLSelectElement;
    expect(locks.options[0].text).toBe("As the team: off");
    await fireEvent.change(locks, { target: { value: "on" } });
    expect(api.SetProjectWorkflow).toHaveBeenLastCalledWith("C:/Game", expect.objectContaining({ locks: expect.objectContaining({ on: true }) }));
  });

  it("is read-only for a member who isn't an admin, and offers moving .r3v.yaml's locks to admins only", async () => {
    api.ProjectWorkflow.mockResolvedValue(view({ canEdit: false, rulesLocks: true, steps: false }));
    render(ProjectWorkflow, { root: "C:/Game" });
    const steps = (await screen.findByRole("combobox", { name: "Record agents' steps" })) as HTMLSelectElement;
    expect(steps.disabled).toBe(true);
    expect(steps.value).toBe("off");
    expect(screen.getByText(/still has file_locks/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Move them here" })).toBeNull();
  });

  it("has file locking only, in a build without agents (Stable)", async () => {
    api.ProjectWorkflow.mockResolvedValue(view({ agentsHere: false }));
    render(ProjectWorkflow, { root: "C:/Game" });
    expect(await screen.findByRole("combobox", { name: "File locking" })).toBeTruthy();
    expect(screen.queryByRole("combobox", { name: "Record agents' steps" })).toBeNull();
    expect(screen.queryByText("Don't keep my prompts")).toBeNull();
  });
});
