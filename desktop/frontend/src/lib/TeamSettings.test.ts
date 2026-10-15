import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// The team's settings page (docs/design/team-settings.md): in tabs on
// Nightly (General, Workflow), all on one page as before on Stable; with
// the Go bindings mocked.
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
});
afterEach(() => cleanup());

describe("TeamSettings", () => {
  it("shows General and Workflow as tabs, the team's members in General, and remembers the tab", async () => {
    render(TeamSettings, props({ tabs: true }));
    const nav = screen.getByRole("navigation", { name: "Team settings" });
    expect(nav.textContent).toContain("General");
    expect(await screen.findByText("Mia")).toBeTruthy(); // (a team on its own storage: who has shared)
    await fireEvent.click(screen.getByRole("button", { name: "Workflow" }));
    expect(screen.getByText(/File locking needs a team on R3V Cloud/)).toBeTruthy();
    expect(screen.queryByText("Mia")).toBeNull();
    expect(localStorage.getItem("r3v.teamSettingsTab")).toBe("workflow");
  });

  it("is one page without tabs (Stable), as before", () => {
    render(TeamSettings, props());
    expect(screen.queryByRole("navigation", { name: "Team settings" })).toBeNull();
    expect(screen.getByText("Name")).toBeTruthy();
    expect(screen.queryByText("Members")).toBeNull();
  });
});
