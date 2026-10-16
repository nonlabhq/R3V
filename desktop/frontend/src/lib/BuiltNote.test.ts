import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The note over the commit box for a new folder that looks like a build's
// output, and its "Leave it out" button. The Go bindings are mocks.
const mocks = vi.hoisted(() => ({ AddIgnoreRule: vi.fn(async () => null), toast: vi.fn() }));
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: { AddIgnoreRule: mocks.AddIgnoreRule } }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));

import BuiltNote from "./BuiltNote.svelte";
import type { State } from "./api";

const st = (built: State["built"]) => ({ built }) as unknown as State;
const folder = { folder: "game/target-dx12", kind: "built", like: "", reason: "holds 120 built files", files: 120, bytes: 3 << 20, rule: "target-*/" };

describe("BuiltNote", () => {
  afterEach(() => { cleanup(); vi.clearAllMocks(); });

  it("says nothing without such folders", () => {
    render(BuiltNote, { root: "C:/Game", st: st([]) });
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("names the folder, and leaves it out with its rule", async () => {
    const onrules = vi.fn();
    render(BuiltNote, { root: "C:/Game", st: st([folder]), onrules });
    const note = screen.getByRole("note");
    expect(note.textContent).toContain("game/target-dx12/");
    expect(note.textContent).toContain("120 files");
    const button = screen.getByRole("button", { name: "Leave it out" });
    expect(button.getAttribute("title")).toContain("target-*/");
    await fireEvent.click(button);
    expect(mocks.AddIgnoreRule).toHaveBeenCalledWith("C:/Game", "target-*/");
    await waitFor(() => expect(onrules).toHaveBeenCalled());
  });

  it("says why by its kind", () => {
    render(BuiltNote, { root: "C:/Song", st: st([
      { folder: "Backup-2024", kind: "name", like: "Backup/", reason: "named like Backup/", files: 3, bytes: 2 << 20, rule: "Backup-*/" },
      { folder: "Renders", kind: "size", like: "", reason: "holds 300 MB of new files", files: 4, bytes: 300 << 20, rule: "/Renders/" },
      folder,
    ]) });
    const [named, big, built] = screen.getAllByRole("note").map((n) => n.textContent ?? "");
    expect(named).toContain("“Backup-2024/” is named like “Backup/”, which the rules leave out (3 files,");
    expect(named).not.toContain("build");
    expect(big).toContain("“Renders/” is big (");
    expect(big).toContain("and new (4 files)");
    expect(built).toContain("looks like a build's output");
    for (const text of [named, big, built]) expect(text).toContain("stays in the team's storage for good");
  });

  it("says so when the rule can't be added", async () => {
    mocks.AddIgnoreRule.mockRejectedValueOnce(new Error("the rule would break .r3v.yaml"));
    const onrules = vi.fn();
    render(BuiltNote, { root: "C:/Game", st: st([folder]), onrules });
    await fireEvent.click(screen.getByRole("button", { name: "Leave it out" }));
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith(expect.stringContaining("would break"), "error", 9000));
    expect(onrules).not.toHaveBeenCalled();
  });
});
