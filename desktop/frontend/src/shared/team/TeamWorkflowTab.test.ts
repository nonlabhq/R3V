import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import TeamWorkflowTab from "./TeamWorkflowTab.svelte";
import { LOCK_KINDS } from "../lockKinds";
import type { TeamSource, TeamWorkflowData } from "./source";

// A kind R3V locks by default since a team picked its own kinds (3D models)
// is offered to the team's admins: added only by their click, "Not now"
// kept for that team on this computer. Its source is a fake.

const workflow = (over: Partial<TeamWorkflowData> = {}): TeamWorkflowData => ({
  canEdit: true, locks: true, locksOn: true, kinds: ["unreal", "unity", "blender"], all: LOCK_KINDS,
  agents: { steps: true, prompts: true, agentLocks: true }, overrides: [], ...over,
});
function fake(data: TeamWorkflowData) {
  let v = data;
  return {
    workflow: vi.fn(async () => v),
    setLocks: vi.fn(async (on: boolean, kinds: string[]) => { v = { ...v, locksOn: on, kinds }; }),
  };
}
const note = "New: 3D models can lock too";
beforeEach(() => localStorage.clear());
afterEach(() => cleanup());

describe("TeamWorkflowTab: a new default kind", () => {
  it("offers it to an admin of a team that picked its kinds, and adds it on Add only", async () => {
    const source = fake(workflow());
    render(TeamWorkflowTab, { source: source as unknown as TeamSource, remember: "t1" });
    expect(await screen.findByText(note)).toBeTruthy();
    expect(source.setLocks).not.toHaveBeenCalled(); // (never by itself)
    await fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(source.setLocks).toHaveBeenCalledWith(true, ["unreal", "unity", "blender", "models"]);
    await vi.waitFor(() => expect(screen.queryByText(note)).toBeNull());
  });

  it("remembers Not now for that team on this computer", async () => {
    const source = fake(workflow());
    render(TeamWorkflowTab, { source: source as unknown as TeamSource, remember: "t1" });
    await fireEvent.click(await screen.findByRole("button", { name: "Not now" }));
    expect(screen.queryByText(note)).toBeNull();
    expect(source.setLocks).not.toHaveBeenCalled();
    cleanup();
    render(TeamWorkflowTab, { source: fake(workflow()) as unknown as TeamSource, remember: "t1" });
    await screen.findByText("Auto-lock");
    expect(screen.queryByText(note)).toBeNull();
    cleanup();
    render(TeamWorkflowTab, { source: fake(workflow()) as unknown as TeamSource, remember: "t2" }); // another team
    expect(await screen.findByText(note)).toBeTruthy();
  });

  it("isn't offered where it has nothing to add, or to members", async () => {
    for (const over of [{ kinds: LOCK_KINDS.filter((k) => k.default).map((k) => k.id) }, { kinds: [] }, { canEdit: false }, { locksOn: false }]) {
      render(TeamWorkflowTab, { source: fake(workflow(over)) as unknown as TeamSource, remember: "t1" });
      await screen.findByText("Agents");
      expect(screen.queryByText(note), JSON.stringify(over)).toBeNull();
      cleanup();
    }
  });

  it("isn't offered back once an admin takes it off", async () => {
    const source = fake(workflow({ kinds: ["unreal", "models"] }));
    render(TeamWorkflowTab, { source: source as unknown as TeamSource, remember: "t1" });
    const box = (await screen.findByText("3D models")).closest("label")!.querySelector("input")!;
    await fireEvent.click(box);
    await vi.waitFor(() => expect(source.setLocks).toHaveBeenCalledWith(true, ["unreal"]));
    await screen.findByText("Auto-lock");
    expect(screen.queryByText(note)).toBeNull();
  });
});
