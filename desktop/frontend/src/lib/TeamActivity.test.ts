import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// A team's Activity: what happened, filtered by each member and each agent;
// a version's line opens it. Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import TeamActivity from "./TeamActivity.svelte";
import type { TeamSummary } from "./api";

const now = Date.now();
const at = (min: number) => new Date(now - min * 60_000).toISOString();
const item = (over: Record<string, unknown>) => ({ time: at(1), kind: "share", project: "p1", projectName: "Game", branch: "main",
  version: "", title: "", by: "m1", byName: "Robin", tool: "", model: "", ...over });

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  api.TeamActivity.mockResolvedValue([
    item({ time: at(1), version: "v3", title: "Harbor lights" }),
    item({ time: at(2), version: "v2", title: "Fix collisions", tool: "claude-code", branch: "agent/fix" }),
    item({ time: at(3), kind: "member-joined", project: "", projectName: "", by: "m2", byName: "Mia" }),
  ]);
});
afterEach(() => cleanup());

describe("TeamActivity", () => {
  it("lists what happened, filters by a member or an agent, and opens a version", async () => {
    const onopen = vi.fn();
    render(TeamActivity, { team: { id: "t1", name: "Studio" } as TeamSummary, onopen });
    expect(await screen.findByText("Robin shared “Harbor lights”")).toBeTruthy();
    expect(screen.getByText("Claude Code (for Robin) shared “Fix collisions”")).toBeTruthy();
    expect(screen.getByText("Mia joined the team")).toBeTruthy();

    const who = screen.getByRole("combobox", { name: "Who" }) as HTMLSelectElement;
    await fireEvent.change(who, { target: { value: "a:claude-code" } });
    expect(screen.queryByText("Robin shared “Harbor lights”")).toBeNull();
    expect(screen.getByText("Claude Code (for Robin) shared “Fix collisions”")).toBeTruthy();
    await fireEvent.change(who, { target: { value: "m:m1" } });
    expect(screen.getByText("Robin shared “Harbor lights”")).toBeTruthy();
    expect(screen.queryByText("Claude Code (for Robin) shared “Fix collisions”")).toBeNull();

    await fireEvent.click(screen.getByText("Robin shared “Harbor lights”"));
    expect(onopen).toHaveBeenCalledWith("p1", "v3");
  });
});
