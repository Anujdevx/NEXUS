// Compares the console's own rankOnNet() with the backend's POST /hospitals/rank at the same points,
// for every flood-prone locality and every clinical need. Needs the stack up and a Vite dev server:
//   scripts/seed.sh --reset && (cd frontend && npx vite --port 5175) &
//   node tools/parity/rank-parity.mjs http://localhost:5175
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
const require = createRequire(join(dirname(fileURLToPath(import.meta.url)), "../../frontend/package.json"));
const { chromium } = require("@playwright/test");
const base = process.argv[2] || "http://localhost:5175";

const b = await chromium.launch();
const p = await b.newPage();
await p.goto(base + "/#/hospitals");
await p.waitForFunction(async () => { const m = await import("/src/state/store.js"); return m.getState().netInfo.source !== "none" && m.getState().netInfo.source !== "builtin" && m.getState().live; }, null, { timeout: 90000 });
const res = await p.evaluate(async () => {
  const m = await import("/src/state/store.js");
  const { FLOOD } = await import("/src/data/index.js");
  const tok = (await (await fetch("/api/v1/auth/demo-token", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ role: "Controller" }) })).json()).token;
  const out = { total: 0, same: 0, diffs: [] };
  for (const f of FLOOD) for (const need of ["Trauma", "Cardiac", "Maternity", "General"]) {
    const local = m.rankOnNet({ lat: f.lat, lng: f.lng }, need);
    const r = await (await fetch("/api/v1/hospitals/rank", { method: "POST", headers: { "Content-Type": "application/json", Authorization: "Bearer " + tok }, body: JSON.stringify({ at: { lat: f.lat, lng: f.lng }, need }) })).json();
    out.total++;
    const a = local[0].h.id, bk = r.items[0].h.id;
    if (a === bk && Math.abs((local[0].min ?? 1e9) - (r.items[0].min ?? 1e9)) < 0.01) out.same++;
    else out.diffs.push({ at: f.n, need, local: a + " " + local[0].min?.toFixed(2) + " cost " + local[0].cost.toFixed(2), backend: bk + " " + (r.items[0].min ?? "cut off") + " cost " + r.items[0].cost.toFixed(2) });
  }
  return out;
});
console.log(`rank parity: ${res.same}/${res.total} top choices and drive times identical`);
res.diffs.slice(0, 8).forEach((d) => console.log("  differs:", JSON.stringify(d)));
await b.close();
process.exit(res.same === res.total ? 0 : 1);
