import { describe, expect, it } from "vitest";
import { extOf, typeOf } from "./fileTypes";

describe("kinds of file", () => {
  it("knows a file's kind by its extension, whatever its case", () => {
    expect(typeOf("Song.als")).toBe("project");
    expect(typeOf("Samples/Kick.WAV")).toBe("audio");
    expect(typeOf("a/b/cover.psd")).toBe("image");
    expect(typeOf("Clips/take.mov")).toBe("video");
    expect(typeOf("Models/hull.fbx")).toBe("model");
    expect(typeOf("Scripts/player.gd")).toBe("code");
    expect(typeOf("settings.yaml")).toBe("data");
    expect(typeOf("notes.txt")).toBe("doc");
    expect(typeOf("Fonts/Inter.ttf")).toBe("font");
    expect(typeOf("old.zip")).toBe("archive");
    expect(typeOf("Bass.adg")).toBe(""); // none of them
    expect(typeOf("README")).toBe("");
  });

  it("reads the extension of the name, not of a folder", () => {
    expect(extOf("v1.2/readme")).toBe("");
    expect(extOf(".gitignore")).toBe("");
    expect(extOf("a.tar.gz")).toBe("gz");
  });
});
