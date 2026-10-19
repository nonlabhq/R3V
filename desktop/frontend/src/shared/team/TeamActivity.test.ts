import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import TeamActivity from "./TeamActivity.svelte";
import type { TeamSource } from "./source";

// A team's Activity: what happened, filtered by each member and each agent;
// a version's line opens it. Its source is a fake.

const now = Date.now();
const at = (min: number) => new Date(now - min * 60_000).toISOString();
const item = (over: Record<string, unknown>) => ({ time: at(1), kind: "share", project: "p1", projectName: "Game", branch: "main",
  version: "", title: "", by: "m1", byName: "Robin", tool: "", model: "", ...over });
const source = () => ({
  activity: vi.fn(async () => [
    item({ time: at(1), version: "v3", title: "Harbor lights" }),
    item({ time: at(2), version: "v2", title: "Fix collisions", tool: "claude-code", branch: "agent/fix" }),
    item({ time: at(3), kind: "member-joined", project: "", projectName: "", by: "m2", byName: "Mia" }),
  ]),
}) as unknown as TeamSource;
afterEach(() => cleanup());

describe("TeamActivity", () => {
  it("lists what happened, filters by a member or an agent, and opens a version", async () => {
    const onopen = vi.fn();
    render(TeamActivity, { source: source(), onopen });
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

  it("reads again when the source says something happened, keeping the list meanwhile", async () => {
    let changed = () => {};
    const stop = vi.fn();
    const src = source() as TeamSource & { activity: ReturnType<typeof vi.fn> };
    src.activityChanges = (fn) => { changed = fn; return stop; };
    const { unmount } = render(TeamActivity, { source: src });
    expect(await screen.findByText("Robin shared “Harbor lights”")).toBeTruthy();
    src.activity.mockImplementation(async () => [item({ time: at(0), version: "v4", title: "Night sky" })]);
    changed();
    expect(screen.getByText("Robin shared “Harbor lights”")).toBeTruthy();
    expect(await screen.findByText("Robin shared “Night sky”")).toBeTruthy();
    expect(src.activity).toHaveBeenCalledTimes(2);
    unmount();
    expect(stop).toHaveBeenCalled();
  });
});
