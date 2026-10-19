import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";

import DeletedProjectDialog from "./DeletedProjectDialog.svelte";
import { DeletedInfo } from "../../bindings/github.com/nonlabhq/r3v/desktop/models";

// A share stopped: the team deleted the project. What the dialog offers,
// by what's known: never anything without a click.
const info = (d: Partial<DeletedInfo>) => new DeletedInfo({ project: "Game", team: "Studio", ...d });

describe("DeletedProjectDialog", () => {
  afterEach(() => { cleanup(); vi.clearAllMocks(); });

  it("offers to point the copy at the team it moved to", async () => {
    const onfollow = vi.fn(), onrestore = vi.fn();
    render(DeletedProjectDialog, { d: info({ movedTo: "Studio Two", movedToTeam: "t2", time: "2026-10-17T00:00:00Z", by: "Mia", canRestore: true }),
      onfollow, onrestore, onclose: vi.fn() });
    expect(document.body.textContent).toContain("It moved to Studio Two; this copy still points at Studio.");
    expect(document.body.textContent).toContain("by Mia");
    await fireEvent.click(screen.getByRole("button", { name: "Point this copy at Studio Two" }));
    expect(onfollow).toHaveBeenCalledOnce();
    expect(onrestore).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Bring it back to Studio" })).toBeTruthy();
  });

  it("tells a member to ask an admin, with nothing to bring it back", () => {
    render(DeletedProjectDialog, { d: info({ canRestore: false }), onfollow: vi.fn(), onrestore: vi.fn(), onclose: vi.fn() });
    expect(document.body.textContent).toContain("Only the team's owners and admins can bring it back to Studio: ask one.");
    expect(screen.queryByRole("button", { name: "Bring it back to Studio" })).toBeNull();
    expect(screen.queryByRole("button", { name: /Point this copy/ })).toBeNull();
  });

  it("brings it back for an admin when asked", async () => {
    const onrestore = vi.fn();
    render(DeletedProjectDialog, { d: info({ canRestore: true }), onfollow: vi.fn(), onrestore, onclose: vi.fn() });
    await fireEvent.click(screen.getByRole("button", { name: "Bring it back to Studio" }));
    expect(onrestore).toHaveBeenCalledOnce();
  });
});
