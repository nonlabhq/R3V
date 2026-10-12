import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

import NoticeSlot, { noticeTiming } from "./NoticeSlot.svelte";

afterEach(() => { cleanup(); vi.useRealTimers(); });
beforeEach(() => { noticeTiming.showAfter = 0; noticeTiming.stay = 0; });

const v = (id: string, author: string, title: string, over: Record<string, unknown> = {}) => ({ id, short: id, author, authorId: author.toLowerCase(),
  time: new Date().toISOString(), message: title, parents: ["p"], branch: "main", branches: [], inBranch: true, notHere: false, actor: null, ...over });
const st = (over: Record<string, unknown> = {}) =>
  ({ root: "C:/Song", name: "Song", teamName: "Band", remoteUrl: "s3://x", head: "h", incoming: [], takenBack: [], olderVersion: null, ...over });

function show(props: Record<string, unknown>) {
  const p = { st: st(), progress: null, busy: "", onupdate: vi.fn(), onpick: vi.fn(), onpreview: vi.fn(), ...props };
  render(NoticeSlot, p as never);
  return p;
}

describe("The notice slot", () => {
  it("is there when empty, its size kept", () => {
    show({});
    const slot = screen.getByRole("status");
    expect(slot.childElementCount).toBe(0);
  });

  it("tells about one new version: preview picks it in the graph, Get updates gets it", async () => {
    const p = show({ st: st({ incoming: [v("a1", "Kai", "Hull UVs")] }) });
    expect(document.querySelector(".capsule .say")?.textContent).toBe("Kai shared a new version · “Hull UVs”");
    await fireEvent.click(screen.getByRole("button", { name: "Preview" }));
    expect(p.onpick).toHaveBeenCalledWith("a1");
    await fireEvent.click(screen.getByRole("button", { name: "Get updates" }));
    expect(p.onupdate).toHaveBeenCalled();
  });

  it("makes several into one digest, its list opening, Preview all changes, Get all", async () => {
    const p = show({ st: st({ incoming: [v("a1", "Kai", "Hull UVs"), v("a2", "Mo", "Fog pass"), v("a3", "Mo", "Harbor lights")] }) });
    expect(screen.getByText("3 updates")).toBeTruthy();
    expect(document.querySelector(".digest .say")?.textContent).toBe("3 updates from Kai and Mo");
    await fireEvent.click(screen.getByRole("button", { name: /3 updates/ }));
    const rows = document.querySelectorAll(".list .row");
    expect(rows.length).toBe(3);
    await fireEvent.click(rows[1].querySelector("button")!);
    expect(p.onpick).toHaveBeenCalledWith("a2");
    await fireEvent.click(screen.getByRole("button", { name: /3 updates/ }));
    await fireEvent.click(screen.getByRole("button", { name: "Preview all changes" }));
    expect(p.onpreview).toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("button", { name: "Get all" }));
    expect(p.onupdate).toHaveBeenCalled();
  });

  it("shows a transfer, the updates that came meanwhile waiting beside it", async () => {
    show({ st: st({ incoming: [v("a1", "Kai", "Hull UVs"), v("a2", "Mo", "Fog")] }), busy: "save",
      progress: { root: "C:/Song", stage: "uploading", done: 1, total: 2, bytes: 50, totalBytes: 100 } });
    const bar = await screen.findByRole("progressbar");
    expect(bar.getAttribute("aria-valuenow")).toBe("50");
    expect(screen.getByText("+2 updates")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Get updates" })).toBeNull();
    expect(screen.getByText(/↑/)).toBeTruthy();
  });

  it("doesn't show a transfer that ends within a second", async () => {
    vi.useFakeTimers();
    noticeTiming.showAfter = 1000;
    const p = { st: st(), progress: { root: "C:/Song", stage: "downloading", done: 0, total: 1, bytes: 1, totalBytes: 2 }, busy: "update",
      onupdate: vi.fn(), onpick: vi.fn() };
    const { rerender } = render(NoticeSlot, p as never);
    await vi.advanceTimersByTimeAsync(500);
    await rerender({ ...p, progress: null, busy: "" } as never);
    await vi.advanceTimersByTimeAsync(2000);
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("says nothing of the team's versions while a download that didn't finish is told about below", () => {
    show({ st: st({ head: "", incoming: [v("a1", "Kai", "Hull UVs")] }) });
    expect(screen.queryByText(/shared a new version/)).toBeNull();
  });

  it("tells about versions taken back", async () => {
    const p = show({ st: st({ takenBack: [v("w1", "Mo", "oops")] }) });
    expect(screen.getByText(/Mo took back “oops”/)).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Get updates" }));
    await waitFor(() => expect(p.onupdate).toHaveBeenCalled());
  });
});
