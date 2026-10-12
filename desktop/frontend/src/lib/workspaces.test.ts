import { afterEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  let done!: (l: unknown[]) => void;
  const api = { AgentWorkspaces: vi.fn(() => new Promise((r) => (done = r))) };
  return { api, finish: (l: unknown[]) => done(l) };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import { cachedWorkspaces, forgetWorkspaces, readingWorkspaces, refreshWorkspaces } from "./workspaces.svelte";

afterEach(() => forgetWorkspaces());

describe("The workspaces kept in the background", () => {
  it("say they're being read, and are kept for the next start", async () => {
    const going = refreshWorkspaces("C:/Song");
    expect(readingWorkspaces("C:/Song")).toBe(true);
    expect(cachedWorkspaces("C:/Song")).toBeNull();
    mocks.finish([{ folder: "C:/Song (lamp)", branch: "lamp" }]);
    await going;
    expect(readingWorkspaces("C:/Song")).toBe(false);
    expect(cachedWorkspaces("C:/Song")?.[0].branch).toBe("lamp");
    expect(JSON.parse(localStorage.getItem("r3v.workspaces")!)["C:/Song"][0].folder).toBe("C:/Song (lamp)");
  });
});
