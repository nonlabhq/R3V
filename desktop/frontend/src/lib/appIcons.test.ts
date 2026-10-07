import { describe, expect, it } from "vitest";
import { appIconFor, appIconNames } from "./appIcons";
import { projectIconNames, projectIcons } from "./projectIcons";

const at = (preset: string, folder = "") => ({ preset, folder, detected: true });

describe("app icons", () => {
  it("have names a team can keep, apart from the drawn icons", () => {
    for (const n of appIconNames) {
      expect(n).toMatch(/^app-[a-z0-9-]+$/);
      expect(projectIconNames).not.toContain(n);
      expect(projectIcons[n]).toContain("<"); // ProjectIcon draws them
    }
  });

  it("are given for the one program a project is made with", () => {
    expect(appIconFor([at("ableton")], ["Song.als"])).toBe("app-ableton");
    expect(appIconFor([at("unity")], ["."])).toBe("app-unity");
    expect(appIconFor([at("unreal")], [])).toBe("app-unreal");
    expect(appIconFor([at("godot")], [])).toBe("app-godot");
    expect(appIconFor([at("design")], ["scene.blend"])).toBe("app-blender");
    expect(appIconFor([at("design")], ["a.blend", "b.blend"])).toBe("app-blender");
    expect(appIconFor([at("ableton"), at("code", "tools")], ["Song.als"])).toBe("app-ableton");
  });

  it("aren't guessed for several programs, or none", () => {
    expect(appIconFor([], [])).toBe("");
    expect(appIconFor(null, null)).toBe("");
    expect(appIconFor([at("code")], [])).toBe("");
    expect(appIconFor([at("unity"), at("ableton", "Audio")], [])).toBe("");
    expect(appIconFor([at("design")], ["scene.blend", "hero.psd"])).toBe("");
    expect(appIconFor([at("design")], ["hero.psd"])).toBe("");
    expect(appIconFor([at("design")], [])).toBe("");
    expect(appIconFor([at("unity"), at("design", "Art")], [".", "Art/ship.blend"])).toBe("");
  });
});
