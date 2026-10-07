import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import Modal from "./Modal.svelte";

afterEach(() => cleanup());
const body = createRawSnippet(() => ({ render: () => "<p>body</p>" }));

// Esc closes the dialog on top only, also one a click beside doesn't
// close; not one whose step can't be left.
it("closes the dialog on top with Esc", async () => {
  const under = vi.fn(), over = vi.fn(), busy = vi.fn();
  render(Modal, { title: "Under", onclose: under, children: body });
  render(Modal, { title: "Over", onclose: over, backdropCloses: false, children: body });
  await fireEvent.keyDown(window, { key: "Escape" });
  expect(over).toHaveBeenCalledTimes(1);
  expect(under).not.toHaveBeenCalled();
  cleanup();
  render(Modal, { title: "Busy", onclose: busy, escCloses: false, children: body });
  await fireEvent.keyDown(window, { key: "Escape" });
  expect(busy).not.toHaveBeenCalled();
});
