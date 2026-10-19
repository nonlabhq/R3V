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

  it("shows a project's newest alone, with its name at the left, in the latest list (onePerProject)", async () => {
    const src = { activity: vi.fn(async () => [
      item({ time: at(1), version: "v5", title: "Sky" }),
      item({ time: at(2), version: "v4", title: "Trees" }),
      item({ time: at(3), kind: "member-joined", project: "", projectName: "", by: "m2", byName: "Mia" }),
      item({ time: at(4), project: "p2", projectName: "Album", version: "a2", title: "Mix", branch: "mix" }),
      item({ time: at(5), version: "v3", title: "Rocks" }),
      item({ time: at(6), project: "p3", projectName: "Film", version: "f1", title: "Cut" }),
    ]) } as unknown as TeamSource;
    render(TeamActivity, { source: src, latest: 3, onePerProject: true });
    expect(await screen.findByText("Robin shared “Sky”")).toBeTruthy();
    const rows = [...document.querySelectorAll("li")];
    expect(rows.map((r) => r.querySelector(".proj")?.textContent)).toEqual(["Game", "", "Album"]);
    expect(rows.map((r) => r.querySelector(".line")?.textContent)).toEqual(["Robin shared “Sky”", "Mia joined the team", "Robin shared “Mix”"]);
    // (the name at the left, so not again on the right: the branch and when)
    expect(rows[2].querySelector(".meta")?.textContent).toMatch(/^mix · /);
    expect(rows[0].firstElementChild?.className).toContain("proj");
  });

  it("keeps every line, the project on the right, without onePerProject", async () => {
    render(TeamActivity, { source: source(), latest: 5 });
    expect(await screen.findByText("Robin shared “Harbor lights”")).toBeTruthy();
    expect(document.querySelectorAll("li")).toHaveLength(3);
    expect(document.querySelector(".proj")).toBeNull();
    expect(document.querySelector("li .meta")?.textContent).toMatch(/^Game · main · /);
  });
});
