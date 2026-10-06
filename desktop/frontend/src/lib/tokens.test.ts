import { describe, expect, it } from "vitest";
import tokens from "../tokens.css?raw";

// Colours live in src/tokens.css; components use them by role. These
// keep it that way: a raw colour in a component, or a token that isn't
// defined (a typo falls back to nothing, silently), fails here.

const sources = import.meta.glob<string>("../**/*.svelte", { query: "?raw", import: "default", eager: true });
const files = Object.entries(sources).map(([path, text]) => {
  const at = text.indexOf("<style");
  // ("./X.svelte" here in lib, "../X.svelte" above it)
  const name = path.startsWith("./") ? "lib/" + path.slice(2) : path.slice(3);
  return { name, text, css: at < 0 ? "" : text.slice(at) };
});

// Their own colours on purpose (see tokens.css).
const ownColours = ["lib/SetView.svelte", "lib/FileIcon.svelte", "lib/ImageCompare.svelte",
  "lib/ModelCompare.svelte", "lib/VideoCompare.svelte", "lib/WeightSummary.svelte"];

describe("design tokens", () => {
  it("finds the components", () => expect(files.length).toBeGreaterThan(40));

  it("components use colours by role", () => {
    const raw = files
      .filter((f) => !ownColours.includes(f.name))
      .flatMap((f) => (f.css.match(/#[0-9a-fA-F]{6}\b/g) ?? []).map((c) => `${f.name}: ${c}`));
    expect(raw).toEqual([]);
  });

  it("every token a component uses is defined", () => {
    const defined = new Set(tokens.match(/--[a-z0-9-]+(?=\s*:)/g));
    // Set by the component itself (style:--grid, --wave: …).
    for (const f of files) for (const m of f.text.matchAll(/(?:style:|[\s;{])(--[a-z0-9-]+)\s*[:=]/g)) defined.add(m[1]);
    const missing = new Set<string>();
    for (const f of files)
      // (not var(--lane-{n}): a name made at run time)
      for (const m of f.text.matchAll(/var\((--[a-z0-9-]+)(?![{$\w-])/g))
        if (!defined.has(m[1])) missing.add(`${f.name}: ${m[1]}`);
    expect([...missing]).toEqual([]);
  });
});
