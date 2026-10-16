import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { tick } from "svelte";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import VersionGraph from "./VersionGraph.svelte";

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
  id, short: id, author, time: "2026-10-06T10:00:00Z", message, parents, branches: [], authorId: "", inBranch: true, notHere: false, branch: "", actor: null,
});
// main: m3 - m2 - m1
const versions = [version("m3", "Mix", ["m2"]), version("m2", "Bass", ["m1"]), version("m1", "Sketch", [])];
const main = { name: "main", latest: "m3" };

function show(over: Record<string, unknown> = {}) {
  const onselect = vi.fn();
  render(VersionGraph, { versions, branches: [main], branch: "main", head: "m3", incoming: new Set<string>(),
    pending: 0, selected: "m3", onselect, ...over });
  return { onselect };
}
afterEach(() => cleanup());

// As R3V Cloud's web app uses it: versions and branches only, nothing of
// the App's (no changes, nothing parked, no card buttons).
describe("VersionGraph read only", () => {
  it("shows and picks without the App's parts", async () => {
    const onselect = vi.fn();
    render(VersionGraph, { versions, branches: [main], branch: "main", head: "m3", selected: "m3", onselect });
    expect(screen.getAllByRole("option")).toHaveLength(3);
    expect(screen.queryByRole("option", { name: "Your changes" })).toBeNull();
    await fireEvent.click(screen.getByRole("option", { name: /^Bass,/ }));
    expect(onselect).toHaveBeenLastCalledWith("m2");
  });
});

describe("VersionGraph", () => {
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
    expect(box.querySelectorAll(".node.ms")).toHaveLength(2); // (an orange ring)
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
    await fireEvent.click(screen.getByRole("button", { name: "View your changes" }));
    expect(a.onselect).toHaveBeenLastCalledWith("pending");
    cleanup();
    const b = show({ selected: "m1" });
    expect(screen.queryByRole("button", { name: "View your changes" })).toBeNull();
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

describe("VersionGraph with agents", () => {
  const dots = () => [...document.querySelectorAll("button.node[role='option']")]; // (not the date range's options)
  // main: m3 (merged the renderer) - m2 - m1; the agent's branch: r2 - r1 off m1.
  const agentVersion = (id: string, message: string, parents: string[]) => ({ ...version(id, message, parents), branch: "b-render",
    actor: { tool: "claude-code", model: "opus" } });
  const history = [{ ...version("m3", "Merge renderer", ["m2", "r2"]), branch: "main" }, { ...version("m2", "Bass", ["m1"]), branch: "main" },
    agentVersion("r2", "Fix the renderer\n\nLights were too bright.", ["r1"]), agentVersion("r1", "Renderer", ["m1"]),
    { ...version("m1", "Sketch", []), branch: "main" }];
  const branches = [main, { name: "b-render", latest: "r2", label: "renderer", kind: "agent" }];

  it("hides merged branches until shown from the filter, agents' labels and versions marked", async () => {
    show({ versions: history, branches });
    expect(dots()).toHaveLength(3);
    expect(screen.queryByText("renderer")).toBeNull();
    const filter = screen.getByRole("button", { name: "Graph filters" });
    expect(filter.getAttribute("title")).toBe("Graph filters: 1 branch hidden");
    await fireEvent.click(filter);
    const merged = screen.getByRole("switch", { name: /Merged branches/ });
    await fireEvent.click(merged);
    expect(dots()).toHaveLength(5);
    screen.getByText("renderer");
    screen.getByTitle("An agent's branch");
    // An agent's version: its title only, the tool named, a spark on its dot.
    const dot = screen.getByRole("option", { name: /^Fix the renderer, Robin \(Claude Code\),/ });
    expect(dot.querySelector("[data-agent='claude-code']")).toBeTruthy();
    expect(screen.getByRole("option", { name: /^Bass,/ }).querySelector("[data-agent]")).toBeNull();
    // Agents' branches off: hidden, merged or not.
    await fireEvent.click(screen.getByRole("switch", { name: /Agents' branches/ }));
    expect(dots()).toHaveLength(3);
  });

  it("keeps only the versions in a date range, and dims those not by the authors picked", async () => {
    const old = { ...version("m0", "Old idea", []), branch: "main", time: new Date(Date.now() - 40 * 86400_000).toISOString() };
    const recent = history.map((v) => (v.id === "m1" ? { ...v, parents: ["m0"] } : v));
    show({ versions: [...recent, old], branches, remember: "C:/Song" });
    expect(dots()).toHaveLength(4);
    await fireEvent.click(screen.getByRole("button", { name: "Graph filters" }));
    await fireEvent.change(screen.getByRole("combobox"), { target: { value: "30" } });
    expect(dots()).toHaveLength(3);
    expect(JSON.parse(localStorage.getItem("r3v.graph.filter:C:/Song")!).range).toBe("30"); // kept
    await fireEvent.click(screen.getByRole("button", { name: /Author/ }));
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /Robin/ }));
    expect(document.querySelectorAll(".node.dim")).toHaveLength(0); // (all Robin's)
    await fireEvent.click(screen.getByRole("button", { name: "Show everything" }));
    expect(dots()).toHaveLength(6);
  });

  it("shows an agent's branch with work not merged yet, folding only the merged ones", () => {
    const open = [{ ...version("u1", "Menu", ["m3"]), branch: "b-ui" }, ...history];
    show({ versions: open, branches: [...branches, { name: "b-ui", latest: "u1", label: "ui", kind: "agent" }] });
    screen.getByText("ui");
    expect(screen.queryByText("renderer")).toBeNull();
    expect(screen.getByRole("button", { name: "Graph filters" }).getAttribute("title")).toBe("Graph filters: 1 branch hidden");
  });

  it("never folds the agent's branch you are on", () => {
    show({ versions: history, branches, branch: "b-render", head: "r2", selected: "r2" });
    expect(dots()).toHaveLength(5);
    expect(screen.getByRole("button", { name: "Graph filters" }).getAttribute("title")).toBe("Graph filters");
  });
});

describe("VersionGraph segments", () => {
  const dots = () => [...document.querySelectorAll("button.node[role='option']")];
  const segNodes = () => [...document.querySelectorAll("button.seg")];
  const agent = { tool: "claude-code", model: "opus" };
  // main: m2 - m1 (a person's); lane/render: r10 … r1 off m1, an agent's.
  const lane = Array.from({ length: 10 }, (_, i) => 10 - i).map((n) => ({
    ...version(`r${n}`, `Step ${n}`, [n === 1 ? "m1" : `r${n - 1}`]), branch: "lane/render", actor: agent,
    time: new Date(Date.UTC(2026, 9, 6, 10, n * 12)).toISOString() }));
  const history = [{ ...version("m2", "Mix", ["m1"]), branch: "main", time: "2026-10-06T14:00:00Z" }, ...lane,
    { ...version("m1", "Sketch", []), branch: "main" }];
  const branches = [main, { name: "lane/render", latest: "r10", kind: "agent" }];
  const open = (over: Record<string, unknown> = {}) => show({ versions: history, branches, head: "m2", selected: "m2", ...over });
  beforeEach(() => localStorage.clear());

  it("draws a run of an agent's versions as one node, what matters around it its own", () => {
    open({ milestones: [{ version: "r6", name: "Lights" }] });
    // r10 (the branch's newest), r6 (a milestone) and r1 (where it starts) stay; r9-r7 and r5-r2 are segments.
    expect(dots().map((d) => d.getAttribute("data-id"))).toEqual(["m2", "r10", "r6", "r1", "m1"]);
    const segs = segNodes();
    expect(segs.map((s) => s.getAttribute("data-id"))).toEqual(["seg:r7", "seg:r2"]);
    expect(segs[1].getAttribute("aria-label")).toBe("lane/render · 4 versions · 36m");
    expect(segs[1].querySelector(".stack")!.textContent).toBe("4");
  });

  it("keeps the version picked and the one you are on out of segments", () => {
    open({ branch: "lane/render", head: "r4", selected: "r8" });
    expect(dots().map((d) => d.getAttribute("data-id"))).toEqual(["m2", "r10", "r9", "r8", "r4", "r3", "r2", "r1", "m1"]);
    expect(segNodes().map((s) => s.getAttribute("data-id"))).toEqual(["seg:r5"]);
  });

  it("opens a segment with a click, groups it again from its bracket, and remembers which are open", async () => {
    open({ remember: "C:/Song" });
    const seg = screen.getByRole("button", { name: /8 versions/ });
    expect(seg.tagName).toBe("BUTTON"); // (Tab reaches it, Enter and Space open it)
    expect(seg.getAttribute("aria-expanded")).toBe("false");
    await fireEvent.click(seg);
    expect(segNodes()).toHaveLength(0);
    expect(dots()).toHaveLength(12);
    expect(JSON.parse(localStorage.getItem("r3v.graph.open:C:/Song")!)).toEqual(["r2"]);
    cleanup();
    open({ remember: "C:/Song" });
    await tick();
    expect(dots()).toHaveLength(12); // (still open)
    await fireEvent.click(screen.getByRole("button", { name: "Group 8 versions again" }));
    expect(segNodes()).toHaveLength(1);
    expect(dots()).toHaveLength(4);
    expect(JSON.parse(localStorage.getItem("r3v.graph.open:C:/Song")!)).toEqual([]);
  });

  it("keeps a segment open when a version in it is picked", async () => {
    localStorage.setItem("r3v.graph.open:C:/Song", JSON.stringify(["r2"]));
    open({ remember: "C:/Song", selected: "r5" });
    await tick();
    expect(dots()).toHaveLength(12);
    cleanup();
    localStorage.clear();
    open({ remember: "C:/Song", selected: "r5" });
    expect(segNodes().map((s) => s.getAttribute("data-id"))).toEqual(["seg:r6", "seg:r2"]);
  });

  it("shows every version from the filter", async () => {
    open({ remember: "C:/Song" });
    expect(dots()).toHaveLength(4);
    await fireEvent.click(screen.getByRole("button", { name: "Graph filters" }));
    await fireEvent.click(screen.getByRole("switch", { name: /Every version/ }));
    expect(segNodes()).toHaveLength(0);
    expect(dots()).toHaveLength(12);
    expect(JSON.parse(localStorage.getItem("r3v.graph.filter:C:/Song")!).every).toBe(true);
  });

  it("passes over a segment with ↑ ↓", async () => {
    const { onselect } = open({ selected: "r10" });
    await fireEvent.keyDown(screen.getByRole("listbox"), { key: "ArrowDown" });
    expect(onselect).toHaveBeenLastCalledWith("r1");
  });
});

// Thousands of versions (agents commit often): only the rows in and near
// the view are in the page, wherever it is moved to.
describe("VersionGraph with a long history", () => {
  const N = 2000;
  const long = Array.from({ length: N }, (_, i) => N - 1 - i).map((n) => ({
    ...version(`v${n}`, `Step ${n}`, n ? [`v${n - 1}`] : []), branch: "main", actor: n ? { tool: "claude-code", model: "opus" } : null,
    time: new Date(Date.UTC(2026, 9, 6) + n * 60_000).toISOString() }));
  const branches = [{ name: "main", latest: `v${N - 1}` }];
  const nodes = () => document.querySelectorAll(".node, .seg").length;
  beforeEach(() => localStorage.clear());

  it("draws a bounded number of rows with every version shown, the rows in view as it scrolls", async () => {
    localStorage.setItem("r3v.graph.filter:C:/Song", JSON.stringify({ every: true, merged: true }));
    const started = performance.now();
    show({ versions: long, branches, head: `v${N - 1}`, selected: `v${N - 1}`, remember: "C:/Song" });
    await tick();
    const took = performance.now() - started;
    expect(nodes()).toBeGreaterThan(10);
    expect(nodes()).toBeLessThan(80);
    expect(document.querySelectorAll(".canvas svg path").length).toBeLessThan(80);
    // Far down: other rows, as few (and the one picked, kept for the keyboard).
    await fireEvent.wheel(screen.getByRole("listbox"), { deltaY: 56 * 1000 });
    expect(nodes()).toBeLessThan(80);
    expect(document.querySelector('[data-id="v999"]')).toBeTruthy();
    expect(document.querySelector('[data-id="v1990"]')).toBeNull();
    expect(document.querySelector(`[data-id="v${N - 1}"]`)).toBeTruthy();
    expect(took).toBeLessThan(5000); // (generous, for a slow machine)
  });

  it("draws an agent's long run as one segment", () => {
    show({ versions: long, branches, head: `v${N - 1}`, selected: `v${N - 1}` });
    expect(document.querySelectorAll(".seg")).toHaveLength(1);
    expect(screen.getByRole("button", { name: /1998 versions/ })).toBeTruthy();
    expect(document.querySelectorAll(".node")).toHaveLength(2);
  });
});
