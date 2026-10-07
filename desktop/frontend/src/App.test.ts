import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The window around the projects: tabs and their shortcuts, with the Go
// bindings mocked (every one resolves to null unless set) and the project
// page replaced by a stub.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./lib/api", async (orig) => ({ ...(await orig<typeof import("./lib/api")>()), api: mocks.api }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: () => () => {} },
  Window: { Minimise: async () => {}, ToggleMaximise: async () => {}, Close: async () => {} } }));
vi.mock("./lib/ProjectView.svelte", async () => ({ default: (await import("./test/Stub.svelte")).default }));

import App from "./App.svelte";
const { api } = mocks;

const project = (name: string) => ({ id: name.toLowerCase(), name, root: `C:/${name}`, status: "downloaded", branch: "main" });
const overview = () => ({
  teams: [{ id: "t", name: "Band", memberName: "Yi" }], currentTeam: "t",
  projects: [project("Song"), project("Beat")], teamChecked: true, teamError: "",
});

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  localStorage.clear();
  api.LocalOverview.mockResolvedValue(overview());
  api.Overview.mockResolvedValue(overview());
  api.UpdateStatus.mockResolvedValue({ auto: false, available: null, progress: null });
});
afterEach(() => cleanup());

const tabs = () => screen.queryAllByRole("tab").map((t) => t.textContent?.trim());

describe("App: tabs", () => {
  it("stays with no tab once the last is closed, when the projects are read again", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.click(screen.getByRole("button", { name: "Close Song" }));
    await screen.findByText(/Pick a project on the left/);
    const reads = api.Overview.mock.calls.length;
    await fireEvent.keyDown(window, { key: "F5" }); // reads the overview again
    await waitFor(() => expect(api.Overview.mock.calls.length).toBeGreaterThan(reads));
    await new Promise((r) => setTimeout(r, 20));
    expect(tabs()).toEqual([]);
    expect(screen.getByText(/Pick a project on the left/)).toBeTruthy();
  });

  it("opens a new tab with Ctrl+T, but not held down, nor while typing", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.keyDown(window, { key: "t", ctrlKey: true });
    await waitFor(() => expect(tabs()).toHaveLength(2));
    await fireEvent.keyDown(window, { key: "t", ctrlKey: true, repeat: true });
    const field = document.createElement("input");
    document.body.append(field);
    await fireEvent.keyDown(field, { key: "t", ctrlKey: true });
    field.remove();
    expect(tabs()).toHaveLength(2);
  });

  it("goes through, closes and reopens tabs, and folds the sidebar, with the keyboard", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.click(screen.getAllByRole("button", { name: /^Beat/ })[0]); // a second tab
    await waitFor(() => expect(tabs()).toHaveLength(2));
    const on = () => screen.getAllByRole("tab").find((t) => t.getAttribute("aria-selected") === "true")?.textContent?.trim();
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "Tab", ctrlKey: true });
    expect(on()).toContain("Song"); // (round to the first)
    await fireEvent.keyDown(window, { key: "Tab", ctrlKey: true, shiftKey: true });
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "1", ctrlKey: true });
    expect(on()).toContain("Song");
    await fireEvent.keyDown(window, { key: "9", ctrlKey: true });
    expect(on()).toContain("Beat");
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    await waitFor(() => expect(tabs()).toEqual([expect.stringContaining("Song")]));
    await fireEvent.keyDown(window, { key: "T", ctrlKey: true, shiftKey: true });
    await waitFor(() => expect(tabs()).toHaveLength(2));
    expect(on()).toContain("Beat");
    expect(screen.getByRole("button", { name: "Hide the sidebar" }).title).toBe("Hide the sidebar (Ctrl+\\)");
    const shell = document.querySelector(".shell")!;
    expect(shell.classList.contains("folded")).toBe(false);
    await fireEvent.keyDown(window, { key: "\\", code: "Backslash", ctrlKey: true });
    expect(shell.classList.contains("folded")).toBe(true);
    await fireEvent.keyDown(window, { key: "\\", code: "Backslash", ctrlKey: true });
    expect(shell.classList.contains("folded")).toBe(false);
  });

  it("lists the shortcuts (Ctrl+/), and Esc closes the list", async () => {
    render(App);
    await waitFor(() => expect(tabs()).toHaveLength(1));
    await fireEvent.keyDown(window, { key: "/", code: "Slash", ctrlKey: true });
    expect(await screen.findByRole("dialog", { name: "Keyboard shortcuts" })).toBeTruthy();
    // (tabs don't move under a dialog)
    await fireEvent.keyDown(window, { key: "w", ctrlKey: true });
    expect(tabs()).toHaveLength(1);
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("shows the version short, says Nightly, and copies it in full", async () => {
    api.Version.mockResolvedValue("0.1.3-nightly.202610070525");
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(App);
    const v = await screen.findByTitle(/0\.1\.3-nightly\.202610070525/);
    expect(v.firstChild?.textContent?.trim()).toBe("v0.1.3");
    expect(v.querySelector(".channel")?.textContent).toBe("Nightly");
    await fireEvent.click(v);
    expect(writeText).toHaveBeenCalledWith("0.1.3-nightly.202610070525");
  });
});
