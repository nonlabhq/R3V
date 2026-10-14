import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/svelte";
import ChangeMark from "./ChangeMark.svelte";

afterEach(() => cleanup());

describe("ChangeMark", () => {
  it("says what happened to a file as one letter, the same everywhere: A C D M", () => {
    for (const [status, letter, name] of [["added", "A", "New"], ["modified", "C", "Changed"], ["deleted", "D", "Deleted"], ["renamed", "M", "Moved"]]) {
      render(ChangeMark, { status });
      const mark = screen.getByLabelText(name);
      expect(mark.textContent).toBe(letter);
      expect(mark.classList.contains(status)).toBe(true);
      cleanup();
    }
  });

  it("shows nothing for a file that didn't change", () => {
    const { container } = render(ChangeMark, { status: "unchanged" });
    expect(container.textContent).toBe("");
  });
});
