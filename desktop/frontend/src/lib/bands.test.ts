import { describe, expect, it } from "vitest";
import base from "../shared/base.css?raw";

// Flat's header bands: the label right after the dot, whatever the place's
// own heading does (a space-between heading with only a label put it at the
// right); only an end part goes to the right.
const sources = import.meta.glob<string>("../**/*.svelte", { query: "?raw", import: "default", eager: true });

// One structure for both looks (AGENTS.md): a look differs only through
// its tokens and the look's stylesheet. No markup or script switches by
// look: no flat-only / cards-only, no reading of data-look or of the look
// in use, outside the look module, tokens.css and base.css (and R3V's
// settings, where the look is picked).
const code = import.meta.glob<string>(["../**/*.{svelte,ts,css}", "!../**/*.test.ts"], { query: "?raw", import: "default", eager: true });
const lookPlaces = ["/look.svelte.ts", "/tokens.css", "/base.css", "/AppSettings.svelte"];
const uncommented = (s: string) => s.replace(/\/\*[\s\S]*?\*\//g, "").replace(/<!--[\s\S]*?-->/g, "").replace(/(^|[^:"'`])\/\/.*$/gm, "$1");

describe("both looks", () => {
  it("have one structure: nothing switches by look outside the look's own places", () => {
    const found = Object.entries(code)
      .filter(([path]) => !lookPlaces.some((p) => path.endsWith(p)))
      .flatMap(([path, text]) => [...uncommented(text).matchAll(/\b(?:flat-only|cards-only)\b|data-look|dataset\.look|\blook\(\)/g)]
        .map((m) => `${path}: ${m[0]}`));
    expect(found).toEqual([]);
  });
});

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
