import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";

// The bindings mocked: each resolves to null unless a test says otherwise.
const mocks = vi.hoisted(() => {
  const fns: Record<string, ReturnType<typeof vi.fn>> = {};
  return { api: new Proxy(fns, { get: (o, k: string) => (o[k] ??= vi.fn(async () => null)) }) };
});
vi.mock("../api", async (orig) => ({ ...(await orig<typeof import("../api")>()), api: mocks.api }));
import RulesViewer from "./RulesViewer.svelte";
import { setRulesPane } from "./settings.svelte";
import type { ViewerProps } from "./types";

beforeEach(() => {
  setRulesPane("rules");
  for (const f of Object.values(mocks.api)) f.mockReset();
  mocks.api.TextFile.mockResolvedValue({ text: true, tooBig: false, lines: ["rules:", "  - ignore: \"Renders/\""], truncated: false });
});
afterEach(() => cleanup());

const file = { path: ".r3v.yaml", status: "modified", size: 1, kind: "other", live: "", from: "", edited: false,
  preview: false, video: false, model: false };
const side = (version: string) => ({ path: ".r3v.yaml", version, label: version ? `“${version}”` : "Now" });
const rules = (over: Record<string, unknown> = {}) => ({ exists: true, error: "", requires: "", gitignore: false,
  presets: [{ folder: "", preset: "ableton", found: true }], rules: [{ kind: "ignore", pattern: "Renders/" }],
  options: [{ name: "ableton", leftOut: ["Backup/"] }], ...over });
const props = (o: Record<string, unknown>) =>
  ({ root: "C:/Song", file, a: side("v1"), b: null, compare: false, stamp: 0, ...o }) as unknown as ViewerProps;

// A past version: the rules in plain words, grouped as the Rules window
// groups them; not changed from here.
it("shows a version's rules in plain words", async () => {
  mocks.api.RulesAt.mockResolvedValue(rules({ requires: "0.1.0", gitignore: true }));
  render(RulesViewer, { props: props({ editable: true }) });
  expect(await screen.findByText("Tools in this project")).toBeTruthy();
  expect(mocks.api.RulesAt).toHaveBeenCalledWith("C:/Song", ".r3v.yaml", "v1");
  expect(screen.getByText("Ableton Live")).toBeTruthy();
  expect(screen.getByText("· leaves out Backup")).toBeTruthy();
  expect(screen.getByText("Your rules")).toBeTruthy();
  expect(screen.getByText("Leave out")).toBeTruthy();
  expect(screen.getByText("Renders/")).toBeTruthy();
  expect(screen.getByText("Other settings")).toBeTruthy();
  expect(screen.getByText("Needs R3V 0.1.0 or newer")).toBeTruthy();
  expect(screen.getByText("Follows the project's .gitignore files")).toBeTruthy();
  expect(mocks.api.ProjectRules).not.toHaveBeenCalled(); // a past version isn't edited
});

// The file now, in the Files tab: the Rules window's editing, right here.
it("offers the rules' editing for the file now", async () => {
  mocks.api.RulesAt.mockResolvedValue(rules());
  mocks.api.ProjectRules.mockResolvedValue({ presets: [], rules: [{ kind: "ignore", pattern: "Renders/" }], suggestions: [], options: [], error: "" });
  mocks.api.RulesFolder.mockResolvedValue([]);
  mocks.api.RemoveRule.mockResolvedValue(undefined);
  render(RulesViewer, { props: props({ a: side(""), editable: true }) });
  const remove = await screen.findByTitle("Remove this rule");
  expect(mocks.api.ProjectRules).toHaveBeenCalledWith("C:/Song");
  await fireEvent.click(remove);
  await waitFor(() => expect(mocks.api.RemoveRule).toHaveBeenCalledWith("C:/Song", 0));
  // Elsewhere (comparing, or not the Files tab): only shown.
  cleanup();
  mocks.api.ProjectRules.mockClear();
  render(RulesViewer, { props: props({ a: side("") }) });
  await screen.findByText("Tools in this project");
  expect(mocks.api.ProjectRules).not.toHaveBeenCalled();
});

// Comparing: what changed, rule by rule, coloured by kind.
it("shows the changes rule by rule", async () => {
  mocks.api.RulesAt.mockImplementation(async (_root: string, _path: string, v: string) => v === "v2"
    ? rules({ rules: [{ kind: "ignore", pattern: "*.wav" }], presets: [{ folder: "", preset: "unity", found: false }] })
    : rules());
  render(RulesViewer, { props: props({ a: side("v2"), b: side("v1"), compare: true }) });
  const added = await screen.findByText("Left out: *.wav");
  expect(added.classList.contains("add")).toBe(true);
  expect(screen.getByText("No longer left out: Renders/").classList.contains("del")).toBe(true);
  expect(screen.getByText("The project folder: preset changed from Ableton Live to Unity").classList.contains("mod")).toBe(true);
});

// A file R3V can't read: said plainly, and its text shown.
it("shows the text of rules it can't read", async () => {
  mocks.api.RulesAt.mockResolvedValue(rules({ error: ".r3v.yaml: line 2: field ignor not found", presets: [], rules: [] }));
  render(RulesViewer, { props: props({}) });
  expect(await screen.findByText(/R3V can't read these rules \(“v1”\): .*ignor/)).toBeTruthy();
  await waitFor(() => expect(mocks.api.TextFile).toHaveBeenCalledWith("C:/Song", ".r3v.yaml", "v1"));
});

// The Text switch: the file's text, remembered for the next rules file.
it("switches to the text and back, remembered", async () => {
  mocks.api.RulesAt.mockResolvedValue(rules());
  render(RulesViewer, { props: props({}) });
  await screen.findByText("Tools in this project");
  await fireEvent.click(screen.getByText("Text"));
  await waitFor(() => expect(mocks.api.TextFile).toHaveBeenCalled());
  expect(screen.queryByText("Tools in this project")).toBeNull();
  expect(localStorage.getItem("r3v.rulesPane")).toBe("text");
  cleanup();
  render(RulesViewer, { props: props({}) });
  expect(screen.queryByText("Tools in this project")).toBeNull(); // still text
  await fireEvent.click(screen.getByText("Rules"));
  expect(await screen.findByText("Tools in this project")).toBeTruthy();
  expect(localStorage.getItem("r3v.rulesPane")).toBe("rules");
});
