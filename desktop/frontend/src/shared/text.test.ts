import { afterEach, describe, expect, it, vi } from "vitest";
import { howLongAgo } from "./text";

describe("howLongAgo", () => {
  afterEach(() => vi.useRealTimers());
  const now = Date.parse("2026-10-19T12:00:00Z");
  const before = (seconds: number) => new Date(now - seconds * 1000).toISOString();

  it("says how long ago in words, never a date, however old", () => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    const day = 86400;
    expect(howLongAgo(before(30))).toMatch(/now/);
    expect(howLongAgo(before(5 * 60))).toMatch(/5 min/);
    expect(howLongAgo(before(3 * 3600))).toMatch(/3 hr/);
    expect(howLongAgo(before(3 * day))).toMatch(/3 days/);
    expect(howLongAgo(before(15 * day))).toMatch(/2 wk/);
    expect(howLongAgo(before(90 * day))).toMatch(/3 mo/);
    expect(howLongAgo(before(800 * day))).toMatch(/2 yr/);
    expect(howLongAgo("not a time")).toBe("");
  });
});
