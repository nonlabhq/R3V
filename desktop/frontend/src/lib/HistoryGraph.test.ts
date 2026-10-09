import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import HistoryGraph from "./HistoryGraph.svelte";

// The Overview's branch graph, rendered in jsdom: what it draws for a
// history, and how picking, the keyboard, the toolbar and dragging behave.

// jsdom has no ResizeObserver (bind:clientWidth) nor PointerEvent.
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;
class FakePointer extends MouseEvent {
  pointerId: number;
  constructor(type: string, init: MouseEventInit & { pointerId?: number } = {}) {
    super(type, { bubbles: true, ...init });
    this.pointerId = init.pointerId ?? 1;
  }
}
// (and no pointer capture)
HTMLElement.prototype.setPointerCapture ??= function () {};
const pointer = (el: Element, type: string, init: MouseEventInit) => fireEvent(el, new FakePointer(type, init));

const version = (id: string, message: string, parents: string[], author = "Robin") => ({
  id, short: id, author, time: "2026-10-06T10:00:00Z", message, parents, branches: [], authorId: "", inBranch: true, notHere: false,
});
// main: m3 - m2 - m1
const versions = [version("m3", "Mix", ["m2"]), version("m2", "Bass", ["m1"]), version("m1", "Sketch", [])];
const main = { name: "main", latest: "m3" };

function show(over: Record<string, unknown> = {}) {
  const onselect = vi.fn();
  render(HistoryGraph, { versions, branches: [main], branch: "main", head: "m3", incoming: new Set<string>(),
    pending: 0, selected: "m3", onselect, ...over });
  return { onselect };
}
afterEach(() => cleanup());

describe("HistoryGraph", () => {
  it("goes back to your changes (else the version you are on) on a click in the background", async () => {
    const { onselect } = show({ pending: 2, selected: "m1" });
    await fireEvent.click(screen.getByRole("listbox"));
    expect(onselect).toHaveBeenLastCalledWith("pending");
    cleanup();
    const second = show({ selected: "m1" });
    await fireEvent.click(screen.getByRole("listbox"));
    expect(second.onselect).toHaveBeenLastCalledWith("m3");
  });

  it("draws a branch just made on main's newest version beside main (no crash on the shared version)", () => {
    show({ branches: [main, { name: "idea", latest: "m3" }], branch: "idea" });
    expect(screen.getByText("main")).toBeTruthy();
    expect(screen.getByText("idea")).toBeTruthy();
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("puts your changes on the branch you are on, even with no versions of its own yet", () => {
    show({ branches: [main, { name: "idea", latest: "m3" }], branch: "idea", pending: 2, selected: "pending" });
    const dot = screen.getByRole("option", { name: "Your changes" });
    const top = screen.getByRole("option", { name: /^Mix,/ });
    expect(dot.style.left).not.toBe(top.style.left); // its own column, not main's
  });

  it("shows your changes before the first version (a project just added), with the branch over them", async () => {
    const { onselect } = show({ versions: [], branches: [], head: "", pending: 5, selected: "pending" });
    expect(screen.getByRole("option", { name: "Your changes" })).toBeTruthy();
    await fireEvent.click(screen.getByText("main"));
    expect(onselect).toHaveBeenLastCalledWith("pending");
    cleanup();
    show({ versions: [], branches: [], head: "", pending: 0, selected: "" });
    expect(screen.getByText(/No versions yet/)).toBeTruthy();
  });

  it("labels a branch with just its name; picking it picks its newest version", async () => {
    const { onselect } = show({ selected: "m1" });
    expect(screen.getByText("main")).toBeTruthy();
    expect(screen.queryByText("Mix")).toBeNull(); // (its newest version's title isn't on the label)
    expect(screen.queryByRole("button", { name: "Branch settings" })).toBeNull(); // (no settings to open)
    await fireEvent.click(screen.getByText("main"));
    expect(onselect).toHaveBeenLastCalledWith("m3");
  });

  it("opens a branch's settings from the right half of its label", async () => {
    const onsettings = vi.fn();
    const { onselect } = show({ branches: [main, { name: "idea", latest: "m3" }], onsettings });
    const buttons = screen.getAllByRole("button", { name: "Branch settings" });
    expect(buttons).toHaveLength(2);
    expect(buttons[0].getAttribute("title")).toBe("Branch settings");
    await fireEvent.click(buttons[1]);
    expect(onsettings).toHaveBeenLastCalledWith("idea");
    expect(onselect).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByText("main"));
    expect(onselect).toHaveBeenLastCalledWith("m3");
    expect(onsettings).toHaveBeenCalledTimes(1);
  });

  it("shows a milestone as a capsule at the left edge, a dashed line and a ring to its version", async () => {
    const { onselect } = show({ milestones: [{ version: "m2", name: "v0.3 Playtest" }, { version: "m1", name: "First mix" }] });
    const capsule = screen.getByRole("button", { name: "v0.3 Playtest" });
    expect(capsule.querySelector("b")?.textContent).toBe("v0.3"); // (the version number stands out)
    expect(screen.getByRole("button", { name: "First mix" }).querySelector("b")).toBeNull();
    const dot = screen.getByRole("option", { name: /^Bass,/ });
    expect(parseFloat(capsule.style.left)).toBeLessThan(parseFloat(dot.style.left));
    expect(capsule.style.top).toBe(dot.style.top); // on its version's row
    const box = screen.getByRole("listbox");
    expect(box.querySelectorAll(".mline")).toHaveLength(2);
    expect(box.querySelectorAll(".mring")).toHaveLength(2);
    expect(dot.getAttribute("aria-label")).toContain("⚑ v0.3 Playtest");
    await fireEvent.click(capsule);
    expect(onselect).toHaveBeenLastCalledWith("m2");
  });

  it("lines the milestones' capsules up on one edge", () => {
    // (a branch off m1: its version is a column over)
    show({ versions: [version("b1", "Idea", ["m1"]), ...versions], branches: [main, { name: "idea", latest: "b1" }],
      milestones: [{ version: "m2", name: "v0.3 Playtest" }, { version: "m1", name: "First mix" }, { version: "b1", name: "Try" }] });
    const lefts = ["v0.3 Playtest", "First mix", "Try"].map((n) => screen.getByRole("button", { name: n }).style.left);
    expect(new Set(lefts).size).toBe(1);
    const dots = ["Bass", "Idea"].map((n) => screen.getByRole("option", { name: new RegExp(`^${n},`) }).style.left);
    expect(dots[0]).not.toBe(dots[1]); // on different columns, and still lined up
  });

  it("keeps a milestone's capsule at the view's left edge as the graph moves, short of its dot", async () => {
    show({ milestones: [{ version: "m2", name: "v0.3 Playtest" }] });
    const capsule = screen.getByRole("button", { name: "v0.3 Playtest" });
    const dot = screen.getByRole("option", { name: /^Bass,/ });
    const box = screen.getByRole("listbox");
    const canvas = box.querySelector<HTMLElement>(".canvas")!;
    const panX = () => parseFloat(/translate\((-?[\d.]+)px/.exec(canvas.style.transform)![1]);
    const drag = async (dx: number) => {
      await pointer(box, "pointerdown", { clientX: 100, clientY: 100, button: 0, buttons: 1 });
      await pointer(box, "pointermove", { clientX: 100 + dx, clientY: 100, buttons: 1 });
      await pointer(box, "pointerup", { clientX: 100 + dx, clientY: 100 });
    };
    // on screen, the capsule's left is its left plus the pan: 24 px in
    const onScreen = () => parseFloat(capsule.style.left) + panX();
    await drag(60);
    expect(onScreen()).toBeCloseTo(24);
    await drag(-30);
    expect(onScreen()).toBeCloseTo(24);
    // dragged far left, it goes along with its dot, still to its left
    await drag(-400);
    expect(parseFloat(capsule.style.left)).toBeLessThan(parseFloat(dot.style.left) - 19 - 12);
    expect(onScreen()).toBeLessThan(24);
  });

  it("glows where the files are now: your changes, else the version you're on", async () => {
    show({ pending: 1 });
    expect(document.querySelector(".node.now")?.getAttribute("data-id")).toBe("pending");
    cleanup();
    show();
    expect(document.querySelectorAll(".node.now")).toHaveLength(1);
    expect(document.querySelector(".node.now")?.classList.contains("here")).toBe(true);
  });

  it("moves through your changes and the versions with ↑ ↓", async () => {
    const { onselect } = show({ pending: 1, selected: "pending" });
    await fireEvent.keyDown(screen.getByRole("listbox"), { key: "ArrowDown" });
    expect(onselect).toHaveBeenLastCalledWith("m3");
  });

  it("offers a way back to your changes, or to the version you are on", async () => {
    const a = show({ pending: 1, selected: "m1" });
    await fireEvent.click(screen.getByRole("button", { name: "View pending changes" }));
    expect(a.onselect).toHaveBeenLastCalledWith("pending");
    cleanup();
    const b = show({ selected: "m1" });
    expect(screen.queryByRole("button", { name: "View pending changes" })).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "View current version" }));
    expect(b.onselect).toHaveBeenLastCalledWith("m3");
    cleanup();
    show();
    expect(screen.queryByRole("button", { name: /^View / })).toBeNull();
  });

  it("zooms from the top right; the crosshair goes back to 100% and to where you are", async () => {
    const { onselect } = show({ selected: "m1" });
    // Zoom spreads the versions out; the dots keep their size (no scaling).
    const gap = () => parseFloat(screen.getByRole("option", { name: /^Bass,/ }).style.top) -
      parseFloat(screen.getByRole("option", { name: /^Mix,/ }).style.top);
    const at100 = gap();
    const canvas = screen.getByRole("listbox").querySelector<HTMLElement>(".canvas")!;
    await fireEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    expect(screen.getByTitle("Back to 100%").textContent).toBe("120%");
    expect(gap()).toBeCloseTo(at100 * 1.2);
    expect(canvas.style.transform).not.toContain("scale");
    await fireEvent.click(screen.getByRole("button", { name: "Back to 100% and to where you are" }));
    expect(screen.getByTitle("Back to 100%").textContent).toBe("100%");
    expect(gap()).toBeCloseTo(at100);
    expect(onselect).not.toHaveBeenCalled(); // (what is picked stays picked)
  });

  it("shows a version's card beside it, over everything (outside the graph, which clips)", async () => {
    show({ actions: undefined });
    await fireEvent.mouseEnter(screen.getByRole("option", { name: /^Bass,/ }));
    const card = screen.getByRole("group", { name: "Bass" });
    expect(card.parentElement).toBe(document.body);
    expect(screen.getByRole("listbox").contains(card)).toBe(false);
    expect(card.querySelector(".arrow")).toBeTruthy();
    cleanup();
    expect(document.body.contains(card)).toBe(false); // (gone with the graph)
  });

  it("doesn't pick a version at the end of a drag, and stops dragging once the button is up", async () => {
    const { onselect } = show();
    const box = screen.getByRole("listbox");
    const dot = screen.getByRole("option", { name: /^Bass,/ });
    const canvas = box.querySelector<HTMLElement>(".canvas")!;
    await pointer(dot, "pointerdown", { clientX: 10, clientY: 10, button: 0, buttons: 1 });
    await pointer(box, "pointermove", { clientX: 60, clientY: 40, buttons: 1 });
    await pointer(box, "pointerup", { clientX: 60, clientY: 40 });
    await fireEvent.click(dot);
    expect(onselect).not.toHaveBeenCalled();
    const after = canvas.style.transform;
    // Pressed, then let go somewhere the release wasn't seen: moving doesn't pan.
    await pointer(box, "pointerdown", { clientX: 10, clientY: 10, button: 0, buttons: 1 });
    await pointer(box, "pointermove", { clientX: 200, clientY: 200, buttons: 0 });
    expect(canvas.style.transform).toBe(after);
  });
});
