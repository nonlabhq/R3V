import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// Looks (Nightly): the user's colour and picture in User settings, a
// project's icon and colour, and teammates' pictures on the history graph;
// with the Go bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api, toast: vi.fn(), square: vi.fn() };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));
vi.mock("./picture", () => ({ squarePicture: mocks.square, SIDE: 128 }));

import UserSettings from "./UserSettings.svelte";
import ProjectSettings from "./ProjectSettings.svelte";
import HistoryGraph from "./HistoryGraph.svelte";
import type { TeamSummary } from "./api";

globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} } as unknown as typeof ResizeObserver;

const PIC = "data:image/png;base64,iVBORw0KGgo=";
const profile = (over = {}) => ({ available: true, name: "Yi", memberId: "m1", color: "", picture: "", notShared: [], ...over });
const team = (over: Partial<TeamSummary> = {}) =>
  ({ id: "t1", name: "Band", memberId: "m1", memberName: "Yi", looks: true, ...over }) as TeamSummary;

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  toast.mockClear();
  mocks.square.mockReset();
});
afterEach(() => cleanup());

function pickFile(file: File) {
  const input = screen.getByTestId("picture-file") as HTMLInputElement;
  Object.defineProperty(input, "files", { value: [file], configurable: true });
  return fireEvent.change(input);
}

describe("User settings", () => {
  it("changes the colour, at random too", async () => {
    api.Profile.mockResolvedValue(profile({ color: "b2" }));
    api.SetProfileColor.mockImplementation(async (c: string) => profile({ color: c }));
    const onchanged = vi.fn();
    render(UserSettings, { team: team(), onchanged, onclose: () => {} });
    const teal = await screen.findByRole("radio", { name: "Teal" });
    expect(teal.getAttribute("aria-checked")).toBe("true");
    await fireEvent.click(screen.getByRole("radio", { name: "Pink" }));
    await waitFor(() => expect(onchanged).toHaveBeenLastCalledWith(expect.objectContaining({ color: "b4" })));
    expect(api.SetProfileColor).toHaveBeenLastCalledWith("b4");
    await waitFor(() => expect(screen.getByRole("radio", { name: "Pink" }).getAttribute("aria-checked")).toBe("true"));
    await fireEvent.click(screen.getByRole("button", { name: "Random" }));
    await waitFor(() => expect(api.SetProfileColor).toHaveBeenCalledTimes(2));
    expect(api.SetProfileColor.mock.calls[1][0]).not.toBe("b4");
  });

  it("uploads a picture (made square and small first) and removes it", async () => {
    api.Profile.mockResolvedValue(profile());
    api.SetProfilePicture.mockImplementation(async (url: string) => profile({ picture: url, notShared: url ? ["Cloud band"] : [] }));
    mocks.square.mockResolvedValue(PIC);
    const { container } = render(UserSettings, { team: team(), onchanged: () => {}, onclose: () => {} });
    await screen.findByRole("button", { name: "Upload picture…" });
    const file = new File(["x"], "me.jpg", { type: "image/jpeg" });
    await pickFile(file);
    await waitFor(() => expect(api.SetProfilePicture).toHaveBeenCalledWith(PIC));
    expect(mocks.square).toHaveBeenCalledWith(file);
    await waitFor(() => expect(container.querySelector(".who img")?.getAttribute("src")).toBe(PIC));
    expect(toast).toHaveBeenCalledWith(expect.stringContaining("Cloud band"), "info", 8000);
    await fireEvent.click(screen.getByRole("button", { name: "Remove picture" }));
    await waitFor(() => expect(api.SetProfilePicture).toHaveBeenLastCalledWith(""));
    await waitFor(() => expect(container.querySelector(".who img")).toBeNull());
  });

  it("says when a file can't be read as a picture, and sends nothing", async () => {
    api.Profile.mockResolvedValue(profile());
    mocks.square.mockRejectedValue(new Error("decode"));
    render(UserSettings, { team: team(), onchanged: () => {}, onclose: () => {} });
    await screen.findByRole("button", { name: "Upload picture…" });
    await pickFile(new File(["x"], "a.txt"));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(api.SetProfilePicture).not.toHaveBeenCalled();
  });

  it("takes one pick at a time, and refuses a huge file before reading it", async () => {
    api.Profile.mockResolvedValue(profile({ color: "b2" }));
    let done!: () => void;
    api.SetProfileColor.mockImplementation((c: string) => new Promise((ok) => (done = () => ok(profile({ color: c })))));
    render(UserSettings, { team: team(), onchanged: () => {}, onclose: () => {} });
    await fireEvent.click(await screen.findByRole("radio", { name: "Pink" }));
    await fireEvent.click(screen.getByRole("radio", { name: "Blue" }));
    expect(api.SetProfileColor).toHaveBeenCalledTimes(1);
    done();
    await waitFor(() => expect(screen.getByRole("radio", { name: "Pink" }).getAttribute("aria-checked")).toBe("true"));
    const huge = new File(["x"], "huge.jpg");
    Object.defineProperty(huge, "size", { value: 100 << 20 });
    await pickFile(huge);
    expect((await screen.findByRole("alert")).textContent).toMatch(/too big/);
    expect(mocks.square).not.toHaveBeenCalled();
  });
});

describe("Project icon and colour", () => {
  const p = { id: "p1", name: "Song", root: "C:/Song", status: "remote", branch: "", branchLabel: "", icon: "", color: "" };
  const props = { p, onclose: () => {}, onrenamed: () => {}, oncheck: () => {}, ondelete: () => {}, onunlink: () => {}, onlocate: () => {} };

  it("picks an icon and a colour for the team", async () => {
    render(ProjectSettings, { ...props, team: team() });
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    await fireEvent.click(screen.getByRole("radio", { name: "drum" }));
    await waitFor(() => expect(api.SetProjectLook).toHaveBeenLastCalledWith("t1", "p1", "drum", ""));
    expect(screen.queryByRole("dialog", { name: "Icon and colour" })).toBeNull(); // an icon picked closes it
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    await fireEvent.click(screen.getByRole("radio", { name: "Lime" }));
    await waitFor(() => expect(api.SetProjectLook).toHaveBeenLastCalledWith("t1", "p1", "drum", "b5"));
    expect(screen.getByRole("radio", { name: "drum" }).getAttribute("aria-checked")).toBe("true"); // a colour doesn't
  });

  it("offers the programs' icons first, named by their brands", async () => {
    render(ProjectSettings, { ...props, team: team() });
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    const apps = screen.getByRole("radiogroup", { name: "Apps" });
    const radios = within(apps).getAllByRole("radio");
    expect(radios[0].getAttribute("aria-label")).toBe("Ableton Live");
    expect(within(apps).getByRole("radio", { name: "Blender" }).getAttribute("title")).toBe("Blender");
    expect(within(apps).queryByRole("radio", { name: "drum" })).toBeNull(); // the drawn ones after
    const groups = screen.getAllByRole("radiogroup");
    expect(groups.indexOf(apps)).toBeLessThan(groups.findIndex((g) => within(g).queryByRole("radio", { name: "drum" })));
    await fireEvent.click(within(apps).getByRole("radio", { name: "Unity" }));
    await waitFor(() => expect(api.SetProjectLook).toHaveBeenLastCalledWith("t1", "p1", "app-unity", ""));
  });

  it("goes back when the team doesn't take it", async () => {
    api.SetProjectLook.mockRejectedValue(new Error("the team doesn't have this project yet"));
    render(ProjectSettings, { ...props, team: team() });
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    await fireEvent.click(screen.getByRole("radio", { name: "drum" }));
    await waitFor(() => expect(toast).toHaveBeenCalledWith(expect.stringContaining("doesn't have"), "error"));
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    expect(screen.getByRole("radio", { name: "The project's initial" }).getAttribute("aria-checked")).toBe("true");
  });

  it("isn't offered where the team keeps no looks", () => {
    render(ProjectSettings, { ...props, team: team({ looks: false }) });
    expect(screen.queryByText("Icon and colour")).toBeNull();
  });

  it("opens the picker from the icon before the name, and Esc closes it", async () => {
    render(ProjectSettings, { ...props, team: team() });
    expect(screen.queryByRole("radio", { name: "drum" })).toBeNull();
    const icon = screen.getByRole("button", { name: "Icon and colour" });
    await fireEvent.click(icon);
    expect(screen.getByRole("dialog", { name: "Icon and colour" })).toBeTruthy();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Icon and colour" })).toBeNull();
    expect(document.activeElement).toBe(icon);
  });

  it("keeps the colours for the icons: the emoji tab has none", async () => {
    render(ProjectSettings, { ...props, team: team() });
    await fireEvent.click(screen.getByRole("button", { name: "Icon and colour" }));
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((b) => b.textContent)).toEqual(["Icons", "Emoji"]);
    expect(screen.getByRole("radio", { name: "Lime" })).toBeTruthy();
    await fireEvent.click(tabs[1]);
    expect(screen.queryByRole("radio", { name: "Lime" })).toBeNull();
  });
});

describe("History graph with looks", () => {
  const version = (id: string, author: string, authorId: string, parents: string[]) => ({
    id, short: id, author, time: "2026-10-06T10:00:00Z", message: id, parents, branches: [], authorId, inBranch: true, notHere: false, branch: "",
  });
  const versions = [version("v2", "Robin", "r1", ["v1"]), version("v1", "Yi", "y1", [])];
  const show = (looks?: Record<string, { color: string; picture: string }>) =>
    render(HistoryGraph, { versions, branches: [{ name: "main", latest: "v2" }], branch: "main", head: "v2",
      incoming: new Set<string>(), pending: 0, selected: "v2", onselect: () => {}, looks });

  it("shows an author's picture, else their initial on their colour", () => {
    show({ y1: { color: "", picture: PIC }, r1: { color: "b4", picture: "" } });
    const yi = screen.getByRole("option", { name: /^v1,/ });
    expect(yi.querySelector("img")?.getAttribute("src")).toBe(PIC);
    const robin = screen.getByRole("option", { name: /^v2,/ });
    expect(robin.querySelector("img")).toBeNull();
    expect(robin.textContent).toBe("R");
    expect(robin.style.getPropertyValue("--m")).toBe("var(--palette-b4)");
  });

  it("shows the initial when a picture can't be shown", async () => {
    show({ y1: { color: "", picture: PIC } });
    const yi = screen.getByRole("option", { name: /^v1,/ });
    await fireEvent.error(yi.querySelector("img")!);
    expect(yi.querySelector("img")).toBeNull();
    expect(yi.textContent).toBe("Y");
  });

  it("falls back to the initial for someone the team has no look for", () => {
    show({});
    const yi = screen.getByRole("option", { name: /^v1,/ });
    expect(yi.textContent).toBe("Y");
    expect(yi.classList.contains("tinted")).toBe(true); // a colour picked from their id
  });

  it("draws initials as before where the team keeps no looks", () => {
    show(undefined);
    const yi = screen.getByRole("option", { name: /^v1,/ });
    expect(yi.textContent).toBe("Y");
    expect(yi.classList.contains("tinted")).toBe(false);
  });
});
