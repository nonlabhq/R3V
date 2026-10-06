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
    await fireEvent.click(screen.getByRole("button", { name: "View latest version" }));
    expect(b.onselect).toHaveBeenLastCalledWith("m3");
    cleanup();
    show();
    expect(screen.queryByRole("button", { name: /^View / })).toBeNull();
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
