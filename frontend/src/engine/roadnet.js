/* The road network every route is planned on.
   1. public/roads.json  real OpenStreetMap roads, saved once with `npm run roads`
   2. Overpass API       the same roads fetched live, if the file is missing
   3. built-in network   a schematic of the main roads, so the planner always works
   Routing is A* (Dijkstra with a straight-line heuristic). A broken road is not deleted:
   its cost is multiplied by report confidence, so a wrong report degrades the route gracefully. */
import { NODES, EDGES, SPEED, WIND, CLOSURES } from "../data/index.js";

const KMH = { motorway: 60, trunk: 50, primary: 38, secondary: 32, tertiary: 26, unclassified: 20, residential: 16 };
const CLASS_NAME = { motorway: "Motorway", trunk: "Trunk road", primary: "Primary road", secondary: "Secondary road", tertiary: "Tertiary road", unclassified: "Minor road", residential: "Residential street" };
const HARD = 40, SLOW = 1.2;
export const TRUE_SLOW = 2.2;
export const BBOX = [30.05, 77.7, 30.56, 78.36]; // south, west, north, east
const CELL = 0.01;
const RAD = Math.PI / 180;

export function hav(aLat, aLng, bLat, bLng) {
  const dLat = (bLat - aLat) * RAD, dLng = (bLng - aLng) * RAD;
  const s = Math.sin(dLat / 2) ** 2 + Math.cos(aLat * RAD) * Math.cos(bLat * RAD) * Math.sin(dLng / 2) ** 2;
  return 12742 * Math.asin(Math.sqrt(s));
}

export const net = { source: "none", lat: [], lng: [], adj: [], edges: [], ways: [], grid: new Map(), cid: {}, version: 0 };
const listeners = new Set();
export const onNet = (f) => { listeners.add(f); return () => listeners.delete(f); };

/* ways: [{ name, hw, pts: [[lat, lng], ...], ids?, kmh?, wind?, cid? }] */
function build(ways, source) {
  const lat = [], lng = [], adj = [], edges = [], index = new Map(), grid = new Map(), cid = {};
  const node = (key, la, ln) => {
    let i = index.get(key);
    if (i === undefined) {
      i = lat.length; index.set(key, i); lat.push(la); lng.push(ln); adj.push([]);
      const g = Math.floor(la / CELL) + ":" + Math.floor(ln / CELL);
      if (!grid.has(g)) grid.set(g, []);
      grid.get(g).push(i);
    }
    return i;
  };
  const drawn = [];
  ways.forEach((w, wi) => {
    const kmh = w.kmh || KMH[w.hw] || 20, wind = w.wind || 1;
    let prev = -1;
    w.pts.forEach((p, k) => {
      const key = w.ids ? w.ids[k] : p[0].toFixed(5) + "," + p[1].toFixed(5);
      const n = node(key, p[0], p[1]);
      if (prev >= 0 && prev !== n) {
        const km = hav(lat[prev], lng[prev], p[0], p[1]) * wind;
        const e = { i: edges.length, a: prev, b: n, km, min: (km / kmh) * 60, name: w.name || CLASS_NAME[w.hw] || "Road", hw: w.hw, way: wi };
        edges.push(e); adj[prev].push(e); adj[n].push(e);
        if (w.cid) (cid[w.cid] = cid[w.cid] || []).push(e.i);
      }
      prev = n;
    });
    drawn.push({ hw: w.hw, coords: w.pts.map((p) => [p[1], p[0]]) });
  });
  Object.assign(net, { source, lat, lng, adj, edges, ways: drawn, grid, cid });
  if (source !== "builtin") Object.entries(CLOSURES).forEach(([id, c]) => { net.cid[id] = edgesNear(c.at[0], c.at[1], 0.15); });
  net.version++;
  listeners.forEach((f) => f());
}

function builtin() {
  const cls = { h: "primary", a: "secondary", m: "tertiary" };
  build(EDGES.map(([a, b, road, c, cid]) => ({ name: road, hw: cls[c], pts: [[NODES[a][0], NODES[a][1]], [NODES[b][0], NODES[b][1]]], kmh: SPEED[c], wind: WIND[c], cid })), "builtin");
}

const QUERY = `[out:json][timeout:60];way["highway"~"^(motorway|trunk|primary|secondary|tertiary|unclassified)(_link)?$"](${BBOX.join(",")});out geom;`;
export function fromOverpass(json) {
  return json.elements.filter((e) => e.type === "way" && e.geometry && e.geometry.length > 1).map((e) => ({
    name: e.tags?.name || e.tags?.ref || "", hw: (e.tags?.highway || "unclassified").replace("_link", ""), ids: e.nodes, pts: e.geometry.map((g) => [g.lat, g.lon])
  }));
}

/* Start on the built-in network at once, then upgrade to real roads if they can be had. */
export async function loadRoadnet({ live = true } = {}) {
  builtin();
  try {
    const r = await fetch(import.meta.env.BASE_URL + "roads.json", { cache: "no-cache" });
    if (r.ok && (r.headers.get("content-type") || "").includes("json")) {
      const j = await r.json();
      if (j.ways && j.ways.length > 50) { build(j.ways, "osm-file"); return net.source; }
    }
  } catch { /* no file: fall through */ }
  if (!live) return net.source;
  /* a copy kept by the browser from an earlier live fetch */
  try {
    const hit = await (await caches.open("nexus-roads")).match("overpass");
    if (hit) { const ways = fromOverpass(await hit.json()); if (ways.length > 50) { build(ways, "osm-live"); return net.source; } }
  } catch { /* no cache storage */ }
  for (const host of ["https://overpass-api.de/api/interpreter", "https://overpass.kumi.systems/api/interpreter"]) {
    try {
      const ctl = new AbortController();
      const t = setTimeout(() => ctl.abort(), 45000);
      const r = await fetch(host, { method: "POST", body: "data=" + encodeURIComponent(QUERY), headers: { "Content-Type": "application/x-www-form-urlencoded" }, signal: ctl.signal });
      clearTimeout(t);
      if (!r.ok) continue;
      const copy = r.clone();
      const ways = fromOverpass(await r.json());
      if (ways.length > 50) {
        build(ways, "osm-live");
        try { (await caches.open("nexus-roads")).put("overpass", copy); } catch { /* ignore */ }
        return net.source;
      }
    } catch { /* offline or refused: try the next mirror, then keep the built-in network */ }
  }
  return net.source;
}

/* ---------- lookups ---------- */
function nearCells(la, ln, ring) {
  const out = [], ci = Math.floor(la / CELL), cj = Math.floor(ln / CELL);
  for (let i = ci - ring; i <= ci + ring; i++) for (let j = cj - ring; j <= cj + ring; j++) { const c = net.grid.get(i + ":" + j); if (c) out.push(...c); }
  return out;
}
export function nearestNode(la, ln) {
  for (const ring of [1, 3, 8, 25, 80]) {
    const c = nearCells(la, ln, ring);
    if (!c.length) continue;
    let best = c[0], bd = Infinity;
    for (const n of c) { const d = hav(la, ln, net.lat[n], net.lng[n]); if (d < bd) { bd = d; best = n; } }
    return best;
  }
  return 0;
}
function segDist(la, ln, e) {
  const ax = net.lng[e.a], ay = net.lat[e.a], bx = net.lng[e.b], by = net.lat[e.b];
  const k = Math.cos(la * RAD), px = (ln - ax) * k, py = la - ay, dx = (bx - ax) * k, dy = by - ay;
  const t = Math.max(0, Math.min(1, (px * dx + py * dy) / (dx * dx + dy * dy || 1)));
  return hav(la, ln, ay + t * (by - ay), ax + t * (bx - ax));
}
export function edgesNear(la, ln, km, strict = false) {
  const seen = new Set(), out = [];
  for (const n of nearCells(la, ln, Math.max(2, Math.ceil(km / 1.0) + 1))) for (const e of net.adj[n]) if (!seen.has(e.i)) { seen.add(e.i); if (segDist(la, ln, e) <= km) out.push(e.i); }
  if (!out.length && !strict) { const e = nearestEdge(la, ln); if (e) out.push(e.i); }
  return out;
}
export function nearestEdge(la, ln) {
  let best = null, bd = Infinity;
  for (const ring of [1, 4, 12, 40]) {
    for (const n of nearCells(la, ln, ring)) for (const e of net.adj[n]) { const d = segDist(la, ln, e); if (d < bd) { bd = d; best = e; } }
    if (best) break;
  }
  return best ? { ...best, dist: bd } : null;
}
export const edgeMid = (e) => [(net.lat[e.a] + net.lat[e.b]) / 2, (net.lng[e.a] + net.lng[e.b]) / 2];

/* ---------- binary heap ---------- */
class Heap {
  constructor() { this.k = []; this.v = []; }
  push(key, val) { const k = this.k, v = this.v; let i = k.length; k.push(key); v.push(val); while (i > 0) { const p = (i - 1) >> 1; if (k[p] <= k[i]) break; [k[p], k[i]] = [k[i], k[p]]; [v[p], v[i]] = [v[i], v[p]]; i = p; } }
  pop() { const k = this.k, v = this.v, top = v[0], lk = k.pop(), lv = v.pop(); if (k.length) { k[0] = lk; v[0] = lv; let i = 0; for (;;) { const l = 2 * i + 1, r = l + 1; let m = i; if (l < k.length && k[l] < k[m]) m = l; if (r < k.length && k[r] < k[m]) m = r; if (m === i) break; [k[m], k[i]] = [k[i], k[m]]; [v[m], v[i]] = [v[i], v[m]]; i = m; } } return top; }
  get size() { return this.k.length; }
}

/* What the planner believes a segment costs, given what it knows. */
const believed = (e, o) => (o.blocked && o.blocked.has(e.i) ? (o.hard ? Infinity : e.min * (1 + o.conf * HARD)) : o.slow && o.slow.has(e.i) ? e.min * (1 + o.conf * SLOW) : e.min);

/* Travel time from one node to every other node. */
export function distFrom(src, o = {}) {
  const opt = { conf: 1, ...o }, dist = new Float64Array(net.lat.length).fill(Infinity), h = new Heap();
  dist[src] = 0; h.push(0, src);
  while (h.size) {
    const u = h.pop();
    for (const e of net.adj[u]) {
      const w = believed(e, opt); if (w === Infinity) continue;
      const v = e.a === u ? e.b : e.a, nd = dist[u] + w;
      if (nd < dist[v]) { dist[v] = nd; h.push(nd, v); }
    }
  }
  return dist;
}

/* Full shortest-path tree under any cost function; used by the evaluation sweep. */
export function treeFrom(src, cost) {
  const n = net.lat.length, dist = new Float64Array(n).fill(Infinity), prev = new Int32Array(n).fill(-1), via = new Int32Array(n).fill(-1), h = new Heap();
  dist[src] = 0; h.push(0, src);
  while (h.size) {
    const u = h.pop();
    for (const e of net.adj[u]) {
      const w = cost(e); if (w === Infinity) continue;
      const v = e.a === u ? e.b : e.a, nd = dist[u] + w;
      if (nd < dist[v]) { dist[v] = nd; prev[v] = u; via[v] = e.i; h.push(nd, v); }
    }
  }
  return { dist, prev, via };
}

/* Shortest path between two nodes. Returns null only if no road joins them at all. */
export function shortest(a, b, o = {}) {
  const opt = { conf: 1, ...o }, n = net.lat.length;
  const g = new Float64Array(n).fill(Infinity), prev = new Int32Array(n).fill(-1), via = new Int32Array(n).fill(-1), done = new Uint8Array(n), h = new Heap();
  const est = (i) => hav(net.lat[i], net.lng[i], net.lat[b], net.lng[b]); // km at 60 km/h is minutes: never overestimates
  g[a] = 0; h.push(est(a), a);
  while (h.size) {
    const u = h.pop();
    if (done[u]) continue; done[u] = 1;
    if (u === b) break;
    for (const e of net.adj[u]) {
      const w = believed(e, opt); if (w === Infinity) continue;
      const v = e.a === u ? e.b : e.a, nd = g[u] + w;
      if (nd < g[v]) { g[v] = nd; prev[v] = u; via[v] = e.i; h.push(nd + est(v), v); }
    }
  }
  if (a !== b && prev[b] < 0) return null;
  const nodes = [b], es = [];
  for (let v = b; v !== a; v = prev[v]) { es.unshift(via[v]); nodes.unshift(prev[v]); }
  let km = 0, min = 0;
  const crosses = [], steps = [];
  es.forEach((ei) => {
    const e = net.edges[ei];
    const slow = opt.slow && opt.slow.has(ei), cut = opt.blocked && opt.blocked.has(ei);
    const m = slow ? e.min * TRUE_SLOW : e.min;
    km += e.km; min += m;
    if (cut) crosses.push(ei);
    const last = steps[steps.length - 1];
    if (last && last.name === e.name) { last.km += e.km; last.min += m; last.edges.push(ei); last.slow = last.slow || slow; last.cut = last.cut || cut; }
    else steps.push({ name: e.name, km: e.km, min: m, edges: [ei], slow, cut });
  });
  return { nodes, edges: es, km, min, crosses, steps, coords: nodes.map((i) => [net.lat[i], net.lng[i]]) };
}

/* Resolve the app's closures and the user's breaks onto this network. */
export function resolve(activeClosureIds, breaks) {
  const blocked = new Set(), slow = new Set();
  activeClosureIds.forEach((id) => (net.cid[id] || []).forEach((e) => (CLOSURES[id].kind === "closed" ? blocked : slow).add(e)));
  breaks.forEach((b) => edgesNear(b.lat, b.lng, 0.03).forEach((e) => blocked.add(e)));
  return { blocked, slow };
}

export const describe = () => ({
  source: net.source, nodes: net.lat.length, edges: net.edges.length,
  label: net.source === "builtin" ? "Built-in schematic network" : net.source === "osm-file" ? "OpenStreetMap roads (saved file)" : "OpenStreetMap roads (live from Overpass)"
});
