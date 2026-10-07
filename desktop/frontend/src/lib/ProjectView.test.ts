import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// ProjectView with the Go bindings mocked: every binding is a mock that
// resolves to null unless a test says otherwise. These tests pin what the
// page does (which binding it calls, with what, and what it says), so it
// can be split into smaller components without changing it.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  // Events the page listens to ("progress", "files"), to send in tests.
  const handlers: Record<string, ((ev: { data: unknown }) => void)[]> = {};
  return { fns, api, toast: vi.fn(), handlers };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: (name: string, fn: (ev: { data: unknown }) => void) => {
    (mocks.handlers[name] ??= []).push(fn);
    return () => { mocks.handlers[name] = mocks.handlers[name].filter((f) => f !== fn); };
  } } }));
const emit = (name: string, data: unknown) => (mocks.handlers[name] ?? []).forEach((fn) => fn({ data }));

import ProjectView from "./ProjectView.svelte";
import { preuploads, queue } from "./preupload.svelte";

// jsdom has no ResizeObserver (bind:clientHeight uses it).
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;

let n = 0;
let ROOT = "";
beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  toast.mockClear();
  ROOT = `C:/Projects/P${++n}`; // the state cache is per root
  api.SampleSpots.mockResolvedValue([]);
  api.CommitWarnings.mockResolvedValue([]);
});
afterEach(() => cleanup());

const version = (id: string, message: string, over: Record<string, unknown> = {}) => ({
  id, short: id.slice(0, 7), author: "Alex", time: "2026-10-05T10:00:00Z", message, parents: ["p"],
  branches: [], authorId: "", inBranch: true, notHere: false, ...over,
});
const change = (path: string, status = "modified") =>
  ({ path, status, from: "", edited: false, details: [], weight: "", tracks: [] });
const state = (over: Record<string, unknown> = {}) => ({
  rules: { applied: [], fromFile: false, error: "", suggestions: [] },
  root: ROOT, name: "Song", author: "Yi", branch: "main", remoteUrl: "s3+https://storage/team",
  teamId: "t", teamName: "Band", teamChecked: true, unshared: false, online: true, offline: "",
  liveRunning: false, head: "h1", tool: "Ableton Live", openable: ["Song.als"], olderVersion: null,
  latest: "h1", unfinished: null, cloudFolder: "", changes: [] as ReturnType<typeof change>[], myEdits: [], incoming: [], takenBack: [],
  history: [version("h1", "v1", { author: "Yi", parents: [] })],
  branches: [{ name: "main", current: true, latest: null }, { name: "idea", current: false, latest: null }],
  ...over,
});
const result = (action: string, over: Record<string, unknown> = {}) => ({
  action, log: [], relinked: [], conflicts: [], liveRunning: false, openSet: "", keptWork: false, takenBack: [], ...over,
});

async function show(over: Record<string, unknown> = {}, props: Record<string, unknown> = {}) {
  const st = state(over);
  api.State.mockResolvedValue(st);
  // The Changes list shows the changed files.
  api.ProjectFiles.mockResolvedValue(st.changes.map((c) => ({ path: c.path, status: c.status, size: 1, kind: "other",
    live: "", from: "", edited: false, preview: false, video: false, model: false })));
  const onchanged = vi.fn();
  render(ProjectView, { root: ROOT, refreshKey: 0, teams: [], onchanged, onsettings: vi.fn(), ...props });
  await screen.findByRole("heading", { name: "Song" });
  return { onchanged };
}

async function typeMessage(text: string) {
  const box = screen.getByPlaceholderText(/What did you change/);
  await fireEvent.input(box, { target: { value: text } });
}
const commitButton = () => screen.getByRole("button", { name: /^Commit/ });
const toasted = (text: string | RegExp) =>
  waitFor(() => expect(toast.mock.calls.some(([t]) => (typeof text === "string" ? t === text : text.test(t)))).toBe(true));

describe("ProjectView: committing", () => {
  it("commits all the changes and shares them", async () => {
    await show({ changes: [change("Song.als"), change("Samples/kick.wav", "added")] });
    api.Save.mockResolvedValue(result("published"));
    await typeMessage("New chorus");
    expect(commitButton().textContent).toContain("Commit & Share");
    await fireEvent.click(commitButton());
    await waitFor(() => expect(api.Save).toHaveBeenCalledWith(ROOT, "New chorus", false, {}, false, []));
    await toasted("Version committed and shared with the team");
  });

  it("lists a few changes, makes a tree of many, keeps the one picked until the next commit", async () => {
    const many = Array.from({ length: 11 }, (_, i) => change(`Samples/take ${i}.wav`, "added"));
    await show({ changes: many });
    const pressed = (name: string) => screen.getByRole("button", { name }).getAttribute("aria-pressed");
    await screen.findByTitle("Samples/take 0.wav");
    expect(pressed("Tree")).toBe("true"); // more than 10
    await fireEvent.click(screen.getByRole("button", { name: "List" }));
    expect(pressed("List")).toBe("true");
    expect(document.querySelector("li.two")).toBeTruthy();
    expect(screen.getAllByText("Samples").length).toBeGreaterThan(0); // the folder under each name
    // (kept when the project is read again)
    cleanup();
    await show({ changes: many });
    expect(pressed("List")).toBe("true");
    // A commit: back to choosing by the number.
    api.Save.mockResolvedValue(result("published"));
    await typeMessage("Takes");
    await fireEvent.click(commitButton());
    await toasted("Version committed and shared with the team");
    cleanup();
    await show({ changes: many });
    expect(pressed("Tree")).toBe("true");
    cleanup();
    await show({ changes: many.slice(0, 3) });
    await waitFor(() => expect(pressed("List")).toBe("true")); // a few (once read: the last state shows first)
  });

  it("says how many changes are ticked, and how big they are", async () => {
    await show({ changes: [change("Song.als"), change("notes.txt", "added"), change("kick.wav", "added")] });
    const total = () => document.querySelector(".h-total")?.textContent?.replace(/\s+/g, " ").trim();
    await waitFor(() => expect(total()).toBe("3/3 selected · 3 B"));
    const row = (await screen.findByTitle("notes.txt")).closest("li")!;
    await fireEvent.click(within(row).getByTitle("Commit this change"));
    expect(total()).toBe("2/3 selected · 2 B");
    expect(document.querySelector(".h-title")?.textContent).toBe("Changes");
  });

  it("goes through the changes with the keyboard: folders open and close", async () => {
    await show({ changes: [change("Samples/kick.wav", "added"), change("Song.als")] });
    const list = document.querySelector<HTMLElement>("aside.files")!;
    await fireEvent.click(screen.getByRole("button", { name: "Tree" }));
    await screen.findByTitle("Samples/kick.wav");
    await fireEvent.keyDown(list, { key: "ArrowDown" }); // the folder
    await fireEvent.keyDown(list, { key: "ArrowLeft" }); // closes it
    expect(screen.queryByTitle("Samples/kick.wav")).toBeNull();
    await fireEvent.keyDown(list, { key: "ArrowRight" }); // opens it
    await fireEvent.keyDown(list, { key: "ArrowDown" });
    expect(screen.getByTitle("Samples/kick.wav").classList.contains("on")).toBe(true);
    await fireEvent.keyDown(list, { key: "ArrowLeft" }); // up to its folder
    await fireEvent.keyDown(list, { key: "Enter" }); // closes it
    expect(screen.queryByTitle("Samples/kick.wav")).toBeNull();
  });

  it("commits only the ticked changes", async () => {
    await show({ changes: [change("Song.als"), change("notes.txt", "added")] });
    api.Save.mockResolvedValue(result("published"));
    const row = (await screen.findByTitle("notes.txt")).closest("li")!;
    await fireEvent.click(within(row).getByTitle("Commit this change"));
    await typeMessage("Only the set");
    expect(commitButton().textContent).toContain("Commit 1 of 2 & Share");
    await fireEvent.click(commitButton());
    await waitFor(() => expect(api.Save).toHaveBeenCalledWith(ROOT, "Only the set", false, {}, false, ["Song.als"]));
  });

  it("says what the project's checks warn about first", async () => {
    await show({ changes: [change("Assets/a.png", "added")] });
    api.CommitWarnings.mockResolvedValue(["Assets/a.png has no .meta yet"]);
    api.Save.mockResolvedValue(result("published"));
    await typeMessage("Art");
    await fireEvent.click(commitButton());
    await screen.findByText(/Assets\/a.png has no .meta yet/);
    expect(api.Save).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("button", { name: "Commit anyway" }));
    await waitFor(() => expect(api.Save).toHaveBeenCalledTimes(1));
  });

  it("asks to close the set in Live, then tries again", async () => {
    await show({ changes: [change("Song.als")] });
    api.Save.mockResolvedValueOnce(result("", { liveRunning: true, openSet: "Song.als" }))
      .mockResolvedValueOnce(result("published"));
    await typeMessage("Mix");
    await fireEvent.click(commitButton());
    await screen.findByText("“Song.als” is open in Live");
    await fireEvent.click(screen.getByRole("button", { name: "I closed it — continue" }));
    await waitFor(() => expect(api.Save).toHaveBeenCalledTimes(2));
    await toasted("Version committed and shared with the team");
  });

  it("asks about conflicts and tries again with the decisions", async () => {
    await show({ changes: [change("Song.als")] });
    api.Save.mockResolvedValueOnce(result("", {
      conflicts: [{ key: "k1", file: "Song.als", unit: "Bass", description: "both changed it", canKeepBoth: false }],
    })).mockResolvedValueOnce(result("published"));
    await typeMessage("Bass");
    await fireEvent.click(commitButton());
    await screen.findByText("You and the team changed the same things");
    await fireEvent.click(screen.getAllByRole("button", { name: "Keep mine" })[0]);
    await fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    await waitFor(() => expect(api.Save).toHaveBeenLastCalledWith(ROOT, "Bass", false, { k1: "ours" }, false, []));
  });

  it("shows what the team committed in the meantime before combining", async () => {
    await show({ changes: [change("Song.als")] });
    api.Save.mockResolvedValue(result("behind"));
    api.PreviewUpdate.mockResolvedValue({ action: "merge", versions: [version("t1", "Drums")], changes: [], conflicts: [],
      message: "", base: "h1", target: "t1" });
    await typeMessage("Keys");
    await fireEvent.click(commitButton());
    await screen.findByText("New on “main”");
    expect(api.PreviewUpdate).toHaveBeenCalledWith(ROOT);
  });
});

describe("ProjectView: the team", () => {
  it("tells about teammates' new versions and gets them", async () => {
    await show({ incoming: [version("t1", "Drums")] });
    expect(screen.getByText(/shared 1 new version/)).toBeTruthy();
    api.Update.mockResolvedValue(result("fast-forward"));
    await fireEvent.click(screen.getByRole("button", { name: "Get updates" }));
    await waitFor(() => expect(api.Update).toHaveBeenCalledWith(ROOT, {}, false));
    await toasted(/^You're up to date/);
  });

  it("finishes a download that was cut off, instead of asking for a first commit", async () => {
    await show({ head: "", latest: "", history: [], incoming: [version("t1", "Team's version")] });
    api.Update.mockResolvedValue(result("fast-forward"));
    await screen.findByText(/The download didn't finish/);
    expect(screen.queryByText(/Not shared with/)).toBeNull();
    expect(screen.queryByText(/shared 1 new version/)).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Finish downloading" }));
    await waitFor(() => expect(api.Update).toHaveBeenCalledWith(ROOT, {}, false));
  });

  it("tells about a version a teammate took back", async () => {
    await show({ takenBack: [version("w1", "oops")] });
    expect(screen.getByText(/Alex took back “oops”/)).toBeTruthy();
    api.Update.mockResolvedValue(result("taken-back", { takenBack: [version("w1", "oops")] }));
    await fireEvent.click(screen.getByRole("button", { name: "Get updates" }));
    await toasted(/Alex took back “oops”. It's taken out of your files too/);
  });

  it("switches branch from the branch menu", async () => {
    await show();
    api.SwitchBranch.mockResolvedValue(result("moved"));
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    // The branch you are on, first; not one to switch to.
    const menu = screen.getByRole("menu");
    expect(menu.textContent).toMatch(/Current branch\s*main/);
    expect(within(menu).queryByRole("button", { name: /^main/ })).toBeNull();
    // "idea" is there twice: to switch to, and to merge from (after).
    await fireEvent.click(within(screen.getByRole("menu")).getAllByRole("button", { name: /^idea/ })[0]);
    await waitFor(() => expect(api.SwitchBranch).toHaveBeenCalledWith(ROOT, "idea", false));
    await toasted(/Now working on “idea”/);
  });

  it("calls branches by their names, and opens a branch's settings", async () => {
    await show({ branchNames: true, branches: [{ name: "main", label: "", color: "", current: true, latest: null },
      { name: "b-1a2b3c4d", label: "Mia 的主歌", color: "b4", current: false, latest: null }] });
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    const menu = screen.getByRole("menu");
    expect(menu.textContent).toContain("Mia 的主歌");
    expect(menu.textContent).not.toContain("b-1a2b3c4d");
    await fireEvent.click(within(menu).getAllByRole("button", { name: "Branch settings" })[1]);
    const dialog = screen.getByRole("dialog", { name: "Branch settings" });
    expect((within(dialog).getByLabelText("Branch name") as HTMLInputElement).value).toBe("Mia 的主歌");
    expect(within(dialog).getByRole("radio", { name: "Pink" }).getAttribute("aria-checked")).toBe("true");
    api.SetBranchRecord.mockResolvedValue(undefined);
    await fireEvent.click(within(dialog).getByRole("radio", { name: "Mint" }));
    await waitFor(() => expect(api.SetBranchRecord).toHaveBeenCalledWith(ROOT, "b-1a2b3c4d", "Mia 的主歌", "b7"));
    await fireEvent.input(within(dialog).getByLabelText("Branch name"), { target: { value: "Verse, take 2" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Rename" }));
    await waitFor(() => expect(api.SetBranchRecord).toHaveBeenLastCalledWith(ROOT, "b-1a2b3c4d", "Verse, take 2", "b7"));
  });

  it("deletes a branch, saying what stays", async () => {
    await show({ branchNames: true, branches: [{ name: "main", label: "", color: "", current: true, latest: null },
      { name: "idea", label: "Idea", color: "", current: false, latest: null }] });
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    await fireEvent.click(within(screen.getByRole("menu")).getAllByRole("button", { name: "Branch settings" })[1]);
    api.VersionsOnlyOnBranch.mockResolvedValue(2);
    api.DeleteBranch.mockResolvedValue(undefined);
    await fireEvent.click(screen.getByRole("button", { name: "Delete branch…" }));
    expect(await screen.findByText(/2 versions are only on this branch/)).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Delete branch" }));
    await waitFor(() => expect(api.DeleteBranch).toHaveBeenCalledWith(ROOT, "idea"));
    await toasted(/Deleted “Idea”/);
  });

  it("can't delete the branch you're on, nor main", async () => {
    await show({ branchNames: true, branch: "idea", branches: [{ name: "main", label: "", color: "", current: false, latest: null },
      { name: "idea", label: "", color: "", current: true, latest: null }] });
    await fireEvent.click(screen.getByRole("button", { name: /idea ▾/ }));
    const gears = within(screen.getByRole("menu")).getAllByRole("button", { name: "Branch settings" });
    await fireEvent.click(gears[0]); // the one you're on
    expect(screen.getByText(/switch to another one to delete it/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Delete branch…" })).toBeNull();
  });

  it("says the branch you're on was deleted, and brings it back", async () => {
    await show({ branch: "idea", branchGone: { name: "idea", label: "Idea", color: "", by: "Mia", time: new Date().toISOString(), latest: null } });
    expect(screen.getByText(/was deleted from the team by Mia/)).toBeTruthy();
    api.RestoreBranch.mockResolvedValue(undefined);
    await fireEvent.click(screen.getByRole("button", { name: "Restore it" }));
    await waitFor(() => expect(api.RestoreBranch).toHaveBeenCalledWith(ROOT, "idea"));
    await toasted(/“Idea” is back/);
  });

  it("offers no branch settings where the team keeps no names", async () => {
    await show();
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    expect(within(screen.getByRole("menu")).queryByRole("button", { name: "Branch settings" })).toBeNull();
  });

  it("makes a new branch", async () => {
    await show();
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    await fireEvent.click(screen.getByRole("button", { name: "New branch from here…" }));
    await fireEvent.input(screen.getByLabelText("Branch name"), { target: { value: "yi-idea" } });
    await fireEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(api.CreateBranch).toHaveBeenCalledWith(ROOT, "yi-idea", "b1")); // the palette's first colour no branch has
  });
});

describe("ProjectView: versions", () => {
  it("goes back to the latest version from an older one", async () => {
    await show({ olderVersion: version("h0", "old one") });
    expect(screen.getAllByText(/You're on an older version/).length).toBeGreaterThan(0);
    api.GoToVersion.mockResolvedValue(result("moved"));
    await fireEvent.click(screen.getAllByRole("button", { name: "Back to latest" })[0]);
    await waitFor(() => expect(api.GoToVersion).toHaveBeenCalledWith(ROOT, "latest", false, false));
  });

  it("asks about uncommitted changes before going to a version", async () => {
    await show({ changes: [change("Song.als")], history: [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })] });
    api.VersionFiles.mockResolvedValue([]);
    await fireEvent.click(screen.getByRole("option", { name: /^v1,/ }));
    await fireEvent.click(await screen.findByRole("button", { name: "Go to" }));
    await screen.findByText("Go to an older version");
    api.GoToVersion.mockResolvedValue(result("moved"));
    await fireEvent.click(screen.getByRole("button", { name: "Discard changes" }));
    await waitFor(() => expect(api.GoToVersion).toHaveBeenCalledWith(ROOT, "h0", true, false));
  });

  it("takes back the latest version", async () => {
    await show({ history: [version("h1", "oops", { author: "Yi" }), version("p", "v1", { parents: [] })] });
    api.PlanUndo.mockResolvedValue({ changed: ["a.txt"], blocked: [], conflicts: [], error: "",
      takeBack: { ok: true, why: "", haveIt: [], branches: [], shared: true, featureOff: false } });
    api.TakeBackVersion.mockResolvedValue(result("taken-back"));
    api.VersionFiles.mockResolvedValue([]);
    await fireEvent.click(await screen.findByRole("button", { name: "Undo" })); // (the version you're on, shown)
    await screen.findByText(/Removes it from the history, yours and the team's/);
    await fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Undo commit" }));
    await waitFor(() => expect(api.TakeBackVersion).toHaveBeenCalledWith(ROOT, "h1", true));
  });
});

describe("ProjectView: tabs", () => {
  it("shows Overview, Files and Settings only (the earlier Changes and History are hidden)", async () => {
    localStorage.setItem(`r3v.tab:${ROOT}`, "history"); // (remembered from before)
    await show({ changes: [change("Song.als")] });
    const nav = document.querySelector("nav")!;
    expect([...nav.querySelectorAll("button")].map((b) => b.textContent?.replace(/\d+/g, "").trim())).toEqual(["Overview", "Files", "Settings"]);
    expect(nav.querySelector("button.on")?.textContent).toContain("Overview");
  });
});

describe("ProjectView: a project just added", () => {
  it("asks to commit and share a first version", async () => {
    const onfirstshared = vi.fn();
    await show({ head: "", latest: "", history: [], changes: [change("Song.als", "added")] }, { firstShare: true, onfirstshared });
    await screen.findByText("“Song” is added");
    expect(onfirstshared).toHaveBeenCalled();
    api.Save.mockResolvedValue(result("published"));
    await fireEvent.click(screen.getByRole("button", { name: "Commit & Share now" }));
    await waitFor(() => expect(api.Save).toHaveBeenCalledWith(ROOT, "First version", false, {}, false, []));
  });

  it("asks to share the versions of a project that joined a team", async () => {
    await show({ unshared: true }, { firstShare: true });
    await screen.findByText("Share “Song” with Band?");
    api.ShareVersions.mockResolvedValue(result("published"));
    await fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Share now" }));
    await waitFor(() => expect(api.ShareVersions).toHaveBeenCalledWith(ROOT));
  });
});

describe("ProjectView: an older version without a team", () => {
  it("makes it the latest version, described", async () => {
    await show({ remoteUrl: "", teamName: "", olderVersion: version("h0", "old one") });
    await fireEvent.click(screen.getByRole("button", { name: "Make this the latest…" }));
    const box = screen.getByLabelText("Describe it") as HTMLInputElement;
    expect(box.value).toBe("Back to “old one”");
    await fireEvent.input(box, { target: { value: "Old one again" } });
    api.KeepThisVersion.mockResolvedValue(result("kept"));
    await fireEvent.click(screen.getByRole("button", { name: "Make it the latest" }));
    await waitFor(() => expect(api.KeepThisVersion).toHaveBeenCalledWith(ROOT, "Old one again", {}));
  });
});

describe("ProjectView: while a version is made", () => {
  it("cancels a commit under way, keeping the message and the ticks", async () => {
    await show({ changes: [change("Song.als"), change("notes.txt", "added")] });
    let finish = (_r: unknown) => {};
    api.Save.mockReturnValue(new Promise((ok) => (finish = ok)));
    api.CancelSave.mockResolvedValue(true);
    const row = (await screen.findByTitle("notes.txt")).closest("li")!;
    await fireEvent.click(within(row).getByTitle("Commit this change"));
    await typeMessage("Oops, wrong text");
    await fireEvent.click(commitButton());
    emit("progress", { root: ROOT, stage: "uploading", done: 0, total: 1, bytes: 10, totalBytes: 100, cancellable: true });
    await fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    expect(api.CancelSave).toHaveBeenCalledWith(ROOT);
    await screen.findByRole("button", { name: "Cancelling…" });
    expect(api.Save).toHaveBeenCalledTimes(1); // (the click didn't open the queue or commit again)
    finish(result("cancelled"));
    await toasted("Commit cancelled: your changes are as they were");
    expect((screen.getByPlaceholderText(/What did you change/) as HTMLTextAreaElement).value).toBe("Oops, wrong text");
    expect(commitButton().textContent).toContain("Commit 1 of 2");
  });

  it("offers no cancel once the team has it (or for an update)", async () => {
    await show({ changes: [change("Song.als")] });
    emit("progress", { root: ROOT, stage: "uploading", done: 0, total: 1, bytes: 10, totalBytes: 100 });
    await screen.findByText(/you can keep working while it uploads/);
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });

  it("dims the changes while it reads the files, then says to go on working", async () => {
    await show({ changes: [change("Song.als")] });
    emit("progress", { root: ROOT, stage: "storing", done: 1, total: 3 });
    await screen.findByText("Don't change the project's files until this step is done.");
    expect(document.querySelector("main")!.classList.contains("reading")).toBe(true);
    emit("progress", { root: ROOT, stage: "uploading", done: 1, total: 3, bytes: 10, totalBytes: 100 });
    await screen.findByText(/you can keep working while it uploads/);
    expect(screen.queryByText(/Don't change the project's files/)).toBeNull();
    expect(document.querySelector("main")!.classList.contains("reading")).toBe(false);
    emit("progress", { root: ROOT, stage: "done", done: 0, total: 0 });
    await waitFor(() => expect(screen.queryByText(/keep working while it uploads/)).toBeNull());
  });
});

describe("ProjectView: a big file going up in the background", () => {
  it("shows a small moving icon by the team", async () => {
    await show();
    expect(screen.queryByTitle(/in the background/)).toBeNull();
    preuploads[ROOT] = { root: ROOT, path: "Video/take.mov", bytes: 25, total: 100, done: false, waiting: [], speed: 10 };
    // What, how far and how fast on hover; a click opens the upload queue.
    const icon = await screen.findByTitle(/take\.mov · .* · 25% · .*\/s/);
    await fireEvent.click(icon);
    expect(queue.open).toBe(true);
    queue.open = false;
    delete preuploads[ROOT];
    await waitFor(() => expect(screen.queryByTitle(/in the background/)).toBeNull());
  });
});

describe("ProjectView: discarding", () => {
  it("asks before discarding everything", async () => {
    await show({ changes: [change("Song.als"), change("notes.txt", "added")] });
    api.DiscardAll.mockResolvedValue(result("discarded"));
    await fireEvent.click(await screen.findByLabelText("Discard the ticked changes"));
    await screen.findByText("Discard all your changes?");
    expect(api.DiscardAll).not.toHaveBeenCalled();
    await fireEvent.click(screen.getByRole("button", { name: "Discard all" }));
    await waitFor(() => expect(api.DiscardAll).toHaveBeenCalledWith(ROOT, false));
  });
});

describe("ProjectView: Overview", () => {
  it("names a version as a milestone, shows it, and goes to it from the branch menu", async () => {
    const ms = { id: "m1", version: "h0", name: "Sent to the label", note: "long intro", by: "Mia", time: "2026-10-05T10:00:00Z" };
    await show({ branchNames: true, milestones: [ms],
      history: [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })] });
    // The flag on its dot.
    expect(screen.getByRole("option", { name: /^v1,.*⚑ Sent to the label/ })).toBeTruthy();
    // From the branch menu, to its version.
    api.VersionFiles.mockResolvedValue([]);
    await fireEvent.click(screen.getByRole("button", { name: /main ▾/ }));
    await fireEvent.click(within(screen.getByRole("menu")).getByRole("button", { name: /Sent to the label/ }));
    await screen.findByRole("heading", { name: "v1" });
    // On the version: the milestone, which opens to be changed or taken away.
    await fireEvent.click(screen.getByRole("button", { name: /^Sent to the label$/ }));
    const dialog = screen.getByRole("dialog", { name: "Milestone" });
    api.EditMilestone.mockResolvedValue(undefined);
    await fireEvent.input(within(dialog).getByRole("textbox", { name: "Name" }), { target: { value: "Sent to the label, v1" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(api.EditMilestone).toHaveBeenCalledWith(ROOT, "m1", "Sent to the label, v1", "long intro"));
    // A new one, on the other version.
    await fireEvent.click(screen.getByRole("option", { name: /^v2,/ }));
    await screen.findByRole("heading", { name: "v2" });
    api.AddMilestone.mockResolvedValue(undefined);
    await fireEvent.click(screen.getByRole("button", { name: "Milestone…" }));
    await fireEvent.input(within(screen.getByRole("dialog", { name: "Mark as a milestone" })).getByRole("textbox", { name: "Name" }),
      { target: { value: "Final mix" } });
    await fireEvent.click(screen.getByRole("button", { name: "Add milestone" }));
    await waitFor(() => expect(api.AddMilestone).toHaveBeenCalledWith(ROOT, "h1", "Final mix", ""));
  });

  it("starts on your changes, and shows a version picked in the graph", async () => {
    await show({ changes: [change("Song.als")], history: [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })] });
    expect(screen.getByRole("option", { name: /Your changes/ }).getAttribute("aria-selected")).toBe("true");
    commitButton(); // your changes, with the commit box
    api.VersionFiles.mockResolvedValue([]);
    await fireEvent.click(screen.getByRole("option", { name: /^v1,/ }));
    await screen.findByRole("heading", { name: "v1" });
    await waitFor(() => expect(api.VersionFiles).toHaveBeenCalledWith(ROOT, "h0"));
    await screen.findByText("No file changes.");
    api.GoToVersion.mockResolvedValue(result("moved"));
    await fireEvent.click(screen.getByRole("button", { name: "Go to" }));
    await screen.findByText("Go to an older version");
  });
});

describe("ProjectView: reloading", () => {
  it("doesn't read a picked version's files again when the window comes back (no blinking)", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const history = () => [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })];
      await show({ changes: [change("Song.als")], history: history() });
      api.TeamState.mockImplementation(async () => ({ history: history(), incoming: [], branches: [], takenBack: [],
        online: true, offline: "", unshared: false }));
      api.VersionFiles.mockResolvedValue([{ path: "a.txt", status: "modified", size: 1, kind: "other", live: "", from: "",
        edited: false, preview: false, video: false, model: false }]);
      await fireEvent.click(screen.getByRole("option", { name: /^v1,/ }));
      await screen.findByTitle("a.txt");
      const calls = api.VersionFiles.mock.calls.length;
      vi.advanceTimersByTime(4000); // (reloads on focus are spaced out)
      window.dispatchEvent(new Event("focus"));
      await waitFor(() => expect(api.TeamState.mock.calls.length).toBeGreaterThan(1));
      await new Promise((r) => setTimeout(r, 50));
      expect(screen.queryByText("Reading…")).toBeNull();
      expect(api.VersionFiles.mock.calls.length).toBe(calls);
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps what the team said while it reads the project again (no blinking banner)", async () => {
    api.TeamState.mockResolvedValue({ online: true, offline: "", branches: [], incoming: [], takenBack: [],
      history: [version("h1", "v1", { parents: [] })], olderVersion: null, unshared: true, capabilities: {} });
    await show({ unshared: false });
    const banner = /its versions are on this computer only/;
    await screen.findByText(banner);
    // The window gets focus again (e.g. while being resized): the project is
    // read again, and the team is slow to answer this time.
    api.TeamState.mockReturnValue(new Promise(() => {}));
    const now = Date.now;
    Date.now = () => now() + 10_000;
    try {
      await fireEvent(window, new Event("focus"));
      await waitFor(() => expect(api.State).toHaveBeenCalledTimes(2));
      await new Promise((r) => setTimeout(r));
      expect(screen.queryByText(banner)).not.toBeNull();
    } finally {
      Date.now = now;
    }
  });
});

describe("ProjectView: files it can't read", () => {
  it("says which files another program holds", async () => {
    await show({ inUse: ["Samples/Processed/Freeze/Freeze 1.wav"] });
    await screen.findByText(/Freeze 1\.wav is in use by another program/);
  });

  it("says when the project can't be read again, keeping what it showed", async () => {
    await show({ changes: [change("Song.als")] });
    api.State.mockRejectedValue(new Error("open Song.als: in use"));
    const now = Date.now;
    Date.now = () => now() + 10_000;
    try {
      await fireEvent(window, new Event("focus"));
      await screen.findByText(/can't read the project right now.*open Song\.als: in use/);
      expect(commitButton()).toBeTruthy(); // the changes are still shown
    } finally {
      Date.now = now;
    }
  });
});

describe("ProjectView: review fixes", () => {
  const later = () => { const now = Date.now; Date.now = () => now() + 10_000; return () => { Date.now = now; }; };

  it("forgets the team's news once the project has left the team", async () => {
    const news = version("t1", "Mia's mix", { author: "Mia", parents: ["h1"] });
    api.TeamState.mockResolvedValue({ online: true, offline: "", branches: [], incoming: [news], takenBack: [],
      history: [news, version("h1", "v1", { parents: [] })], olderVersion: null, unshared: false, capabilities: {} });
    await show();
    await screen.findByRole("button", { name: "Get updates" });
    // Detached from the team: same version, same branch, no team any more.
    api.State.mockResolvedValue(state({ remoteUrl: "", teamId: "", teamName: "" }));
    const back = later();
    try {
      await fireEvent(window, new Event("focus"));
      await waitFor(() => expect(screen.queryByRole("button", { name: "Get updates" })).toBeNull());
    } finally {
      back();
    }
  });

  it("doesn't ask for a branch name after going to a version when New branch was cancelled", async () => {
    await show({ changes: [change("Song.als")], history: [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })] });
    // New branch from v1 on its card: your changes first; cancelled.
    await fireEvent.mouseEnter(screen.getByRole("option", { name: /^v1,/ }));
    await fireEvent.click(await screen.findByRole("button", { name: "New branch" }));
    await fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    // Later: going to v1, discarding the changes.
    await fireEvent.click(screen.getByRole("option", { name: /^v1,/ }));
    api.VersionFiles.mockResolvedValue([]);
    api.GoToVersion.mockResolvedValue(result("moved"));
    // (the version's Go to; its card may still be open, with one too)
    await waitFor(() => expect(screen.getAllByRole("button", { name: "Go to" }).length).toBeGreaterThan(0));
    await fireEvent.click(screen.getAllByRole("button", { name: "Go to" })[0]);
    await fireEvent.click(await screen.findByRole("button", { name: "Discard changes" }));
    await waitFor(() => expect(api.GoToVersion).toHaveBeenCalledWith(ROOT, "h0", true, false));
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByText("Branch name")).toBeNull();
  });

  it("doesn't show the error of a version picked before", async () => {
    await show({ history: [version("h1", "v2", { parents: ["h0"] }), version("h0", "v1", { parents: [] })] });
    let fail!: (e: Error) => void;
    api.VersionFiles.mockReturnValueOnce(new Promise((_, rej) => (fail = rej)));
    await fireEvent.click(screen.getByRole("option", { name: /^v1,/ }));
    api.VersionFiles.mockResolvedValueOnce([]);
    await fireEvent.click(screen.getByRole("option", { name: /^v2,/ }));
    await screen.findByText("No file changes.");
    fail(new Error("team storage not reachable"));
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.queryByText("team storage not reachable")).toBeNull();
    expect(screen.getByText("No file changes.")).toBeTruthy();
  });
});
