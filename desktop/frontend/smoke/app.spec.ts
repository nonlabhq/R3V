import { expect, test, type Page } from "@playwright/test";

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

test("lists the shortcuts, and Esc closes the list", async () => {
  await page.locator("body").click({ position: { x: 5, y: 300 } });
  await page.keyboard.press("Control+/");
  await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toHaveCount(0);
});

test("opens the project's settings", async () => {
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(page.getByText(/rules|Rules/).first()).toBeVisible();
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
