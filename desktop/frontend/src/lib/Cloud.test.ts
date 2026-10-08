import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";

// The R3V-Cloud screens with the Go bindings mocked: what they call, with
// what, and what they show to whom.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  const api = new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) });
  return { fns, api, toast: vi.fn() };
});
const { api, toast } = mocks;
vi.mock("./api", async (orig) => ({ ...(await orig<typeof import("./api")>()), api: mocks.api }));
vi.mock("./notify.svelte", () => ({ toast: mocks.toast }));
vi.mock("@wailsio/runtime", async (orig) => ({ ...(await orig<typeof import("@wailsio/runtime")>()),
  Events: { On: () => () => {} } }));

import CloudJoin from "./CloudJoin.svelte";
import CloudPeople from "./CloudPeople.svelte";
import JoinOrCreate from "./JoinOrCreate.svelte";
import TeamSettings from "./TeamSettings.svelte";
import type { TeamSummary } from "./api";

beforeEach(() => {
  for (const k of Object.keys(mocks.fns)) delete mocks.fns[k];
  toast.mockClear();
});
afterEach(() => cleanup());

const signedOut = { available: true, service: "https://api.r3v.so", signedIn: false, email: "" };
const signedIn = { ...signedOut, signedIn: true, email: "yi@example.test" };
const band: TeamSummary = { id: "t1", name: "Band", address: "r3v-cloud+https://api.r3v.so/v1/teams/t1", isStorage: false, memberId: "m1",
  memberName: "Yi", keysUnreadable: false, shareSetup: false, canShareSetup: false, preupload: false, askShareSetup: false,
  backupFailing: false, hosted: true, signedOut: false, noAccess: false, looks: false, moving: false, movedTo: "", movedToTeam: "" };

describe("CloudJoin", () => {
  it("signs in through the browser and goes to the account's team", async () => {
    api.CloudStatus.mockResolvedValue(signedOut);
    let finish!: (s: unknown) => void;
    api.CloudSignIn.mockReturnValue(new Promise((r) => (finish = r)));
    api.Overview.mockResolvedValue({ teams: [band], currentTeam: "t1" });
    const onconnected = vi.fn();
    render(CloudJoin, { onconnected });
    await fireEvent.click(await screen.findByRole("button", { name: "Sign in with your browser" }));
    expect(api.CloudSignIn).toHaveBeenCalled();
    await screen.findByText("Finish signing in in your browser…");
    finish(signedIn);
    await waitFor(() => expect(onconnected).toHaveBeenCalledWith(band));
  });

  it("can stop waiting for the browser", async () => {
    api.CloudStatus.mockResolvedValue(signedOut);
    api.CloudSignIn.mockReturnValue(new Promise(() => {}));
    render(CloudJoin, { onconnected: vi.fn() });
    await fireEvent.click(await screen.findByRole("button", { name: "Sign in with your browser" }));
    await fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
    expect(api.CloudCancelSignIn).toHaveBeenCalled();
    await screen.findByRole("button", { name: "Sign in with your browser" });
  });

  it("creates a team and joins one with a link, signed in", async () => {
    api.CloudStatus.mockResolvedValue(signedIn);
    api.CloudCreateTeam.mockResolvedValue(band);
    api.CloudInvitationInfo.mockRejectedValue(new Error("This invitation was used, withdrawn or has expired: ask for a new one."));
    const onconnected = vi.fn();
    render(CloudJoin, { onconnected });
    await screen.findByText(/Signed in as yi@example.test/);
    await fireEvent.input(screen.getByLabelText("Create a team"), { target: { value: "Band" } });
    await fireEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(onconnected).toHaveBeenCalledWith(band));
    expect(api.CloudCreateTeam).toHaveBeenCalledWith("Band");

    await fireEvent.input(screen.getByLabelText("Join with an invitation link"), { target: { value: "https://r3v.so/invite/x" } });
    await fireEvent.click(screen.getByRole("button", { name: "Join" }));
    await screen.findByText(/ask for a new one/);
    expect(api.CloudInvitationInfo).toHaveBeenCalledWith("https://r3v.so/invite/x");
    expect(api.CloudJoinAs).not.toHaveBeenCalled();
  });

  it("asks who you were in a team that moved in, then joins as them", async () => {
    api.CloudStatus.mockResolvedValue(signedIn);
    api.CloudInvitationInfo.mockResolvedValue({ team: "Band", people: [
      { id: "a1", name: "Mia", claimed: false }, { id: "b2", name: "Robin", claimed: true }] });
    api.CloudJoinAs.mockResolvedValue(band);
    const onconnected = vi.fn();
    render(CloudJoin, { onconnected });
    await screen.findByText(/Signed in as/);
    await fireEvent.input(screen.getByLabelText("Join with an invitation link"), { target: { value: "https://r3v.so/invite/x" } });
    await fireEvent.click(screen.getByRole("button", { name: "Join" }));
    await screen.findByText("Who were you in Band?");
    expect(screen.queryByLabelText("Robin")).toBeNull(); // claimed already
    await fireEvent.click(screen.getByLabelText("Mia"));
    await fireEvent.click(screen.getByRole("button", { name: "Join" }));
    await waitFor(() => expect(api.CloudJoinAs).toHaveBeenCalledWith("https://r3v.so/invite/x", "a1"));
    expect(onconnected).toHaveBeenCalledWith(band);
  });

  it("joins at once where nobody is to be claimed", async () => {
    api.CloudStatus.mockResolvedValue(signedIn);
    api.CloudInvitationInfo.mockResolvedValue({ team: "Band", people: [] });
    api.CloudJoinAs.mockResolvedValue(band);
    render(CloudJoin, { onconnected: vi.fn() });
    await screen.findByText(/Signed in as/);
    await fireEvent.input(screen.getByLabelText("Join with an invitation link"), { target: { value: "https://r3v.so/invite/y" } });
    await fireEvent.click(screen.getByRole("button", { name: "Join" }));
    await waitFor(() => expect(api.CloudJoinAs).toHaveBeenCalledWith("https://r3v.so/invite/y", ""));
  });
});

describe("JoinOrCreate", () => {
  it("offers R3V-Cloud only where it's available", async () => {
    api.CloudStatus.mockResolvedValue({ ...signedOut, available: false });
    render(JoinOrCreate, { onconnected: vi.fn() });
    await waitFor(() => expect(api.CloudStatus).toHaveBeenCalled());
    expect(screen.queryByRole("tab", { name: "R3V-Cloud" })).toBeNull();
    cleanup();
    api.CloudStatus.mockResolvedValue(signedOut);
    render(JoinOrCreate, { onconnected: vi.fn() });
    await fireEvent.click(await screen.findByRole("tab", { name: "R3V-Cloud" }));
    await screen.findByRole("button", { name: "Sign in with your browser" });
  });
});

const people = (myRole: string) => ({
  myRole, myUserId: "u1",
  members: [
    { userId: "u1", memberId: "m1", name: "Yi", email: "yi@example.test", role: myRole },
    { userId: "u2", memberId: "m2", name: "Alex", email: "alex@example.test", role: "owner" },
    { userId: "u3", memberId: "m3", name: "", email: "sam@example.test", role: "collaborator", projects: { p1: "write" } },
  ],
  invitations: myRole === "owner" || myRole === "admin" ? [{ id: "i1", email: "kim@example.test", role: "member", expiresAt: 0 }] : [],
  projects: [{ id: "p1", name: "Song", access: "write" }, { id: "p2", name: "Album", access: "write" }],
});

describe("CloudPeople", () => {
  it("lets an owner change roles, project access and invitations", async () => {
    api.CloudPeople.mockResolvedValue(people("owner"));
    render(CloudPeople, { team: band });
    await screen.findByText("Alex");
    const sam = screen.getByText("sam@example.test").closest("li")!;
    await fireEvent.change(within(sam).getByLabelText("Access to Album"), { target: { value: "read" } });
    await waitFor(() => expect(api.CloudSetAccess).toHaveBeenCalledWith("t1", "p2", "u3", "read"));
    await fireEvent.change(within(sam).getAllByLabelText("Role")[0], { target: { value: "member" } });
    await waitFor(() => expect(api.CloudSetRole).toHaveBeenCalledWith("t1", "u3", "member"));
    // Yourself: no role to pick, nothing to remove.
    const me = screen.getByText("(you)").closest("li")!;
    expect(within(me).queryByRole("combobox")).toBeNull();
    // Pending invitation, withdrawn.
    await fireEvent.click(screen.getByRole("button", { name: "Withdraw" }));
    await waitFor(() => expect(api.CloudWithdraw).toHaveBeenCalledWith("t1", "i1"));
  });

  it("invites a collaborator to the chosen projects only", async () => {
    api.CloudPeople.mockResolvedValue(people("admin"));
    render(CloudPeople, { team: band });
    await screen.findByText("Invite someone");
    const form = screen.getByText("Send invitation").closest("form")!;
    await fireEvent.input(within(form).getByLabelText("Their email"), { target: { value: "lee@example.test" } });
    await fireEvent.change(within(form).getByLabelText("Role"), { target: { value: "collaborator" } });
    await fireEvent.change(within(form).getByLabelText("Access to Song"), { target: { value: "write" } });
    await fireEvent.click(within(form).getByRole("button", { name: "Send invitation" }));
    await waitFor(() => expect(api.CloudInvite).toHaveBeenCalledWith("t1", "lee@example.test", "collaborator", { p1: "write" }));
    expect(toast).toHaveBeenCalledWith("Invitation sent to lee@example.test", "ok");
    // An admin doesn't touch owners or make them.
    const alex = screen.getByText("Alex").closest("li")!;
    expect(within(alex).queryByRole("combobox")).toBeNull();
    expect(within(form).queryByRole("option", { name: "Owner" })).toBeNull();
  });

  it("only shows a member who is in it", async () => {
    api.CloudPeople.mockResolvedValue(people("member"));
    render(CloudPeople, { team: band });
    await screen.findByText("Alex");
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByText("Invite someone")).toBeNull();
    expect(screen.queryByRole("button", { name: "Remove" })).toBeNull();
    expect(screen.getByText("Song · Can edit")).toBeTruthy(); // Sam's project, read only
  });
});

describe("TeamSettings of a hosted team", () => {
  const open = async (team: TeamSummary) => {
    api.HistoryDownloadSize.mockResolvedValue(0);
    render(TeamSettings, { team, reload: vi.fn(async () => {}), onclose: vi.fn() });
  };

  it("leaves it, in those words", async () => {
    await open(band);
    await fireEvent.click(await screen.findByRole("button", { name: "Leave the team…" }));
    await screen.findByText("Leave Band?");
    screen.getByText(/someone has to invite you again/);
    screen.getByText(/no longer shared with the team/);
    await fireEvent.click(screen.getByRole("button", { name: "Leave the team" }));
    await waitFor(() => expect(api.RemoveTeam).toHaveBeenCalledWith("t1", true, false));
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Band is no longer on this computer. Its projects are under Local now.", "info", 7000));
  });

  it("removes one the account is no longer in", async () => {
    await open({ ...band, noAccess: true });
    await fireEvent.click(await screen.findByRole("button", { name: "Remove the team…" }));
    await screen.findByText("Remove Band?");
    screen.getByText(/You're no longer in this team/);
    expect(screen.queryByText(/someone has to invite you again/)).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.RemoveTeam).toHaveBeenCalledWith("t1", true, false));
  });
});
