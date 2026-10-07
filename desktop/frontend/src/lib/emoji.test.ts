import { describe, expect, it } from "vitest";
import { render } from "@testing-library/svelte";
import { emojiName, emojiOf } from "./emoji";
import ProjectIcon from "./ProjectIcon.svelte";

// An emoji kept as a project's icon: a name of a-z, 0-9 and "-".
describe("emoji names", () => {
  it("keeps an emoji as its code points, and gives it back", () => {
    for (const e of ["🎵", "🥁", "👍🏽", "🏳️‍🌈", "👨🏻‍👩🏻‍👧🏻", "🏴󠁧󠁢󠁳󠁣󠁴󠁿"]) {
      const name = emojiName(e);
      expect(name).toMatch(/^e(-[0-9a-f]+)+$/);
      expect(name.length).toBeLessThanOrEqual(64);
      expect(emojiOf(name)).toBe(e);
    }
    expect(emojiName("🎵")).toBe("e-1f3b5");
  });

  it("isn't fooled by other names", () => {
    for (const n of ["", "drum", "e-", "e-zz", "e-110000", "e-1f3b5-", "x-1f3b5", undefined]) expect(emojiOf(n)).toBeNull();
  });
});

describe("a project's emoji", () => {
  it("shows as it is, on no colour", () => {
    const { container } = render(ProjectIcon, { p: { name: "Song", status: "downloaded", icon: "e-1f3b5" } });
    expect(container.textContent).toContain("🎵");
    expect(container.querySelector(".pi")!.classList.contains("emoji")).toBe(true);
  });
});
