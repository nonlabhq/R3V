import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("../api", async (orig) => ({ ...(await orig<typeof import("../api")>()), api: mocks.api }));
import MarkdownViewer from "./MarkdownViewer.svelte";
const { api } = mocks;
afterEach(() => { cleanup(); localStorage.clear(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

const file = { path: "README.md", status: "unchanged", size: 10, kind: "other" } as never;
const side = { path: "README.md", version: "", label: "Now" };

describe("MarkdownViewer", () => {
  it("shows notes as they read, opens their links in the browser, and as text on request", async () => {
    api.TextFile.mockResolvedValue({ text: true, tooBig: false, lines: ["# Mix notes", "See [the brief](https://example.com)."] });
    render(MarkdownViewer, { root: "C:/Song", file, a: side, b: null, compare: false, stamp: 0 });
    expect(await screen.findByRole("heading", { name: "Mix notes" })).toBeTruthy();
    await fireEvent.click(screen.getByText("the brief"));
    expect(api.OpenURL).toHaveBeenCalledWith("https://example.com");
    await fireEvent.click(screen.getByRole("button", { name: "Text" }));
    await waitFor(() => expect(screen.queryByRole("heading", { name: "Mix notes" })).toBeNull());
    expect(localStorage.getItem("r3v.mdText")).toBe("1");
  });
});
