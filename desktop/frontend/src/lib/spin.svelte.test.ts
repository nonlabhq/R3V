import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushSync } from "svelte";
import { spinner } from "./spin.svelte";

// The refresh icon turns whole turns: at least one, stopping on one.
describe("spinner", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  function run(steps: (busy: { on: boolean }, turning: { on: boolean }) => void) {
    const busy = $state({ on: false });
    const cleanup = $effect.root(() => {
      const turning = spinner(() => busy.on, 800);
      steps(busy, turning);
    });
    return cleanup;
  }

  it("turns one whole turn for a quick refresh, and whole turns for a long one", () => {
    let t!: { on: boolean };
    let b!: { on: boolean };
    const stop = run((busy, turning) => { b = busy; t = turning; });
    b.on = true; flushSync();
    expect(t.on).toBe(true);
    vi.advanceTimersByTime(100); b.on = false; flushSync();
    vi.advanceTimersByTime(600);
    expect(t.on).toBe(true); // still in its first turn
    vi.advanceTimersByTime(150);
    expect(t.on).toBe(false);
    // A long one: busy 1.2 s, stops at 1.6 s (two turns).
    b.on = true; flushSync();
    vi.advanceTimersByTime(1200); b.on = false; flushSync();
    vi.advanceTimersByTime(350);
    expect(t.on).toBe(true);
    vi.advanceTimersByTime(100);
    expect(t.on).toBe(false);
    stop();
    expect(vi.getTimerCount()).toBe(0);
  });
});
