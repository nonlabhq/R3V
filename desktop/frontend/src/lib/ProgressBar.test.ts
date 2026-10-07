import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/svelte";
import ProgressBar from "./ProgressBar.svelte";

afterEach(() => { cleanup(); vi.useRealTimers(); });

const p = (bytes: number) => ({ root: "", stage: "uploading", done: 1, total: 4, bytes, totalBytes: 1000, cancellable: false });

describe("ProgressBar", () => {
  it("keeps a gradient moving when the progress doesn't change for a while", async () => {
    vi.useFakeTimers();
    const { rerender } = render(ProgressBar, { p: p(400) });
    const bar = screen.getByTestId("progress-bar");
    expect(bar.classList.contains("still")).toBe(false);
    await vi.advanceTimersByTimeAsync(2100);
    expect(bar.classList.contains("still")).toBe(true);
    await rerender({ p: p(600) }); // it moves again
    expect(bar.classList.contains("still")).toBe(false);
  });

  it("moves on its own while how much is to do isn't known", () => {
    render(ProgressBar, { p: { root: "", stage: "checking", done: 0, total: 0, bytes: 0, totalBytes: 0, cancellable: false } });
    expect(screen.getByTestId("progress-bar").classList.contains("indeterminate")).toBe(true);
  });
});
