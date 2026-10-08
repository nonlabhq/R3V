import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

const api = vi.hoisted(() => ({ SetOverview: vi.fn() }));
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api }));

import MergeDecisions from "./MergeDecisions.svelte";
import type { Conflict, Version } from "./api";

afterEach(() => cleanup());
beforeEach(() => {
  api.SetOverview.mockReset();
  api.SetOverview.mockImplementation(async (_root: string, _file: string, version: string) => ({
    now: {
      tempo: 120, timeSig: [4, 4], length: 16, scenes: [], locators: [], creator: "",
      tracks: [
        { id: "3", name: "Drums", kind: "audio", clips: [{ slot: -1, start: 0, end: 8, name: "" }] },
        { id: "14", name: "Bass", kind: "audio", clips: [{ slot: -1, start: 0, end: version === "t1" ? 16 : 4, name: "" }] },
        { id: "20", name: "Pads", kind: "midi", clips: [] },
      ],
    },
    before: null,
  }));
});

const version = (id: string, author: string, authorId: string, message: string): Version => ({
  id, short: id, author, authorId, message, time: new Date().toISOString(), parents: [], branches: [], inBranch: false, notHere: false,
});
const conflict = (over: Partial<Conflict>): Conflict => ({
  key: "k", file: "Song.als", unit: "Song.als", description: "changed on both sides", canKeepBoth: true,
  kind: "file", track: "", name: "Song.als", ours: "changed", theirs: "changed", ...over,
});
const bass = conflict({ key: "Song.als#track:14", unit: `AudioTrack "Bass"`, kind: "track", track: "14", name: "Bass",
  description: "modified on both sides" });
const kick = conflict({ key: "file:Samples/kick.wav", file: "Samples/kick.wav", unit: "Samples/kick.wav", name: "kick.wav",
  description: "changed by you, deleted by others", theirs: "deleted", canKeepBoth: false });

const props = (over: Record<string, unknown> = {}) => ({
  conflicts: [bass, kick], root: "C:/P", project: "Lighthouse", me: "Yi",
  ours: version("o1", "Yi", "u-yi", "Choir layer"), theirs: version("t1", "Mo", "u-mo", "Bass EQ"),
  combined: [{ file: "Song.als", name: "Choir", what: "added" }, { file: "Samples/snare.wav", name: "", what: "changed" }],
  kind: "merge" as const, ourBranch: "darker-chorus", theirBranch: "main",
  branches: [{ name: "main" }, { name: "darker-chorus" }],
  onresolve: vi.fn(), onclose: vi.fn(), ...over,
});
const next = () => screen.getByRole("button", { name: /Next decision|Merge|Combine|Undo/ }) as HTMLButtonElement;

describe("MergeDecisions", () => {
  it("asks one thing at a time, naming who else changed it", async () => {
    const p = props();
    render(MergeDecisions, p);
    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-label")).toBe("You and Mo both changed the Bass track");
    screen.getByText("1 of 2 decisions");
    screen.getByText(/Everything else was combined on its own: Choir added, snare.wav changed\./);
    screen.getByText(/Not sure\? Ask Mo before you decide\./);
    expect(screen.queryByText(/conflict|rebase/i)).toBeNull();

    // Each side: who, where, and the track as it is there.
    const yours = screen.getByRole("radio", { name: /Keep yours/ });
    const theirs = screen.getByRole("radio", { name: /Keep Mo's/ });
    within(yours).getByText(/on darker-chorus · Choir layer/);
    within(theirs).getByText(/on main · Bass EQ/);
    await waitFor(() => expect(api.SetOverview).toHaveBeenCalledWith("C:/P", "Song.als", "o1", "", "none"));
    expect(api.SetOverview).toHaveBeenCalledWith("C:/P", "Song.als", "t1", "", "none");
    await within(yours).findByText("Bass");
    await within(theirs).findByText("Bass");
    screen.getByText(/adds Mo's as another track, “Bass \[theirs\]”/);

    expect(next().disabled).toBe(true);
    await fireEvent.click(screen.getByRole("radio", { name: /Keep both/ }));
    expect(screen.getByRole("radio", { name: /Keep both/ }).getAttribute("aria-checked")).toBe("true");
    await fireEvent.click(next());

    // The file Mo deleted: no "keep both" (it would be the same as yours).
    screen.getByText("2 of 2 decisions");
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("You and Mo both changed kick.wav");
    expect(screen.queryByRole("radio", { name: /Keep both/ })).toBeNull();
    within(screen.getByRole("radio", { name: /Keep Mo's/ })).getByText("Deleted by Mo");
    within(screen.getByRole("radio", { name: /Keep yours/ })).getByText("Changed by you");
    await fireEvent.click(screen.getByRole("radio", { name: /Keep Mo's/ }));
    expect(next().textContent).toBe("Merge");
    await fireEvent.click(next());
    expect(p.onresolve).toHaveBeenCalledWith({ "Song.als#track:14": "both", "file:Samples/kick.wav": "theirs" });
  });

  it("goes back with the choice kept, and on with Enter", async () => {
    const p = props();
    render(MergeDecisions, p);
    await fireEvent.keyDown(window, { key: "Enter" });
    screen.getByText("1 of 2 decisions"); // nothing picked: stays
    await fireEvent.click(screen.getByRole("radio", { name: /Keep yours/ }));
    await fireEvent.keyDown(window, { key: "Enter" });
    screen.getByText("2 of 2 decisions");
    await fireEvent.click(screen.getByRole("button", { name: /Back/ }));
    screen.getByText("1 of 2 decisions");
    expect(screen.getByRole("radio", { name: /Keep yours/ }).getAttribute("aria-checked")).toBe("true");
  });

  it("decides the rest the same way", async () => {
    const p = props({ conflicts: [bass, kick, conflict({ key: "file:a.txt", file: "a.txt", unit: "a.txt", name: "a.txt" })] });
    render(MergeDecisions, p);
    await fireEvent.click(screen.getByRole("radio", { name: /Keep both/ }));
    await fireEvent.click(screen.getByRole("button", { name: "Same for the rest" }));
    screen.getByText("3 of 3 decisions");
    await fireEvent.click(next());
    // (kick can't keep both: yours)
    expect(p.onresolve).toHaveBeenCalledWith({ "Song.als#track:14": "both", "file:Samples/kick.wav": "ours", "file:a.txt": "both" });
  });

  it("Esc cancels, asking first once something was decided", async () => {
    const p = props();
    const { unmount } = render(MergeDecisions, p);
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(p.onclose).toHaveBeenCalledTimes(1);
    unmount();

    const q = props();
    render(MergeDecisions, q);
    await fireEvent.click(screen.getByRole("radio", { name: /Keep yours/ }));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(q.onclose).not.toHaveBeenCalled();
    screen.getByText("The decisions you made are forgotten and nothing changes.");
    await fireEvent.click(screen.getByRole("button", { name: "Keep deciding" }));
    expect(screen.queryByText("The decisions you made are forgotten and nothing changes.")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Cancel merge" }));
    await fireEvent.click(within(screen.getByRole("dialog", { name: "Cancel the merge?" })).getByRole("button", { name: "Cancel merge" }));
    expect(q.onclose).toHaveBeenCalledTimes(1);
  });

  it("your own versions from another computer: this computer's or the team's", async () => {
    const p = props({ kind: "update" as const, ourBranch: "main", theirBranch: "", conflicts: [kick], combined: [],
      theirs: version("t1", "Yi", "u-yi", "From the laptop") });
    render(MergeDecisions, p);
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("Both sides changed kick.wav");
    screen.getByText(/Bringing the team's versions into/);
    screen.getByText(/^Everything else was combined on its own\./);
    screen.getByRole("radio", { name: /Keep this computer's/ });
    await fireEvent.click(screen.getByRole("radio", { name: /Keep the team's/ }));
    expect(next().textContent).toBe("Combine");
    await fireEvent.click(next());
    expect(p.onresolve).toHaveBeenCalledWith({ "file:Samples/kick.wav": "theirs" });
  });

  it("undoing: keep it as it is now, or take it back", async () => {
    const p = props({ kind: "undo" as const, version: "Bass EQ", conflicts: [kick], ours: version("o1", "Yi", "u-yi", "Later"),
      theirs: version("b1", "Mo", "u-mo", "Before") });
    render(MergeDecisions, p);
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("kick.wav changed again after that version");
    screen.getByText("Undoing “Bass EQ”");
    screen.getByRole("radio", { name: /Keep it as it is now/ });
    await fireEvent.click(screen.getByRole("radio", { name: /Take it back anyway/ }));
    expect(next().textContent).toBe("Undo");
    await fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    screen.getByRole("dialog", { name: "Stop undoing?" });
  });

  it("works with conflicts sent the older way (no sides)", async () => {
    const p = props({ ours: null, theirs: null, combined: [], kind: "update" as const, ourBranch: "main", theirBranch: "",
      conflicts: [{ key: "k1", file: "Song.als", unit: "Bass", description: "both changed it", canKeepBoth: false }] });
    render(MergeDecisions, p);
    expect(screen.getByRole("dialog").getAttribute("aria-label")).toBe("Both sides changed the same part of Song.als");
    await fireEvent.click(screen.getByRole("radio", { name: /Keep this computer's/ }));
    await fireEvent.click(next());
    expect(p.onresolve).toHaveBeenCalledWith({ k1: "ours" });
  });
});
