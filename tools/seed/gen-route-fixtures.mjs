// Generates services/graph-engine/testdata/routes.json: expected output of the frontend's own
// roadnet.js on the built-in network, so the Go port can be tested against it.
//   node tools/seed/gen-route-fixtures.mjs
import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { net, loadRoadnet, shortest, resolve, nearestNode, nearestEdge, edgesNear, distFrom } from "../../frontend/src/engine/roadnet.js";
import { NODES } from "../../frontend/src/data/index.js";

await loadRoadnet({ live: false });
if (net.source !== "builtin") throw new Error("expected the built-in network, got " + net.source);

const PAIRS = [
  ["ct", "mus", [], [], 1], ["ct", "aiims", [], [], 1], ["isbt", "sark", [], [], 0.9], ["rajp", "vik", [], [], 0.9],
  ["mal", "doi", ["c_kesar"], [], 0.9], ["ct", "mus", ["c_kolhu", "c_galogi"], [], 0.9], ["prem", "jg", ["c_nkc", "c_funv"], [], 0.9],
  ["sdr", "pn", ["c_itpark", "c_rajpur"], [], 0.5], ["kalsi", "ct", ["c_jaj"], [], 0.9], ["raip", "aiims", [], [{ lat: 30.12, lng: 78.14 }], 0.9],
  ["ct", "chak", ["c_jaj"], [{ lat: 30.4, lng: 77.8 }], 0.7]
];
const cases = PAIRS.map(([a, b, closures, breaks, conf]) => {
  const ctx = { ...resolve(closures, breaks), conf };
  const A = nearestNode(NODES[a][0], NODES[a][1]), B = nearestNode(NODES[b][0], NODES[b][1]);
  const r = shortest(A, B, ctx), h = shortest(A, B, { ...ctx, hard: true });
  const pick = (x) => x && { nodes: x.nodes, edges: x.edges, km: x.km, min: x.min, crosses: x.crosses, steps: x.steps.map((s) => ({ name: s.name, km: s.km, min: s.min, slow: !!s.slow, cut: !!s.cut })) };
  return { from: a, to: b, closures, breaks, conf, A, B, soft: pick(r), hard: pick(h), blocked: [...ctx.blocked].sort((x, y) => x - y), slow: [...ctx.slow].sort((x, y) => x - y) };
});
const d = distFrom(nearestNode(NODES.ct[0], NODES.ct[1]), { conf: 1 });
const lookups = [[30.33, 78.05], [30.1, 78.3], [30.5, 77.8], [30.2, 78.0], [30.39, 78.09]].map(([la, ln]) => {
  const e = nearestEdge(la, ln);
  return { la, ln, node: nearestNode(la, ln), edge: e && e.i, edgeDist: e && e.dist, near: edgesNear(la, ln, 0.5).length };
});
const out = { source: net.source, nodes: net.lat.length, edges: net.edges.length, cases, lookups, distFromCT: Array.from(d).slice(0, 80), cid: net.cid };
const dir = join(dirname(fileURLToPath(import.meta.url)), "../../services/graph-engine/testdata");
mkdirSync(dir, { recursive: true });
writeFileSync(join(dir, "routes.json"), JSON.stringify(out));
console.log("fixtures:", cases.length, "cases,", net.lat.length, "nodes,", net.edges.length, "edges");
