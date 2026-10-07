import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import MoveProjectDialog from "./MoveProjectDialog.svelte";
import type { TeamSummary } from "./api";

afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const team = (id: string, name: string, over: Partial<TeamSummary> = {}) => ({ id, name, ...over }) as TeamSummary;
const props = (onmoved = vi.fn()) => ({
  root: "C:/Song", name: "Song", team: team("a", "Band"), progress: null, onmoved, onclose: () => {},
  teams: [team("a", "Band"), team("b", "Label"), team("c", "Cloud band", { hosted: true }), team("d", "Gone", { noAccess: true })],
});

describe("Moving a project to another team", () => {
  it("offers the other teams it can go to, and moves it", async () => {
    const onmoved = vi.fn();
    render(MoveProjectDialog, props(onmoved));
    const radios = screen.getAllByRole("radio");
    expect(radios.map((r) => (r as HTMLInputElement).value)).toEqual(["b", "c"]); // not its own, not one without access
    expect(screen.getByRole("button", { name: "Move" })).toHaveProperty("disabled", true);
    await fireEvent.click(radios[1]);
    expect(screen.getByText(/Band won't have it any more/)).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Move" }));
    await waitFor(() => expect(mocks.api.MoveProject).toHaveBeenCalledWith("C:/Song", "c", false));
    expect(onmoved).toHaveBeenCalledWith("c", false);
  });

  it("copies it instead, keeping it in its team", async () => {
    const onmoved = vi.fn();
    render(MoveProjectDialog, props(onmoved));
    await fireEvent.click(screen.getAllByRole("radio")[0]);
    await fireEvent.click(screen.getByRole("checkbox"));
    await fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    await waitFor(() => expect(mocks.api.MoveProject).toHaveBeenCalledWith("C:/Song", "b", true));
    expect(onmoved).toHaveBeenCalledWith("b", true);
  });

  it("says what went wrong, and stays", async () => {
    mocks.api.MoveProject.mockRejectedValue(new Error("share your versions first"));
    const onmoved = vi.fn();
    render(MoveProjectDialog, props(onmoved));
    await fireEvent.click(screen.getAllByRole("radio")[0]);
    await fireEvent.click(screen.getByRole("button", { name: "Move" }));
    expect((await screen.findByRole("alert")).textContent).toMatch(/share your versions first/);
    expect(onmoved).not.toHaveBeenCalled();
  });
});
