import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/svelte";
import Wordmark from "./Wordmark.svelte";
import app from "../App.svelte?raw";

afterEach(() => cleanup());

// The top-left wordmark: all one colour, the colour of where it sits (the
// accent), not the two-colour picture.
describe("Wordmark", () => {
  it("draws every part in currentColor", () => {
    render(Wordmark, { height: 18 });
    const svg = screen.getByRole("img", { name: "R3V" });
    expect(svg.getAttribute("height")).toBe("18");
    const painted = [...svg.querySelectorAll("path, circle")].filter((e) => !e.closest("mask"));
    expect(painted.length).toBe(8);
    for (const e of painted) {
      const fill = e.getAttribute("fill") ?? e.closest("[fill]")?.getAttribute("fill");
      expect(["currentColor", "none"]).toContain(fill);
    }
  });

  it("is the one at the top left of the side bar", () => {
    expect(app).toContain("<Wordmark />");
    expect(app).not.toContain('class="wordmark" src=');
  });
});
