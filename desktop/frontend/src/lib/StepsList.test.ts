import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: vi.fn() }));

import StepsList from "./StepsList.svelte";

afterEach(() => cleanup());

const step = (n: number, time: string, tool: string, files: string[], detail = "") =>
  ({ n, turn: 1, version: `v${n}`, on: "h", time, tool, detail, files });
const relayed = `<cross-session-message from="uds:\\\\.\\pipe\\LOCAL\\cc-msg-1" from-name="Lane integrator" from-mode="prompting">
Integrator: run r3v merge main, then rebuild.
</cross-session-message>`;

describe("A session's steps", () => {
  it("shows a relayed message as who sent it and its words, the agent's replies and what each call did, in order", async () => {
    mocks.api.StepsTalk.mockResolvedValue([
      { time: "2026-10-12T05:00:00.500Z", kind: "prompt", text: "…" },
      { time: "2026-10-12T05:00:02.000Z", kind: "tool", tool: "Bash", detail: "Merge main into the lane" },
      { time: "2026-10-12T05:00:20.000Z", kind: "reply", text: "Merged main; rebuilding now." },
      { time: "2026-10-12T05:00:25.000Z", kind: "tool", tool: "Bash", detail: "Rebuild the editor" },
    ]);
    render(StepsList, { root: "C:/Game-ws/scene", session: { id: "s1", last: "", steps: 2, turns: [
      { n: 1, time: "2026-10-12T05:00:00Z", prompt: relayed, steps: [
        step(1, "2026-10-12T05:00:10Z", "Bash", ["Cargo.lock"]), step(2, "2026-10-12T05:00:40Z", "Bash", ["target/x"])] }] } });
    // Folded: its time, who sent it and its words on one line; the toggle opens it.
    expect(document.querySelector(".gist")?.textContent).toBe("Lane integrator · Integrator: run r3v merge main, then rebuild.");
    expect(document.querySelector(".step")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(document.querySelector(".whole-prompt")?.textContent).toBe("Integrator: run r3v merge main, then rebuild.");
    await waitFor(() => expect(mocks.api.StepsTalk).toHaveBeenCalledWith("C:/Game-ws/scene", "s1"));
    const reply = await screen.findByText("Merged main; rebuilding now.");
    const rows = [...document.querySelectorAll(".step .tool, .reply .text")].map((e) => e.textContent);
    expect(rows).toEqual(["Bash · Merge main into the lane", "Merged main; rebuilding now.", "Bash · Rebuild the editor"]);
    // Long words are cut to two lines; a click shows them whole.
    const box = reply.closest("button")!;
    expect(box.classList.contains("whole")).toBe(false);
    await fireEvent.click(box);
    expect(box.classList.contains("whole")).toBe(true);
  });

  it("keeps a step's own detail, recorded by the hook, over the conversation's", async () => {
    mocks.api.StepsTalk.mockResolvedValue([]);
    render(StepsList, { root: "C:/p", session: { id: "s2", last: "", steps: 1, turns: [
      { n: 1, time: "2026-10-12T05:00:00Z", prompt: "Go", steps: [step(1, "2026-10-12T05:00:10Z", "Bash", ["a.ts"], "Run the tests")] }] }, openLatest: true });
    expect(await screen.findByText(/Run the tests/)).toBeTruthy();
  });
});
