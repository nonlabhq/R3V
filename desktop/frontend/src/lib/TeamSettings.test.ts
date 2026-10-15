import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// The team's settings page (docs/design/team-settings.md): in tabs
// (General, Activity, Usage; Workflow on Nightly, whose settings are its
// own: agents', R3V Cloud's file locks); with the Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import TeamSettings from "./TeamSettings.svelte";
import type { TeamSummary } from "./api";

const team = (over: Partial<TeamSummary> = {}) =>
  ({ id: "t1", name: "Band", memberId: "m1", memberName: "Yi", isStorage: true, ...over }) as TeamSummary;
const props = (over = {}) => ({ team: team(), inline: true, reload: vi.fn(async () => {}), onclose: vi.fn(), ...over });

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  localStorage.clear();
  api.TeamMembers.mockResolvedValue([{ id: "m1", name: "Yi" }, { id: "m2", name: "Mia" }]);
  api.TeamWorkflow.mockResolvedValue({ canEdit: true, locks: false, locksOn: false, kinds: [], all: [],
    agents: { steps: true, prompts: true, agentLocks: true }, overrides: [] });
});
afterEach(() => cleanup());

describe("TeamSettings", () => {
  it("shows General and Workflow as tabs, the team's members in General, and remembers the tab", async () => {
    render(TeamSettings, props({ tabs: true, workflowTab: true }));
    const nav = screen.getByRole("navigation", { name: "Team settings" });
    expect(nav.textContent).toContain("General");
    expect(await screen.findByText("Mia")).toBeTruthy(); // (a team on its own storage: who has shared)
    await fireEvent.click(screen.getByRole("button", { name: "Workflow" }));
    expect(await screen.findByText(/File locking needs a team on R3V Cloud/)).toBeTruthy();
    expect(screen.queryByText("Mia")).toBeNull();
    expect(localStorage.getItem("r3v.teamSettingsTab")).toBe("workflow");
  });

  it("has no Workflow tab without its settings (Stable)", () => {
    render(TeamSettings, props({ tabs: true }));
    const nav = screen.getByRole("navigation", { name: "Team settings" });
    expect([...nav.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["General", "Activity", "Usage"]);
  });
});
