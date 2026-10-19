import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

// The app's main paths on the demo team (desktop/cmd/r3v-demo), in order:
// the steps change the team (an update, a commit). Night Drive starts with
// a teammate's new version (Mia's) and two changes not committed yet.

test.describe.configure({ mode: "serial" });

let page: Page;
const errors: string[] = [];

test.beforeAll(async ({ browser }) => {
  page = await browser.newPage();
  page.on("pageerror", (e) => errors.push(`page error: ${e.message}`));
  page.on("console", (m) => { if (m.type() === "error") errors.push(`console: ${m.text()}`); });
  await page.goto("/");
});
test.afterAll(async () => { await page.close(); });
// Nothing broke along the way.
test.afterEach(() => { expect(errors, "errors in the page").toEqual([]); });

test("opens on the team's projects", async () => {
  for (const name of ["Night Drive", "Moonrise", "Fresh Idea"]) {
    await expect(page.getByRole("button", { name: new RegExp(`^${name}`) }).first()).toBeVisible();
  }
  await page.getByRole("button", { name: /^Night Drive/ }).first().click();
  await expect(page.getByRole("heading", { name: "Night Drive" })).toBeVisible();
});

// The demo runs apart from this computer's own R3V: no R3V Cloud account
// signed in, and no team but the demo's (its storage is the demo's fake, on
// this computer). The app has started by now, so a sync it ran would show.
test("sees only the demo's team, and no R3V Cloud account", async () => {
  const store = JSON.parse(readFileSync(join(tmpdir(), "r3v-smoke", "you", "config", "teams.json"), "utf8"));
  const foreign = (store.teams ?? []).filter((t: { remote?: { url?: string } }) => !t.remote?.url?.startsWith("s3+http://127.0.0.1:"));
  expect(foreign.length, "teams not made by the demo").toBe(0);

  await page.locator("button.who").first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(/Signed in as/)).toHaveCount(0);
  if (await dialog.getByRole("heading", { name: "R3V Cloud" }).count()) {
    await expect(dialog.getByText("Not signed in.")).toBeVisible();
  }
  await dialog.getByRole("button", { name: "Done" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("shows the versions in the graph, and your changes", async () => {
  const versions = page.getByRole("listbox", { name: "Versions" }).getByRole("option");
  await expect(versions.first()).toBeVisible();
  expect(await versions.count()).toBeGreaterThanOrEqual(4);
  await expect(page.getByText("Your changes", { exact: true })).toBeVisible();
  await expect(page.getByText("Lyrics.txt").first()).toBeVisible();
});

test("shows a version picked in the graph", async () => {
  await page.getByRole("option", { name: /^Bass line, drums tightened/ }).click();
  await expect(page.getByRole("heading", { name: "Bass line, drums tightened" })).toBeVisible();
  await expect(page.getByText(/files? changed/)).toBeVisible();
});

test("previews a Live set in the Files tab", async () => {
  await page.getByRole("button", { name: "Files", exact: true }).click();
  await page.getByRole("row", { name: /Night Drive\.als/ }).click();
  await expect(page.getByText("MIDI Tracks").first()).toBeVisible({ timeout: 15_000 });
  // The project's rules file: its rules in plain words, editable here.
  await page.getByRole("row", { name: /\.r3v\.yaml/ }).click();
  await expect(page.getByText("Project types").first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText("Files and folders").first()).toBeVisible();
  await page.getByRole("button", { name: /^Versions/ }).click();
});

test("gets the teammate's new version, keeping your changes", async () => {
  await page.getByRole("button", { name: "Get updates" }).first().click();
  await expect(page.getByText(/You're up to date/).first()).toBeVisible({ timeout: 30_000 });
  // still yours, uncommitted
  await page.getByRole("option", { name: "Your changes" }).click();
  await expect(page.getByText("Lyrics.txt").first()).toBeVisible();
});

test("commits and shares your changes", async () => {
  await page.getByPlaceholder(/What did you change/).fill("Smoke test: lyrics and a vocal take");
  await page.keyboard.press("Control+Enter");
  await expect(page.getByText("Version committed and shared with the team").first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("option", { name: /^Smoke test: lyrics and a vocal take/ })).toBeVisible();
  await expect(page.getByRole("option", { name: "Your changes" })).toHaveCount(0);
});

// Alex's branch and main both changed notes.txt (Mia's line, Alex's line).
test("merges a branch, asking about what both sides changed", async () => {
  await page.getByRole("button", { name: /Current branch/ }).click();
  await page.getByRole("menu").getByRole("button", { name: "half-time-chorus" }).last().click();
  await page.getByRole("button", { name: "Merge and share" }).click();
  const decide = page.getByRole("dialog", { name: "You and Alex both changed notes.txt" });
  await expect(decide).toBeVisible({ timeout: 30_000 });
  await expect(decide.getByText("1 of 1 decision")).toBeVisible();
  await decide.getByRole("radio", { name: /Keep yours/ }).click();
  await decide.getByRole("button", { name: "Merge", exact: true }).click();
  await expect(page.getByText(/Merged half-time-chorus into main and shared it/).first()).toBeVisible({ timeout: 30_000 });
});

// Nightly: a workspace for an agent, made beside the project, then removed.
test("makes a workspace in the Workspaces tab, and removes it", async () => {
  // (the merge before offers archiving its branch: not now)
  const archive = page.getByRole("dialog", { name: /now that it's merged/ });
  if (await archive.count()) await archive.getByRole("button", { name: "Close" }).click();
  await page.getByRole("button", { name: /^Workspaces/ }).click();
  await expect(page.getByRole("heading", { name: "Work side by side" })).toBeVisible();
  await page.getByRole("button", { name: /New workspace/ }).click();
  const dialog = page.getByRole("dialog", { name: "New workspace" });
  await dialog.getByLabel("Branch name").fill("smoke-agent");
  await expect(dialog.getByLabel("Folder")).toHaveValue(/Night Drive Project\.workspaces[\\/]smoke-agent$/);
  await dialog.getByRole("button", { name: "Create" }).click();
  await expect(page.getByText(/The workspace “smoke-agent” is ready/).first()).toBeVisible({ timeout: 30_000 });
  const item = page.locator(".agents li").filter({ hasText: "smoke-agent" });
  await expect(item.getByText("Nothing to merge")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("button", { name: /^Workspaces/ }).locator(".count")).toHaveText("1");
  await item.getByRole("button", { name: "Remove…" }).click();
  await page.getByRole("dialog", { name: /Remove the workspace/ }).getByRole("button", { name: "Remove", exact: true }).click();
  await expect(page.getByText(/Removed the workspace “smoke-agent”/).first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("heading", { name: "Work side by side" })).toBeVisible();
  await page.getByRole("button", { name: /^Versions/ }).click();
});

test("lists the shortcuts, and Esc closes the list", async () => {
  await page.locator("body").click({ position: { x: 5, y: 300 } });
  await page.keyboard.press("Control+/");
  await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toHaveCount(0);
});

test("offers the project's sets and other programs for its folder in the Open in menu", async () => {
  await page.getByRole("button", { name: "More ways to open the project" }).click();
  const menu = page.getByRole("menu", { name: "Open in…" });
  await expect(menu.getByText("Project files")).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Explorer" })).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Choose a program…" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
});

test("offers the same programs for its folder in the project's right-click menu", async () => {
  await page.getByRole("button", { name: /^Night Drive/ }).first().click({ button: "right" });
  await page.locator(".row-menu").getByRole("menuitem", { name: /Open the folder in/ }).hover();
  const list = page.getByRole("menu", { name: "Open the folder in" });
  await expect(list.getByRole("menuitem", { name: "Explorer" })).toHaveCount(0); // ("Open folder" is in the menu)
  await expect(list.getByRole("menuitem", { name: "Choose a program…" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator(".row-menu")).toHaveCount(0);
});

test("opens the project's settings", async () => {
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  const tabs = page.getByRole("navigation", { name: "Project settings" });
  await tabs.getByRole("button", { name: "Rules" }).click();
  await expect(page.getByRole("button", { name: "Rules…" })).toBeVisible();
  await tabs.getByRole("button", { name: "Branches" }).click();
  await expect(page.getByText(/^\d+ versions? · /).first()).toBeVisible(); // (each branch's count)
  await tabs.getByRole("button", { name: "General" }).click(); // (remembered: the next visit opens there)
});

test("opens the team's home, and a project from the quick launcher", async () => {
  await page.locator(".team-menu .switch").click();
  await expect(page.getByRole("heading", { name: "Demo Band" })).toBeVisible();
  await page.getByRole("button", { name: "Team settings" }).click();
  await expect(page.getByRole("textbox", { name: "Team name" })).toBeVisible();
  await page.keyboard.press("Control+t");
  const search = page.getByRole("combobox", { name: "Open a project or team…" });
  await expect(search).toBeFocused();
  await search.fill("moon");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { name: "Moonrise" })).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("a project card in the team's home has the project's menu behind its ⋯", async () => {
  await page.locator(".team-menu .switch").click();
  const more = page.locator(".slot").filter({ hasText: "Night Drive" }).getByRole("button", { name: "More" });
  await more.focus();
  await page.keyboard.press("Enter");
  const menu = page.getByRole("menu", { name: "Night Drive" });
  await expect(menu.getByRole("menuitem", { name: "Open folder" })).toBeFocused();
  await expect(menu.getByRole("menuitem", { name: /Open the folder in/ })).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Settings…" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(more).toBeFocused();
});

test("the folded sidebar's team mark has the team list beside it, on hover", async () => {
  await page.getByRole("button", { name: "Hide the sidebar" }).click();
  const mark = page.locator(".mark-slot .team-mark"), caret = page.locator(".mark-slot").getByRole("button", { name: "Switch team" });
  await mark.hover();
  await expect(caret).toHaveCSS("opacity", "1");
  const m = (await mark.boundingBox())!, c = (await caret.boundingBox())!;
  expect(c.x).toBeGreaterThanOrEqual(m.x + m.width - 1); // (to its right, over the room there)
  await caret.click();
  await expect(page.getByRole("menu").getByRole("button", { name: /Demo Band/ })).toBeVisible();
  await caret.click();
  await expect(page.getByRole("menu")).toHaveCount(0);
  await page.getByRole("button", { name: "Show the sidebar" }).click();
});

test("archives a project for the team from its settings (not its menu), and brings it back from the team's home", async () => {
  const row = () => page.locator("aside").getByRole("button", { name: /^Night Drive/ });
  await row().first().click({ button: "right" });
  const menu = page.getByRole("menu", { name: "Night Drive" });
  await expect(menu.getByRole("menuitem", { name: "Settings…" })).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: "Archive project…" })).toHaveCount(0);
  await page.keyboard.press("Escape");
  await row().first().click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("navigation", { name: "Project settings" }).getByRole("button", { name: "General" }).click();
  await page.getByRole("button", { name: "Archive…" }).click();
  const dialog = page.getByRole("dialog", { name: "Archive “Night Drive”?" });
  await expect(dialog).toContainText("Nothing is deleted");
  await dialog.getByRole("button", { name: /^Archive( anyway)?$/ }).click();
  await expect(dialog).toHaveCount(0);
  await expect(row()).toHaveCount(0);
  await page.locator(".team-menu .switch").click();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  const fold = page.locator(".home h3").getByRole("button", { name: /Archived/ });
  await expect(fold).toHaveAttribute("aria-expanded", "false");
  await fold.click();
  await page.getByRole("button", { name: "Unarchive" }).click();
  await expect(row().first()).toBeVisible();
});

// A new section from the Projects band's + (Project or Section).
async function newSection(name: string) {
  await page.locator("aside").getByRole("button", { name: "Add a project or a section" }).click();
  await page.getByRole("menu").getByRole("menuitem", { name: "Section", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New section" });
  await dialog.getByLabel("Section name").fill(name);
  await dialog.getByRole("button", { name: "Create" }).click();
  await expect(dialog).toHaveCount(0);
}

test("makes a section, drags a project onto it, sees it grouped in the panel and the team's home, and deletes the section", async () => {
  await newSection("Smoke");
  const head = page.locator("aside .sect-h").filter({ hasText: "Smoke" });
  await expect(head).toBeVisible();
  // dragged onto the section's header, in steps
  const row = page.locator("aside li[data-key]").filter({ hasText: "Moonrise" });
  const from = (await row.boundingBox())!, to = (await head.boundingBox())!;
  const x = from.x + from.width / 2, y0 = from.y + from.height / 2, y1 = to.y + to.height / 2;
  await page.mouse.move(x, y0);
  await page.mouse.down();
  for (let i = 1; i <= 12; i++) await page.mouse.move(x, y0 + ((y1 - y0) * i) / 12);
  await expect(head).toHaveClass(/\bdrop\b/);
  await page.mouse.up();
  await expect(page.locator("aside ul[aria-label='Smoke']").getByText("Moonrise")).toBeVisible();
  // the team's home groups it the same way
  await expect(page.locator(".home .cards[aria-label='Smoke']").getByText("Moonrise")).toBeVisible();
  // a second section, dragged by its header above the first (this
  // computer's order: the team is asked nothing)
  await newSection("Later");
  const names = () => page.locator("aside .sect-h .sect-name").allTextContents();
  await expect.poll(names).toEqual(["Smoke", "Later"]);
  const later = page.locator("aside .sect-h").filter({ hasText: "Later" });
  const a = (await later.boundingBox())!, b = (await head.boundingBox())!;
  await page.mouse.move(a.x + 40, a.y + a.height / 2);
  await page.mouse.down();
  for (let i = 1; i <= 12; i++) await page.mouse.move(a.x + 40, a.y + a.height / 2 + ((b.y - a.y - 10) * i) / 12);
  await page.mouse.up();
  await expect.poll(names).toEqual(["Later", "Smoke"]);
  expect(await page.evaluate(() => Object.keys(localStorage).some((k) => k.startsWith("r3v.sectionOrder:")))).toBe(true);
  // Moonrise out of its section from its menu: Move to › None
  await page.locator("aside li[data-key]").filter({ hasText: "Moonrise" }).click({ button: "right" });
  await page.getByRole("menu", { name: "Moonrise" }).getByRole("menuitem", { name: /Move to/ }).click();
  const moveTo = page.getByRole("menu", { name: "Move to" });
  await expect(moveTo.getByRole("menuitemradio", { name: "Smoke" })).toHaveAttribute("aria-checked", "true");
  await moveTo.getByRole("menuitemradio", { name: "None" }).click();
  await expect(page.locator("aside ul[aria-label='Smoke']").getByText("Moonrise")).toHaveCount(0);
  await expect(page.locator("aside ul[data-section='']").getByText("Moonrise")).toBeVisible();
  await page.locator("aside .sect-h").filter({ hasText: "Later" }).hover();
  await page.locator("aside .sect-h").filter({ hasText: "Later" }).getByRole("button", { name: "Section menu" }).click();
  await page.getByRole("menu", { name: "Later" }).getByRole("menuitem", { name: "Delete section…" }).click();
  await page.getByRole("dialog", { name: "Delete the section “Later”?" }).getByRole("button", { name: "Delete" }).click();
  // the sections deleted
  await head.hover();
  await head.getByRole("button", { name: "Section menu" }).click();
  await page.getByRole("menu", { name: "Smoke" }).getByRole("menuitem", { name: "Delete section…" }).click();
  await page.getByRole("dialog", { name: "Delete the section “Smoke”?" }).getByRole("button", { name: "Delete" }).click();
  await expect(head).toHaveCount(0);
  await expect(page.locator("aside li[data-key]").filter({ hasText: "Moonrise" })).toBeVisible();
});
