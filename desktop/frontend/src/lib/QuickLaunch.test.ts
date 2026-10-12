import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import QuickLaunch from "./QuickLaunch.svelte";

afterEach(() => cleanup());

describe("The quick launcher", () => {
  it("opens what was typed even when Enter comes before the projects are read", async () => {
    let done: (x: unknown) => void = () => {};
    mocks.api.AllProjects.mockReturnValue(new Promise((r) => (done = r)));
    const onpick = vi.fn();
    render(QuickLaunch, { teams: [{ id: "t1", name: "Demo Band" }] as never, current: "t1", recent: [], onpick, onclose: vi.fn() });
    const search = screen.getByRole("combobox");
    await fireEvent.input(search, { target: { value: "moon" } });
    await fireEvent.keyDown(search, { key: "Enter" });
    expect(onpick).not.toHaveBeenCalled();
    done([{ team: "t1", projects: [{ id: "p1", name: "Moonrise", root: "C:\\Moonrise", status: "downloaded" }] }]);
    await waitFor(() => expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ team: "t1", project: expect.objectContaining({ name: "Moonrise" }) })));
  });
});
