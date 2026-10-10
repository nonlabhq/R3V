import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import KindDialog from "./KindDialog.svelte";

afterEach(() => cleanup());

describe("KindDialog", () => {
  it("asks how to keep a folder R3V doesn't recognise, code first where there is code", async () => {
    const onpick = vi.fn();
    render(KindDialog, { folder: "D:\\Projects\\Plans", code: true, onpick, onclose: () => {} });
    expect(screen.getByText("How should R3V keep “Plans”?")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Add project" }));
    expect(onpick).toHaveBeenLastCalledWith("code");
    await fireEvent.click(screen.getByLabelText(/Files as they are/));
    await fireEvent.click(screen.getByRole("button", { name: "Add project" }));
    expect(onpick).toHaveBeenLastCalledWith("none");
  });

  it("offers files as they are only, where the build has no code kind", () => {
    render(KindDialog, { folder: "D:\\Projects\\Plans", code: false, onpick: () => {}, onclose: () => {} });
    expect(screen.queryByText("Code and documents")).toBeNull();
    expect(screen.getByText("Files as they are")).toBeTruthy();
  });
});
