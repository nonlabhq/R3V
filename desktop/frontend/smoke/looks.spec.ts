import { expect, test, type Page } from "@playwright/test";

// The two looks have one structure (AGENTS.md): each main screen, opened in
// Flat and in Cards, has the same landmarks, headings, buttons, tabs and
// menu items (by role and name); only how they're drawn differs. (The demo
// team on a page of its own; nothing is changed.)

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
test.afterEach(() => { expect(errors, "errors in the page").toEqual([]); });

const ROLES = "banner|navigation|main|complementary|region|dialog|heading|button|tab|tablist|menu|menuitem|menuitemradio|link|combobox|textbox|checkbox|radio";

// What a screen offers: its roles and names, in order (times and other text
// left out).
async function read(scope = page.locator("body"), roles = ROLES): Promise<string[]> {
  const snap = await scope.ariaSnapshot();
  return snap.split("\n").map((l) => l.match(new RegExp(`^\\s*- (${roles})(?: "([^"]*)")?`))).filter((m) => !!m)
    .map((m) => `${m![1]} ${m![2] ?? ""}`.trim());
}
// (once the screen has read what it reads: the same twice, 400ms apart)
async function offered(scope = page.locator("body"), roles = ROLES): Promise<string[]> {
  let last = await read(scope, roles);
  for (let i = 0; i < 20; i++) {
    await page.waitForTimeout(400);
    const now = await read(scope, roles);
    if (JSON.stringify(now) === JSON.stringify(last)) return now;
    last = now;
  }
  return last;
}

async function inLook(look: "flat" | "cards") {
  await page.evaluate((l) => localStorage.setItem("r3v.look", l), look);
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-look", look);
  await expect(page.getByRole("button", { name: /^Night Drive/ }).first()).toBeVisible();
  // (the team asked, the list redrawn: then it holds still)
  await expect(page.getByRole("button", { name: "Check the team for new projects" })).not.toHaveClass(/\bspin\b/);
}

// Each main screen, in turn, the same steps in either look.
async function screens(look: "flat" | "cards"): Promise<Record<string, string[]>> {
  await inLook(look);
  const out: Record<string, string[]> = {};
  await page.getByRole("button", { name: /^Night Drive/ }).first().click();
  await page.getByRole("button", { name: /^Versions/ }).click();
  await expect(page.getByRole("listbox", { name: "Versions" })).toBeVisible();
  out.versions = await offered();
  await page.getByRole("button", { name: "Files", exact: true }).click();
  await expect(page.getByRole("navigation", { name: "Folders" })).toBeVisible();
  out.files = await offered();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("navigation", { name: "Project settings" }).getByRole("button", { name: "General" }).click();
  out.projectSettings = await offered();
  await page.locator(".team-menu .switch").click();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Demo Band" })).toBeVisible();
  out.teamHome = await offered();
  await page.getByRole("button", { name: "Team settings" }).click();
  await expect(page.getByRole("textbox", { name: "Team name" })).toBeVisible();
  out.teamSettings = await offered();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.keyboard.press("Control+t");
  const launcher = page.getByRole("dialog");
  await expect(page.getByRole("combobox", { name: "Open a project or team…" })).toBeFocused();
  out.quickSwitcher = await offered(launcher, `${ROLES}|listbox|option`); // (its list too: the projects and teams)
  await page.keyboard.press("Escape");
  return out;
}

test("each main screen offers the same in Flat and in Cards", async () => {
  await screens("cards"); // (once first: what the screens remember is then the same for both)
  const flat = await screens("flat");
  const cards = await screens("cards");
  for (const name of Object.keys(flat)) {
    expect(flat[name].length, `${name}: something offered`).toBeGreaterThan(2);
    expect(cards[name], `${name} in Cards, as in Flat`).toEqual(flat[name]);
  }
  await inLook("flat");
});

// Team home's Activity holds still while the team is read again: its box
// doesn't move or change size, and its lines aren't drawn again.
test("the team's home Activity doesn't move as the team is read again", async () => {
  await page.locator(".team-menu .switch").click();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  const list = page.locator(".activity");
  await expect(list.locator("li").first()).toBeVisible();
  await page.waitForTimeout(500);
  const box = await list.boundingBox();
  await list.locator("li").evaluateAll((els) => els.forEach((el) => el.setAttribute("data-seen", "1")));
  const lines = await list.locator("li").count();
  for (let i = 0; i < 4; i++) {
    await page.getByRole("button", { name: "Check the team for new projects" }).click();
    for (let f = 0; f < 10; f++) {
      await page.waitForTimeout(60);
      expect(await list.boundingBox(), `refresh ${i + 1}`).toEqual(box);
    }
    await expect(page.getByRole("button", { name: "Check the team for new projects" })).not.toHaveClass(/\bspin\b/);
  }
  expect(await list.locator("li[data-seen]").count(), "the same lines, not drawn again").toBe(lines);
});
