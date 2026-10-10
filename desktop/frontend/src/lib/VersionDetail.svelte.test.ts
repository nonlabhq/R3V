import { afterEach, expect, it, vi } from "vitest";
import { flushSync } from "svelte";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The bindings mocked: each resolves to null unless a test says otherwise.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return { api: new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) }) };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import VersionDetail from "./VersionDetail.svelte";

// jsdom has no ResizeObserver (bind:clientWidth uses it).
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;
afterEach(() => cleanup());

const version = (id: string) => ({ id, short: id, author: "Alex", time: "2026-10-06T10:00:00Z", message: "Mix", parents: ["p"],
  branches: [], authorId: "", inBranch: true, notHere: false, branch: "" });
const file = (path: string) => ({ path, status: "modified", size: 1, kind: "other", live: "", from: "", edited: false,
  preview: false, video: false, model: false });
const picked = () => screen.getAllByTitle(/\.txt$/).find((b) => b.classList.contains("on"))?.title;

// The project read again (on focus) brings the same version as a new
// object: its files aren't read again, the list doesn't blink, and the
// picked file stays picked.
it("doesn't read a version's files again when the project is read again", async () => {
  mocks.api.VersionFiles.mockResolvedValue([file("a.txt"), file("b.txt")]);
  // (root as the app passes it: a getter that follows the app's lists)
  let reads = $state(0);
  const props = $state({ v: version("v1"), branch: "main" });
  render(VersionDetail, { props: { get root() { void reads; return "C:/Song"; }, get v() { return props.v; },
    get branch() { return props.branch; } } });
  await fireEvent.click(await screen.findByTitle("b.txt"));
  expect(picked()).toBe("b.txt");
  props.v = version("v1");
  reads++;
  flushSync();
  expect(screen.queryByText("Reading…")).toBeNull();
  expect(picked()).toBe("b.txt");
  expect(mocks.api.VersionFiles).toHaveBeenCalledTimes(1);
  // Another version is read.
  props.v = version("v2");
  flushSync();
  await waitFor(() => expect(mocks.api.VersionFiles).toHaveBeenCalledTimes(2));
});

it("shows a version's files as a list or a tree, as your changes are shown", async () => {
  localStorage.clear();
  mocks.api.VersionFiles.mockResolvedValue([file("Samples/Kick.txt"), file("Samples/Loops/Loop.txt"), file("notes.txt")]);
  render(VersionDetail, { root: "C:/Song", v: version("v2"), branch: "main" });
  await screen.findByTitle("notes.txt");
  // a list at first (few files): each with its folder under its name
  expect(screen.getByText("Samples/Loops/")).toBeTruthy();
  await fireEvent.click(screen.getByRole("button", { name: "Tree" }));
  expect(localStorage.getItem("r3v.changesView:C:/Song")).toBe("tree");
  const samples = screen.getByRole("button", { name: /^Samples/ });
  expect(samples.getAttribute("aria-expanded")).toBe("true");
  expect(screen.queryByText("Samples/Loops/")).toBeNull();
  await fireEvent.click(samples); // closed: its files go
  expect(screen.queryByTitle("Samples/Kick.txt")).toBeNull();
  expect(screen.getByTitle("notes.txt")).toBeTruthy();
  await fireEvent.click(screen.getByRole("button", { name: "List" }));
  expect(screen.getByTitle("Samples/Kick.txt")).toBeTruthy();
});
