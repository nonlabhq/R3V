import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api, toast: vi.fn() };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));

import StepsTimeline from "./StepsTimeline.svelte";

const ROOT = "C:\\Projects\\Arena";
const now = new Date().toISOString();
const step = (n: number, tool: string, files: string[]) => ({ n, turn: 1, version: `v${n}`, on: "h1", time: now, tool, actor: "claude-code", files });
const info = (over: Record<string, unknown> = {}) => ({
  available: true, on: true, settings: {},
  session: { id: "s1", last: now, steps: 2, turns: [{ n: 1, time: now, prompt: "Warmer arena light", steps: [
    step(1, "Write", ["scenes/arena.json"]), step(2, "mcp__blender__execute_code", ["models/lamp.blend", "a.png", "b.png"])] }] },
  sessions: [{ id: "s1", last: now, steps: 2, prompt: "Warmer arena light" }],
  ...over,
});

function show(props: Record<string, unknown> = {}) {
  const p = { root: ROOT, version: "h1", onback: vi.fn(), ...props };
  render(StepsTimeline, p);
  return p;
}

beforeEach(() => { for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });
afterEach(() => cleanup());

describe("The steps pill", () => {
  it("is lit while an agent makes steps, and opens to each step under its prompt", async () => {
    api.Steps.mockResolvedValue(info());
    show();
    const pill = await screen.findByRole("button", { name: /Recording steps · 2/ });
    expect(pill.classList.contains("live")).toBe(true);
    await fireEvent.click(pill);
    expect(screen.getByText(/2 steps · .* · “Warmer arena light”/)).toBeTruthy();
    expect(screen.getByText("“Warmer arena light”")).toBeTruthy();
    expect(screen.getByText("blender · execute_code")).toBeTruthy();
    expect(screen.getByText("models/lamp.blend, a.png and 1 more")).toBeTruthy();
  });

  it("says Steps, not lit, when the last step is a while ago", async () => {
    const old = new Date(Date.now() - 3600_000).toISOString();
    const i = info();
    i.session.last = old;
    api.Steps.mockResolvedValue(i);
    show();
    const pill = await screen.findByRole("button", { name: /^Steps · 2/ });
    expect(pill.classList.contains("live")).toBe(false);
  });

  it("goes back to a step after asking, through the view", async () => {
    api.Steps.mockResolvedValue(info());
    const p = show();
    await fireEvent.click(await screen.findByRole("button", { name: /Recording steps/ }));
    await fireEvent.click(screen.getAllByRole("button", { name: "Go back" })[1]); // step 1 (the latest first)
    expect(screen.getByText("Go back to step 1?")).toBeTruthy();
    await fireEvent.click(screen.getAllByRole("button", { name: "Go back" }).at(-1)!);
    expect(p.onback).toHaveBeenCalledWith("s1", 1);
  });

  it("compares two steps", async () => {
    api.Steps.mockResolvedValue(info());
    api.StepDiff.mockResolvedValue([{ path: "a.png", status: "added" }, { path: "scenes/arena.json", status: "modified" }]);
    show();
    await fireEvent.click(await screen.findByRole("button", { name: /Recording steps/ }));
    await fireEvent.click(screen.getAllByRole("button", { name: "Compare" })[0]);
    await fireEvent.click(screen.getByRole("button", { name: "Compare with step 2" }));
    await waitFor(() => expect(api.StepDiff).toHaveBeenCalledWith(ROOT, "s1", 1, 2));
    expect(await screen.findByText("From step 1 to step 2")).toBeTruthy();
    expect(screen.getByText("changed")).toBeTruthy();
  });

  it("isn't there without steps, or with them off, or in a build without them", async () => {
    for (const i of [info({ session: null }), info({ on: false }), { available: false, on: false, settings: {}, session: null, sessions: [] }]) {
      api.Steps.mockResolvedValue(i);
      show();
      await waitFor(() => expect(api.Steps).toHaveBeenCalled());
      expect(screen.queryByRole("button", { name: /Steps/ })).toBeNull();
      cleanup();
    }
  });
});
