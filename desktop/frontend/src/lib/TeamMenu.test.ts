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
});
