// Exports every dataset in frontend/src/data/index.js to tools/seed/out/<NAME>.json.
// The frontend module is the single source of truth: services seed their stores from these files.
// Also writes two derived files (HOSP_STATE, STOCK) computed with the frontend's own seeding
// logic (store.js: seedHosp, seedStock) so the Go ports can be tested against them.
//   node tools/seed/export-data.mjs
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import * as D from "../../frontend/src/data/index.js";

const out = join(dirname(fileURLToPath(import.meta.url)), "out");
mkdirSync(out, { recursive: true });

const names = ["HOSPITALS", "PHARMACIES", "MEDICINES", "SHELTERS", "UNITS", "NODES", "EDGES", "CLOSURES", "SCENARIOS", "FLOOD", "SLIDES", "INFRA", "RAIN", "ZONING", "RIVERWATCH", "DISTRICTS", "MODULES", "PLACES", "RIVERS", "SRC", "SPEED", "WIND", "CAPS", "SIMCAP"];
for (const n of names) {
  if (!(n in D)) throw new Error("missing export " + n);
  writeFileSync(join(out, n + ".json"), JSON.stringify(D[n]));
}

// ---- derived: same formulas as src/state/store.js ----
const hosp = {};
D.HOSPITALS.forEach((h, i) => {
  const cap = D.SIMCAP[h.lvl];
  hosp[h.id] = { cap, free: Math.max(1, Math.round(cap * (0.18 + ((i * 37) % 50) / 100))), inbound: 0, divert: false };
});
writeFileSync(join(out, "HOSP_STATE.json"), JSON.stringify(hosp));

const stock = Object.fromEntries(D.PHARMACIES.map((p, i) => [p.id, Object.fromEntries(D.MEDICINES.map((m, j) => {
  const h = Math.abs(Math.sin((i + 1) * 12.9898 + (j + 1) * 78.233) * 43758.5453) % 1;
  return [m.id, h < 0.2 ? 0 : Math.round(m.base * (0.15 + h * 1.2))];
}))]));
writeFileSync(join(out, "STOCK.json"), JSON.stringify(stock));

console.log(`seed: wrote ${names.length + 2} files to ${out}`);
