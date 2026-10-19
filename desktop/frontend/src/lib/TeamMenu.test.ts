import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// The team in the sidebar and the list of teams behind its caret, the
// sidebar open or folded; with the bindings mocked.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import TeamMenu from "./TeamMenu.svelte";
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const overview = {
  teams: [{ id: "t1", name: "Band", memberId: "m1", color: "#3a6" }, { id: "t2", name: "Duo", memberId: "m2" }],
  currentTeam: "t1", projects: [], author: "", teamChecked: true, teamError: "",
} as never;
const props = (over = {}) => ({ overview, reload: vi.fn(async () => {}), onhome: vi.fn(), onhomebehind: vi.fn(), onteam: vi.fn(), onsettings: vi.fn(), ...over });

describe("TeamMenu", () => {
  it("lists each team with its mark before its name", async () => {
    render(TeamMenu, props());
    await fireEvent.click(screen.getByRole("button", { name: "Switch team" }));
    const items = [...screen.getByRole("menu").querySelectorAll(".item")].slice(0, 2);
    expect(items.map((b) => b.querySelector(".tname")?.textContent)).toEqual(["Band", "Duo"]);
    for (const b of items) {
      const mark = b.querySelector(".pi.solid"); // (TeamMark: a solid ProjectIcon)
      expect(mark).toBeTruthy();
      expect(mark!.compareDocumentPosition(b.querySelector(".tname")!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it("folded: the team's mark opens its home, and the caret beside it the same list of teams", async () => {
    const pr = props();
    render(TeamMenu, { ...pr, folded: true, home: true });
    const mark = screen.getByRole("button", { name: "Band" });
    expect(mark.getAttribute("aria-current")).toBe("page");
    expect(mark.textContent!.trim()).toBe("B"); // the mark alone, no name beside it
    await fireEvent.click(mark);
    expect(pr.onhome).toHaveBeenCalled();
    await fireEvent.click(mark, { ctrlKey: true });
    expect(pr.onhomebehind).toHaveBeenCalled();
    const caret = screen.getByRole("button", { name: "Switch team" });
    expect(caret.parentElement).toBe(mark.parentElement); // (over the room beside the mark: placed by its slot)
    await fireEvent.click(caret);
    expect(caret.getAttribute("aria-expanded")).toBe("true");
    await fireEvent.click(screen.getByRole("menu").querySelectorAll(".item")[1]);
    expect(pr.onteam).toHaveBeenCalledWith("t2", false);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
