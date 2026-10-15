import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import TeamUsage from "./TeamUsage.svelte";
import type { TeamSource, TeamUsageData } from "./source";

// A team's Usage: counted on request on its own storage, the count shown
// by project; R3V Cloud's against the plan. Its source is a fake.

const MB = 1 << 20;
const usage = (over: Partial<TeamUsageData> = {}): TeamUsageData =>
  ({ hosted: false, counted: "", used: 0, stored: 0, versions: 0, projects: [], quota: 0, plan: "", ...over });
afterEach(() => cleanup());

describe("TeamUsage", () => {
  it("counts a team on its own storage on request, by project", async () => {
    const source = { usage: vi.fn(async () => usage()), countUsage: vi.fn(async () => usage({ counted: new Date().toISOString(),
      used: 30 * MB, stored: 32 * MB, versions: 12,
      projects: [{ id: "p1", name: "Game", bytes: 25 * MB, removed: false }, { id: "p2", name: "Trailer", bytes: 10 * MB, removed: true }] })) };
    render(TeamUsage, { source: source as unknown as TeamSource });
    expect(await screen.findByText("Not counted yet.")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Count now" }));
    expect(await screen.findByText("Game")).toBeTruthy();
    expect(screen.getByText("(removed)")).toBeTruthy();
    expect(source.countUsage).toHaveBeenCalled();
  });

  it("shows an R3V Cloud team's usage against its plan, with nothing to count", async () => {
    const source = { usage: vi.fn(async () => usage({ hosted: true, counted: new Date().toISOString(), used: 50 * MB, stored: 50 * MB, quota: 100 * MB, plan: "Team" })) };
    render(TeamUsage, { source: source as unknown as TeamSource });
    const meter = await screen.findByRole("meter", { name: "Used" });
    expect(meter.getAttribute("aria-valuenow")).toBe(String(50 * MB));
    expect(screen.getByText("Team")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Count now" })).toBeNull();
  });
});
