import { describe, expect, it } from "vitest";

// src/shared is used by R3V Cloud's web app too (README.md): what is here
// imports only what is here, Svelte and the test tools, never the App
// (../lib, its bindings, Wails).

const sources = import.meta.glob<string>("./**/*.{ts,svelte,css}", { query: "?raw", import: "default", eager: true });
const allowed = /^(\.\/|svelte($|\/)|vitest$|@testing-library\/)/;

describe("shared", () => {
  it("finds its files", () => expect(Object.keys(sources).length).toBeGreaterThan(15));

  it("imports nothing of the App", () => {
    const bad: string[] = [];
    for (const [path, text] of Object.entries(sources))
      for (const m of text.matchAll(/(?:\bfrom\s*|\bimport\s*\(?\s*|@import\s+(?:url\()?)["']([^"']+)["']/g))
        if (!allowed.test(m[1]) || m[1].includes("..")) bad.push(`${path}: ${m[1]}`);
    expect(bad).toEqual([]);
  });
});
