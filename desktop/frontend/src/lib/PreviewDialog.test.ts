import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => [])) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import PreviewDialog from "./PreviewDialog.svelte";
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const version = (id: string, message: string, wip = false) => ({
  id, short: id, author: "Mia", time: "2026-10-16T10:00:00Z", message, parents: ["base"], branches: [], authorId: "", branch: "lane/vfx",
  inBranch: false, notHere: false, actor: null, wip,
});
const suggested = "Merge lane/vfx: 3 versions\n- Add bloom pass\n- Tune shadow cascades";
const preview = {
  action: "merge", base: "base", target: "c", conflicts: [], changes: [], message: suggested,
  versions: [version("c", "Tune shadow cascades"), version("b", "WIP glow", true), version("a", "Add bloom pass")],
};

describe("PreviewDialog", () => {
  it("starts with the merge's suggested description, editable, and marks unfinished versions", async () => {
    const onconfirm = vi.fn();
    render(PreviewDialog, { root: "C:/Song", title: "Merge", preview: preview as never, actionLabel: "Merge and share",
      message: preview.message, onconfirm, onclose: vi.fn() });
    const box = screen.getByLabelText("Description of the merge version") as HTMLTextAreaElement;
    expect(box.value).toBe(suggested);
    await fireEvent.input(box, { target: { value: "Bring in the bloom" } });
    expect(box.value).toBe("Bring in the bloom");
    // Emptied, it can still merge (the plain "Merge branch …" then).
    await fireEvent.input(box, { target: { value: "" } });
    const merge = screen.getByRole("button", { name: "Merge and share" });
    expect(merge.hasAttribute("disabled")).toBe(false);
    await fireEvent.click(merge);
    expect(onconfirm).toHaveBeenCalled();
    expect(screen.getAllByText("unfinished")).toHaveLength(1);
  });
});
