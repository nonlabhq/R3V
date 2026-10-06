import { describe, expect, it } from "vitest";
import { apply, themes, tokens } from "./themes";

describe("themes", () => {
  const r3v = themes.find((t) => t.id === "r3v")!;
  const classic = themes.find((t) => t.id === "classic")!;

  it("sets only the theme's tokens while the knobs are where the theme has them", () => {
    expect(tokens(r3v, r3v.knobs)).toEqual({});
    expect(tokens(classic, classic.knobs)).toEqual(classic.vars);
  });

  it("adds what a knob moved", () => {
    const v = tokens(r3v, { ...r3v.knobs, radius: 4, accent: "#3366ff", surfaceAlpha: 0.5 });
    expect(v["--radius"]).toBe("4px");
    expect(v["--radius-xl"]).toBe("8px");
    expect(v["--accent"]).toBe("#3366ff");
    expect(v["--surface-menu"]).toContain("50%");
    expect(v["--density"]).toBeUndefined();
  });

  it("takes off what it put on before", () => {
    apply({ "--x-one": "1", "--x-two": "2" });
    apply({ "--x-two": "3" });
    const s = document.documentElement.style;
    expect(s.getPropertyValue("--x-one")).toBe("");
    expect(s.getPropertyValue("--x-two")).toBe("3");
    apply({});
  });
});
