// Generates services/graph-engine/testdata/eval.json: the frontend's own simulation.js sweep on the
// built-in network, so the Go port can be tested against it.
//   node tools/seed/gen-eval-fixture.mjs
import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { net, loadRoadnet, resolve } from "../../frontend/src/engine/roadnet.js";
import { sweep } from "../../frontend/src/engine/simulation.js";

await loadRoadnet({ live: false });
if (net.source !== "builtin") throw new Error("expected the built-in network");
const closures = ["c_kolhu", "c_nkc", "c_kesar", "c_itpark"], breaks = [{ lat: 30.2, lng: 78.1 }];
const sets = resolve(closures, breaks);
const params = { nSos: 20, runs: 6, seed: 2026, relay: 12, retry: 25 };
const out = await sweep({ sets, ...params });
const clean = JSON.parse(JSON.stringify(out, (k, v) => (typeof v === "number" && !isFinite(v) ? null : v)));
const dir = join(dirname(fileURLToPath(import.meta.url)), "../../services/graph-engine/testdata");
writeFileSync(join(dir, "eval.json"), JSON.stringify({ closures, breaks, params, out: clean }));
console.log("eval fixture:", out.length, "rows");
