import { describe, expect, it } from "vitest";
import { diffOf, hunks, split, textOf } from "./text";
import { TooBig, type FileSource } from "./source";

// Text read and compared in the browser (the web's source has no Go): as
// internal/textdiff does.

const source = (files: Record<string, string | Uint8Array>): FileSource => ({
  open: async (path, version) => {
    const body = files[`${version}:${path}`];
    if (body === undefined) throw new Error("no such file");
    const bytes = typeof body === "string" ? new TextEncoder().encode(body) : body;
    let s = "";
    for (const b of bytes) s += String.fromCharCode(b);
    return { url: `data:application/octet-stream;base64,${btoa(s)}`, size: bytes.length, type: "", close() {} };
  },
});

describe("shared text", () => {
  it("splits lines as textdiff.Split", () => {
    expect(split("")).toEqual([]);
    expect(split("a\r\nb\n")).toEqual(["a", "b"]);
    expect(split("a\n\nb")).toEqual(["a", "", "b"]);
  });

  it("finds the changes in hunks with three lines around them", () => {
    const a = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"];
    const b = ["1", "2", "3", "4", "five", "6", "7", "8", "9", "10", "11"];
    const far = hunks(a, b, 2); // (5 lines apart: more than 2 × 2)
    expect(far).toHaveLength(2);
    expect(far[0].lines.map((l) => `${l.kind}:${l.text}`)).toEqual(["same:3", "same:4", "del:5", "add:five", "same:6", "same:7"]);
    expect(far[1].lines.at(-1)).toEqual({ kind: "add", old: 0, new: 11, text: "11" });
    expect(hunks(a, b, 3)).toHaveLength(1); // (within 2 × 3: one hunk)
  });

  it("reads text, and says when it isn't text or is too big", async () => {
    const src = source({ "v1:a.txt": "one\ntwo\n", "v1:b.bin": new Uint8Array([1, 0, 2]) });
    expect(await textOf(src, "a.txt", "v1")).toEqual({ text: true, tooBig: false, lines: ["one", "two"], truncated: false });
    expect((await textOf(src, "b.bin", "v1")).text).toBe(false);
    const big: FileSource = { open: async () => ({ url: "", size: 9 << 20, type: "", close() {} }) };
    expect((await textOf(big, "x.txt", "v1")).tooBig).toBe(true);
    const refused: FileSource = { open: async () => { throw new TooBig(5 << 20, 4 << 20); } };
    await expect(textOf(refused, "x.txt", "v1")).rejects.toBeInstanceOf(TooBig);
  });

  it("compares two versions, an added file with none", async () => {
    const src = source({ "v1:a.txt": "one\ntwo\n", "v2:a.txt": "one\n2\nthree\n" });
    const d = await diffOf(src, "a.txt", "v1", "v2", false, "");
    expect([d.text, d.added, d.removed]).toEqual([true, 2, 1]);
    const added = await diffOf(src, "a.txt", "none", "v1", false, "");
    expect([added.added, added.removed]).toEqual([2, 0]);
  });
});
