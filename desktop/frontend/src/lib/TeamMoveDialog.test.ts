import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: () => () => {} } }));

import TeamMoveDialog from "./TeamMoveDialog.svelte";
import type { TeamSummary } from "./api";

afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const team = (id: string, name: string, over: Partial<TeamSummary> = {}) => ({ id, name, ...over }) as TeamSummary;
const own = team("own", "Band", { isStorage: true });
const teams = [own, team("cloud", "Band on Cloud", { hosted: true }), team("other", "Signed out", { hosted: true, signedOut: true })];
const estimate = { projects: [{ id: "p1", name: "Song", versions: 3, bytes: 2_000_000 }, { id: "p2", name: "Archive", versions: 9, bytes: 9_000_000 }],
  hosted: 11_000_000, storage: 10_000_000, people: 2, endpoint: "https://acc.r2.cloudflarestorage.com", bucket: "band", folder: "r3v", provider: "r2" };

describe("Moving a team to R3V Cloud", () => {
  it("says what moves, then starts copying with a read-only key", async () => {
    mocks.api.TeamMoveState.mockResolvedValue({ phase: "" });
    mocks.api.EstimateTeamMove.mockResolvedValue(estimate);
    render(TeamMoveDialog, { team: own, teams, reload: async () => {}, onclose: () => {} });
    await screen.findByText("Song");
    expect(screen.getByText(/Object Read only.*band/)).toBeTruthy();
    // Archives can stay behind.
    await fireEvent.click(screen.getAllByRole("checkbox")[1]);
    const select = screen.getByLabelText("Team on R3V Cloud") as HTMLSelectElement;
    expect([...select.options].map((o) => o.value)).toEqual(["", "cloud"]); // not one signed out
    await fireEvent.change(select, { target: { value: "cloud" } });
    await fireEvent.input(screen.getByLabelText("Access key"), { target: { value: "ro" } });
    await fireEvent.input(screen.getByLabelText("Secret key"), { target: { value: "secret" } });
    mocks.api.TeamMoveState.mockResolvedValue({ phase: "copying", bytes: 100, bytesDone: 40, items: 10, itemsDone: 4, copied: false, failed: 0, error: "" });
    await fireEvent.click(screen.getByRole("button", { name: "Start copying" }));
    await waitFor(() => expect(mocks.api.StartTeamMove).toHaveBeenCalledWith("own", "cloud", ["p1"], "ro", "secret", ""));
    expect(await screen.findByText(/keep working/)).toBeTruthy();
  });

  it("finishes once copied", async () => {
    mocks.api.TeamMoveState.mockResolvedValue({ phase: "copying", bytes: 100, bytesDone: 100, items: 10, itemsDone: 10, copied: true, failed: 0, error: "" });
    render(TeamMoveDialog, { team: own, teams, reload: async () => {}, onclose: () => {} });
    await fireEvent.click(await screen.findByRole("button", { name: "Finish the move" }));
    await waitFor(() => expect(mocks.api.FinishTeamMove).toHaveBeenCalledWith("own"));
  });
});
