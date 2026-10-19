import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

// The look (Flat or Cards): data-look on <html>, kept on this computer,
// Flat for those who never chose; picked in R3V's settings.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return { fns, api: new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) }) };
});
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));

import { look, setLook, startLook } from "./look.svelte";
import AppSettings from "./AppSettings.svelte";

const attr = () => document.documentElement.dataset.look;

beforeEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.look;
});
afterEach(() => cleanup());

describe("look", () => {
  it("is Flat for someone who never chose", () => {
    startLook();
    expect(look()).toBe("flat");
    expect(attr()).toBe("flat");
  });

  it("keeps the look picked, and starts with it", () => {
    setLook("cards");
    expect(attr()).toBe("cards");
    expect(localStorage.getItem("r3v.look")).toBe("cards");
    document.documentElement.dataset.look = "";
    startLook(); // (the next start)
    expect(look()).toBe("cards");
    expect(attr()).toBe("cards");
  });

  it("starts Flat when what was kept isn't a look", () => {
    localStorage.setItem("r3v.look", "glass");
    startLook();
    expect(attr()).toBe("flat");
  });

  it("is picked in R3V's settings, under Appearance", async () => {
    startLook();
    render(AppSettings, {
      version: "0.2.1", edition: "", autostart: false, autoUpdate: true, downloadDir: "", update: null, checking: false,
      onautostart: () => {}, onautoupdate: () => {}, oncheck: () => {}, ondownloaddir: () => {}, onupdate: () => {}, onclose: () => {},
    });
    expect(screen.getByText("Appearance")).toBeTruthy();
    const flat = screen.getByRole("radio", { name: "Flat" }), cards = screen.getByRole("radio", { name: "Cards" });
    expect(flat.getAttribute("aria-checked")).toBe("true");
    await fireEvent.click(cards);
    expect(cards.getAttribute("aria-checked")).toBe("true");
    expect(flat.getAttribute("aria-checked")).toBe("false");
    expect(attr()).toBe("cards");
    expect(localStorage.getItem("r3v.look")).toBe("cards");
    await fireEvent.click(flat);
    expect(attr()).toBe("flat");
    expect(localStorage.getItem("r3v.look")).toBe("flat");
  });
});
