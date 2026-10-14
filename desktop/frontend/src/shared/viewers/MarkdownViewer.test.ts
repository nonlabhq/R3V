import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import MarkdownViewer from "./MarkdownViewer.svelte";
import type { FileSource } from "./source";
import { setMdText } from "./settings.svelte";

afterEach(() => { cleanup(); setMdText(false); localStorage.clear(); });

const file = { path: "README.md", size: 10 };
const side = { path: "README.md", version: "", label: "Now" };

describe("MarkdownViewer", () => {
  it("shows notes as they read, hands their links over, and as text on request", async () => {
    const source: FileSource = {
      open: vi.fn(async () => { throw new Error("not read as bytes here"); }),
      text: vi.fn(async () => ({ text: true, tooBig: false, truncated: false, lines: ["# Mix notes", "See [the brief](https://example.com)."] })),
    };
    const onlink = vi.fn();
    render(MarkdownViewer, { source, file, a: side, b: null, compare: false, onlink });
    expect(await screen.findByRole("heading", { name: "Mix notes" })).toBeTruthy();
    await fireEvent.click(screen.getByText("the brief"));
    expect(onlink).toHaveBeenCalledWith("https://example.com");
    await fireEvent.click(screen.getByRole("button", { name: "Text" }));
    await waitFor(() => expect(screen.queryByRole("heading", { name: "Mix notes" })).toBeNull());
    expect(localStorage.getItem("r3v.mdText")).toBe("1");
  });

  it("reads a note and its pictures through open() alone (the web's source), and closes them", async () => {
    const closed: string[] = [];
    const files: Record<string, string> = { "docs/notes.md": "# Devlog\n![shot](media/a.png)\n" };
    const source: FileSource = {
      open: vi.fn(async (path: string) => {
        const url = path.endsWith(".png") ? `blob:test/${path}` : `data:text/plain;base64,${btoa(files[path])}`;
        return { url, size: 0, type: "", close: () => closed.push(path) };
      }),
    };
    const { unmount } = render(MarkdownViewer, { source, file: { path: "docs/notes.md" },
      a: { path: "docs/notes.md", version: "v1", label: "v1" }, b: null, compare: false });
    expect(await screen.findByRole("heading", { name: "Devlog" })).toBeTruthy();
    await waitFor(() => expect(screen.getByAltText("shot").getAttribute("src")).toBe("blob:test/docs/media/a.png"));
    expect(source.open).toHaveBeenCalledWith("docs/media/a.png", "v1");
    unmount();
    expect(closed).toContain("docs/media/a.png");
  });
});
