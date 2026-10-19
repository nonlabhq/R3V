import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// Your name in a team is in User settings (from your picture at the side
// bar's bottom), not in the team's settings: in every build, looks or not.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return { fns, api: new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) }), toast: vi.fn() };
});
const { api } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));

import UserSettings from "./UserSettings.svelte";
import TeamSettings from "./TeamSettings.svelte";
import type { TeamSummary } from "./api";

const team = (over: Partial<TeamSummary> = {}) =>
  ({ id: "t1", name: "Band", memberId: "m1", memberName: "Yi", ...over }) as TeamSummary;
// (a build without looks: no colour or picture)
const profile = { available: false, name: "Yi", memberId: "m1", color: "", picture: "", notShared: [] };

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  api.Profile.mockResolvedValue(profile);
});
afterEach(() => cleanup());

describe("User settings: your name", () => {
  it("renames you in the team, without looks too", async () => {
    api.SetIdentity.mockImplementation(async (_t: string, id: string, name: string) => team({ memberId: id, memberName: name }));
    const onidentity = vi.fn();
    render(UserSettings, { team: team(), onchanged: () => {}, onidentity, onclose: () => {} });
    const name = await screen.findByRole("textbox", { name: "Name" });
    expect(screen.queryByRole("button", { name: "Upload picture…" })).toBeNull();
    await fireEvent.input(name, { target: { value: "Yi Chen" } });
    await fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    await waitFor(() => expect(api.SetIdentity).toHaveBeenCalledWith("t1", "m1", "Yi Chen"));
    await waitFor(() => expect(onidentity).toHaveBeenCalledWith(expect.objectContaining({ memberName: "Yi Chen" })));
  });

  it("asks who you are in a team where you have no name yet", async () => {
    api.TeamMembers.mockResolvedValue([]);
    render(UserSettings, { team: team({ memberId: "", memberName: "" }), author: "Yi", onchanged: () => {}, onclose: () => {} });
    expect(await screen.findByLabelText("Your name")).toBeTruthy();
  });

  it("is not in the team's settings", async () => {
    render(TeamSettings, { team: team(), reload: async () => {}, onclose: () => {}, inline: true });
    expect(await screen.findByRole("textbox", { name: "Team name" })).toBeTruthy();
    expect(screen.queryByText("Your name in this team")).toBeNull();
    expect(screen.queryByRole("button", { name: "Change…" })).toBeNull();
  });
});
