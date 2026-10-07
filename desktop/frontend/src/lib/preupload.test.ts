import { describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// What goes up: the store fed by events, the upload queue showing it, and
// the progress banner that opens it.
const mocks = vi.hoisted(() => {
  const handlers: Record<string, (ev: { data: unknown }) => void> = {};
  let list!: (l: unknown[]) => void;
  const api = { Preuploads: vi.fn(() => new Promise((r) => (list = r))), CancelSave: vi.fn(async () => true) };
  return { handlers, api, resolveList: (l: unknown[]) => list(l) };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: (name: string, fn: (ev: { data: unknown }) => void) => { mocks.handlers[name] = fn; return () => {}; } } }));

import { cancelling, preuploads, transfers, watchPreuploads } from "./preupload.svelte";
import UploadQueue from "./UploadQueue.svelte";
import ProjectBanners from "./ProjectBanners.svelte";

const emit = (name: string, data: unknown) => mocks.handlers[name]({ data });
const pre = (over: Record<string, unknown> = {}) =>
  ({ root: "C:/Song", path: "Video/take.mov", bytes: 10, total: 100, done: false, waiting: [], ...over });

describe("transfers", () => {
  watchPreuploads();

  it("follows background uploads; the start's list doesn't bring back one already done", async () => {
    emit("preupload", pre());
    expect(preuploads["C:/Song"].path).toBe("Video/take.mov");
    emit("preupload", pre({ done: true }));
    expect(preuploads["C:/Song"]).toBeUndefined();
    mocks.resolveList([pre({ bytes: 5 })]); // late: listed before it was done
    await new Promise((r) => setTimeout(r));
    expect(preuploads["C:/Song"]).toBeUndefined();
  });

  it("keeps uploads, not downloads or conversions, until they are done", () => {
    emit("progress", { root: "C:/Song", stage: "uploading", done: 1, total: 3, bytes: 50, totalBytes: 100 });
    expect(transfers["C:/Song"].stage).toBe("uploading");
    emit("progress", { root: "C:/New", stage: "downloading", done: 0, total: 1, bytes: 1, totalBytes: 9 });
    expect(transfers["C:/New"]).toBeUndefined();
    emit("progress", { root: "C:/Song", stage: "done", done: 0, total: 0 });
    expect(transfers["C:/Song"]).toBeUndefined();
  });

  it("shows them in the upload queue", () => {
    render(UploadQueue, { names: {}, onclose: () => {} });
    expect(screen.getByText("Nothing is going up right now.")).toBeTruthy();
    cleanup();
    emit("preupload", pre({ root: "C:\Music\Song", waiting: [{ path: "Video/b.mov", size: 2048 }] }));
    render(UploadQueue, { names: {}, onclose: () => {} });
    expect(screen.getByText("Song", { exact: false })).toBeTruthy(); // the folder's name
    expect(screen.getByText("take.mov")).toBeTruthy();
    expect(screen.getByText("b.mov")).toBeTruthy();
    cleanup();
    emit("preupload", pre({ root: "C:\Music\Song", done: true }));
  });

  it("cancels a commit from the queue until its step ends", async () => {
    emit("progress", { root: "C:/Song", stage: "uploading", done: 0, total: 1, bytes: 5, totalBytes: 10, cancellable: true });
    emit("progress", { root: "C:/Other", stage: "uploading", done: 0, total: 1, bytes: 5, totalBytes: 10 });
    render(UploadQueue, { names: {}, onclose: () => {} });
    const buttons = screen.getAllByRole("button", { name: "Cancel" });
    expect(buttons).toHaveLength(1); // not the one that can't stop any more
    await fireEvent.click(buttons[0]);
    expect(mocks.api.CancelSave).toHaveBeenCalledWith("C:/Song");
    expect(await screen.findByRole("button", { name: "Cancelling…" })).toBeTruthy();
    emit("progress", { root: "C:/Song", stage: "done", done: 0, total: 0 });
    emit("progress", { root: "C:/Other", stage: "done", done: 0, total: 0 });
    expect(cancelling["C:/Song"]).toBeUndefined();
    cleanup();
  });

  it("opens the queue from a step's banner when it can", async () => {
    const onqueue = vi.fn();
    const st = { root: "C:/Song", name: "Song", teamName: "Band", remoteUrl: "", head: "h", incoming: [], takenBack: [],
      changes: [], rules: { applied: [], fromFile: false, error: "", suggestions: [] }, cloudFolder: "", unshared: false,
      olderVersion: null, unfinished: null, inUse: [] };
    const progress = { root: "C:/Song", stage: "uploading", done: 1, total: 2, bytes: 1, totalBytes: 2 };
    const noop = () => {};
    const props = { st, busy: "save", progress, restorable: 0, missingSamples: 0, onshare: noop, onrecover: noop, onpreset: noop,
      onbranchhere: noop, onlatest: noop, oncombine: noop, onnewbranch: noop, onkeep: noop, onupdate: noop, onpreview: noop,
      onrestore: noop, onopenrules: noop };
    render(ProjectBanners, { ...props, onqueue } as never);
    const banner = screen.getByRole("button", { name: /upload queue|Uploading/i });
    await fireEvent.keyDown(banner, { key: "Enter" });
    await fireEvent.click(banner);
    expect(onqueue).toHaveBeenCalledTimes(2);
    cleanup();
    render(ProjectBanners, props as never);
    expect(screen.queryByRole("button", { name: /upload queue/i })).toBeNull();
    cleanup();
  });
});
