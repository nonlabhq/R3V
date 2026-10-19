import { expect, test, type Page } from "@playwright/test";

// Dragging to reorder, in Edge: the project (or tab) dragged stays under the
// pointer at every step, whatever the order does meanwhile; the others slide
// out of its way. (The demo team on a page of its own: the order changed here
// is kept in this page's storage only.)

test.describe.configure({ mode: "serial" });

let page: Page;
const errors: string[] = [];

test.beforeAll(async ({ browser }) => {
  page = await browser.newPage();
  page.on("pageerror", (e) => errors.push(`page error: ${e.message}`));
  page.on("console", (m) => { if (m.type() === "error") errors.push(`console: ${m.text()}`); });
  await page.goto("/");
  await expect(page.locator("li[data-key]").nth(2)).toBeVisible();
  // (the team asked, the list redrawn: then it holds still)
  await expect(page.getByRole("button", { name: "Check the team for new projects" })).not.toHaveClass(/\bspin\b/);
});
test.afterAll(async () => { await page.close(); });
test.afterEach(() => { expect(errors, "errors in the page").toEqual([]); });

const frame = (p: Page) => p.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
const order = (p: Page, sel: string) => p.locator(sel).evaluateAll((els) => els.map((el) => (el as HTMLElement).dataset.key));

// Presses the nth item of sel and drags it along axis through the offsets
// in path (px), checking at each step, once it is a drag, that it is where
// the pointer holds it (within 2px).
async function drag(p: Page, sel: string, nth: number, axis: "x" | "y", path: number[]) {
  // (a handle: the nth item changes as the order does)
  const el = (await p.locator(sel).nth(nth).elementHandle())!;
  await p.waitForTimeout(400); // (a drag just let go settles first)
  const box = (await el.boundingBox())!;
  const x0 = box.x + box.width / 2, y0 = box.y + box.height / 2;
  await p.mouse.move(x0, y0);
  await p.mouse.down();
  try {
    let started = false;
    for (const d of path) {
      await p.mouse.move(axis === "x" ? x0 + d : x0, axis === "y" ? y0 + d : y0);
      await frame(p);
      started ||= Math.abs(d) >= 6; // (a press becomes a drag past a few pixels)
      if (!started) continue;
      const b = (await el.boundingBox())!;
      const off = axis === "y" ? b.y - (box.y + d) : b.x - (box.x + d);
      expect(Math.abs(off), `${axis} off by ${off.toFixed(1)}px at ${d}px`).toBeLessThanOrEqual(2);
    }
  } finally {
    await p.mouse.up();
  }
}

// Out to `to` px and back to a little before the start, by steps of `by`.
const there = (to: number, by: number) => {
  const out: number[] = [];
  for (let d = by; d <= to; d += by) out.push(d);
  for (let d = to - by; d >= -by * 3; d -= by) out.push(d);
  return out;
};

test("a project dragged in the sidebar stays under the pointer as the others slide", async () => {
  const before = await order(page, "li[data-key]");
  const h = (await page.locator("li[data-key]").first().boundingBox())!.height;
  await drag(page, "li[data-key]", 0, "y", there(Math.round(h * 2.4), 3)); // past two others, and back
  await expect.poll(() => order(page, "li[data-key]")).toEqual(before);
  await drag(page, "li[data-key]", 0, "y", [3, 10, 20, Math.round(h * 1.2)]); // let go past the next one
  await expect.poll(() => order(page, "li[data-key]")).toEqual([before[1], before[0], ...before.slice(2)]);
});

test("a project dragged in the folded sidebar stays under the pointer", async () => {
  await page.getByRole("button", { name: "Hide the sidebar" }).click();
  await expect(page.locator(".folded-row").nth(2)).toBeVisible();
  const before = await order(page, ".folded-row");
  const h = (await page.locator(".folded-row").first().boundingBox())!.height;
  await drag(page, ".folded-row", 0, "y", there(Math.round(h * 2.4), 3));
  await expect.poll(() => order(page, ".folded-row")).toEqual(before);
  await page.getByRole("button", { name: "Show the sidebar" }).click();
});

test("a tab dragged stays under the pointer, keeps its new place, and a click still switches tabs", async () => {
  for (const name of ["Night Drive", "Moonrise", "Fresh Idea"]) { // (each opened behind, once)
    await page.getByRole("button", { name: new RegExp(`^${name}`) }).first().click({ modifiers: ["Control"] });
  }
  await expect(page.locator(".tab").nth(2)).toBeVisible();
  const before = await order(page, ".tab");
  const w = (await page.locator(".tab").first().boundingBox())!.width;
  await drag(page, ".tab", 0, "x", there(Math.round(w * 2.3), 4));
  await expect.poll(() => order(page, ".tab")).toEqual(before);
  await drag(page, ".tab", 0, "x", [4, 10, 20, Math.round(w * 1.2)]);
  const after = [before[1], before[0], ...before.slice(2)];
  await expect.poll(() => order(page, ".tab")).toEqual(after);
  // kept with the tabs
  const saved = await page.evaluate(() => (JSON.parse(localStorage.getItem("r3v.tabs") ?? "[]") as { team: string; key: string }[])
    .map((x) => `${x.team}\n${x.key}`));
  expect(saved).toEqual(after.filter((id) => saved.includes(id!)));
  expect(saved.length).toBeGreaterThanOrEqual(3);
  await page.locator(".tab").nth(2).getByRole("tab").click();
  await expect(page.locator(".tab").nth(2).getByRole("tab")).toHaveAttribute("aria-selected", "true");
});
