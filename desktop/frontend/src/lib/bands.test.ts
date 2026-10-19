import { describe, expect, it } from "vitest";
import base from "../shared/base.css?raw";

// Flat's header bands: the label right after the dot, whatever the place's
// own heading does (a space-between heading with only a label put it at the
// right); only an end part goes to the right.
const sources = import.meta.glob<string>("../**/*.svelte", { query: "?raw", import: "default", eager: true });

describe("header bands", () => {
  it("start with the label in the Flat look", () => {
    const rule = base.match(/:root\[data-look="flat"\] \.band \{([^}]*)\}/)?.[1] ?? "";
    expect(rule).toMatch(/justify-content:\s*flex-start/);
    expect(base).toMatch(/\.band \.end \{[^}]*margin-left:\s*auto/);
  });

  it("are only the panels' headings (nothing else is class band)", () => {
    const others = Object.entries(sources).flatMap(([path, text]) =>
      [...text.matchAll(/<(\w+)[^>]*class="((?:[^"]*\s)?band(?:\s[^"]*)?)"/g)]
        .filter((m) => !["div", "h2", "h3"].includes(m[1]))
        .map((m) => `${path}: <${m[1]} class="${m[2]}">`));
    expect(others).toEqual([]);
  });
});
