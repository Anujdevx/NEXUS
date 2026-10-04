// Clicks every control on every screen and fails on any page error.
// Run:  npm run build && npm run preview   (in one terminal)
//       npx playwright test                (in another; needs `npm i -D @playwright/test` once)
import { test, expect } from "@playwright/test";

const BASE = process.env.NEXUS_URL || "http://localhost:4173/";
const VIEWS = ["overview", "incidents", "map", "routes", "hospitals", "pharmacies", "sos", "shelters", "responders", "warning", "infrastructure", "evaluation", "modules", "architecture", "sitrep", "audit", "sources", "settings"];
const CONTROLS = "main button, main select, main input, main textarea";

test.beforeEach(async ({ context }) => {
  // These tests click controls, not road data. Run on the built-in network (public/roads.json and Overpass refused),
  // so each of the hundreds of page loads does not rebuild a 170,000-junction graph.
  await context.route(/roads\.json|overpass/, (r) => r.abort());
  await context.addInitScript(() => { try { sessionStorage.setItem("nexus.booted", "1"); } catch { /* ignore */ } window.print = () => {}; });
});

for (const view of VIEWS) {
  test(`every control works on ${view}`, async ({ page }) => {
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    await page.goto(BASE + "#/" + view);
    await page.waitForTimeout(500);
    const n = await page.locator(CONTROLS).count();
    for (let i = 0; i < n; i++) {
      await page.goto(BASE + "#/" + view);
      await page.reload();
      await page.waitForTimeout(250);
      const el = page.locator(CONTROLS).nth(i);
      if (!(await el.isVisible()) || (await el.isDisabled())) continue;
      const tag = await el.evaluate((e) => e.tagName);
      if (tag === "BUTTON") {
        const hold = (await el.getAttribute("aria-label") || "").startsWith("Hold");
        if (hold) { const b = await el.boundingBox(); await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2); await page.mouse.down(); await page.waitForTimeout(1150); await page.mouse.up(); }
        else await el.click();
      } else if (tag === "SELECT") {
        const values = await el.evaluate((e) => [...e.options].map((o) => o.value));
        if (values.length > 1) await el.selectOption(values[values.length - 1]);
      } else {
        const type = await el.getAttribute("type");
        if (type === "range") await el.evaluate((e) => { const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set; set.call(e, (Number(e.max) + Number(e.min)) / 2); e.dispatchEvent(new Event("input", { bubbles: true })); });
        else await el.fill(type === "number" ? "7" : "ra");
      }
      await page.waitForTimeout(100);
    }
    expect(errors).toEqual([]);
  });
}

test("breaking a road on the route makes the planner find another way", async ({ page }) => {
  await page.goto(BASE + "#/routes");
  await page.getByRole("button", { name: "Place", exact: true }).click();
  await page.getByLabel("Destination place").selectOption("node:rish");
  const first = page.locator(".steps").first();
  await expect(first).toContainText("km");
  const before = await first.innerText();
  await first.getByRole("button", { name: "Break" }).nth(2).click();
  await expect(page.locator(".note").first()).toContainText("Rerouted");
  expect(await first.innerText()).not.toEqual(before);
});

test("an SOS sent with the network down is held, then delivered", async ({ page }) => {
  await page.goto(BASE + "#/sos");
  await page.getByRole("button", { name: "Cut the network" }).click();
  const b = await page.locator(".sos-btn").boundingBox();
  await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2); await page.mouse.down(); await page.waitForTimeout(1150); await page.mouse.up();
  await page.getByRole("button", { name: "Send request" }).click();
  await expect(page.locator(".phone")).toContainText("Held on this device");
  await page.getByRole("button", { name: "Peer finds an uplink" }).click();
  await expect(page.locator("ol.flow").first()).toContainText("Delivered by peer relay", { timeout: 4000 });
});
