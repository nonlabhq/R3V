import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import Tooltip, { splitTip } from "./Tooltip.svelte";

// jsdom has no ResizeObserver (bind:clientWidth uses it).
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;
afterEach(() => { cleanup(); vi.useRealTimers(); document.body.innerHTML = ""; });
const pointer = (el: Element, type: string, related?: Element) =>
  fireEvent(el, new MouseEvent(type, { bubbles: true, relatedTarget: related ?? null }));

describe("splitTip", () => {
  it("takes a shortcut off the end", () => {
    expect(splitTip("New tab (Ctrl+T)")).toEqual({ text: "New tab", keys: "Ctrl+T" });
    expect(splitTip("Refresh (F5)")).toEqual({ text: "Refresh", keys: "F5" });
    expect(splitTip("Close (Esc)")).toEqual({ text: "Close", keys: "Esc" });
    expect(splitTip("Hide the sidebar (Ctrl+\\)")).toEqual({ text: "Hide the sidebar", keys: "Ctrl+\\" });
    expect(splitTip("Mix (the final one)")).toEqual({ text: "Mix (the final one)", keys: "" });
  });
});

describe("Tooltip", () => {
  it("shows an element's title its own way, after a moment, and goes when the pointer leaves", async () => {
    vi.useFakeTimers();
    render(Tooltip);
    const b = document.createElement("button");
    b.title = "New tab (Ctrl+T)";
    b.textContent = "+";
    const other = document.createElement("div");
    document.body.append(b, other);
    await pointer(b, "pointerover");
    expect(b.hasAttribute("title")).toBe(false); // (no browser tooltip as well)
    expect(b.dataset.tip).toBe("New tab (Ctrl+T)");
    expect(screen.queryByRole("tooltip")).toBeNull();
    await vi.advanceTimersByTimeAsync(500);
    const tip = screen.getByRole("tooltip");
    expect(tip.textContent).toContain("New tab");
    expect(tip.querySelector("kbd")?.textContent).toBe("Ctrl+T");
    await pointer(b, "pointerout", other);
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("keeps an icon button named by its title", async () => {
    render(Tooltip);
    const b = document.createElement("button");
    b.title = "Refresh (F5)";
    document.body.append(b);
    await pointer(b, "pointerover");
    expect(b.getAttribute("aria-label")).toBe("Refresh (F5)");
  });
});
