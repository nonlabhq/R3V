import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// A team's Usage: counted on request on its own storage, the count shown
// by project; R3V Cloud's against the plan. Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import TeamUsage from "./TeamUsage.svelte";
import type { TeamSummary } from "./api";

const team = { id: "t1", name: "Studio" } as TeamSummary;
const MB = 1 << 20;
beforeEach(() => { for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });
afterEach(() => cleanup());

describe("TeamUsage", () => {
  it("counts a team on its own storage on request, by project", async () => {
    api.TeamUsage.mockResolvedValue({ hosted: false, counted: "", used: 0, stored: 0, versions: 0, projects: [], quota: 0, plan: "" });
    api.CountTeamUsage.mockResolvedValue({ hosted: false, counted: new Date().toISOString(), used: 30 * MB, stored: 32 * MB, versions: 12,
      projects: [{ id: "p1", name: "Game", bytes: 25 * MB }, { id: "p2", name: "Trailer", bytes: 10 * MB }], quota: 0, plan: "" });
    render(TeamUsage, { team });
    expect(await screen.findByText("Not counted yet.")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Count now" }));
    expect(await screen.findByText("Game")).toBeTruthy();
    expect(screen.getByText("Trailer")).toBeTruthy();
    expect(api.CountTeamUsage).toHaveBeenCalledWith("t1");
  });

  it("shows an R3V Cloud team's usage against its plan", async () => {
    api.TeamUsage.mockResolvedValue({ hosted: true, counted: new Date().toISOString(), used: 50 * MB, stored: 50 * MB, versions: 0,
      projects: [], quota: 100 * MB, plan: "Team" });
    render(TeamUsage, { team });
    const meter = await screen.findByRole("meter", { name: "Used" });
    expect(meter.getAttribute("aria-valuenow")).toBe(String(50 * MB));
    expect(screen.getByText("Team")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Count now" })).toBeNull();
  });
});
