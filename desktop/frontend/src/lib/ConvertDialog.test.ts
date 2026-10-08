import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
import ConvertDialog from "./ConvertDialog.svelte";
const { api } = mocks;
afterEach(() => { cleanup(); for (const k of Object.keys(mocks.fns)) delete mocks.fns[k]; });

function setup() {
  api.ConvertFormats.mockResolvedValue([{ id: "mp3", name: "MP3", ext: "mp3", bitrates: [320, 256], rates: [44100, 48000], bits: [] }, { id: "wav", name: "WAV", ext: "wav", bitrates: [], rates: [44100, 48000], bits: [16, 24] }]);
  api.ConvertPlan.mockResolvedValue({ rate: 48000, channels: 2, bits: 24, bitrate: 0, notes: [] });
  api.ConvertTarget.mockResolvedValue("Samples/a.wav");
}

describe("ConvertDialog", () => {
  it("converts several samples the same way, one after another", async () => {
    setup();
    api.ConvertFile.mockImplementation(async (_root: string, f: string) => f.replace(".aif", ".wav"));
    const ondone = vi.fn();
    render(ConvertDialog, { root: "C:/Song", files: ["Samples/a.aif", "Samples/b.aif"], onclose: vi.fn(), ondone });
    expect(screen.getByText("Convert 2 samples")).toBeTruthy();
    await waitFor(() => expect(screen.getByRole("button", { name: "Convert" }).hasAttribute("disabled")).toBe(false));
    await fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(ondone).toHaveBeenCalledWith(["Samples/a.wav", "Samples/b.wav"]));
    expect(api.ConvertFile.mock.calls.map((c) => c[1])).toEqual(["Samples/a.aif", "Samples/b.aif"]);
  });

  it("says which didn't convert, and goes on with the rest", async () => {
    setup();
    api.ConvertFile.mockImplementation(async (_root: string, f: string) => {
      if (f.includes("a.aif")) throw new Error("can't read it");
      return f.replace(".aif", ".wav");
    });
    const ondone = vi.fn();
    render(ConvertDialog, { root: "C:/Song", files: ["Samples/a.aif", "Samples/b.aif"], onclose: vi.fn(), ondone });
    await waitFor(() => expect(screen.getByRole("button", { name: "Convert" }).hasAttribute("disabled")).toBe(false));
    await fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    expect(await screen.findByText("a.aif: can't read it")).toBeTruthy();
    expect(screen.getByText(/Converted 1 of 2/)).toBeTruthy();
    expect(api.ConvertFile).toHaveBeenCalledTimes(2);
    expect(ondone).not.toHaveBeenCalled();
  });
});
