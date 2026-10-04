/* Connectivity sweep on the loaded road network (real OpenStreetMap roads when available).
   The roads are real; the SOS load, hospital capacity and connectivity are generated.
   Both policies see identical seeds. */
import { HOSPITALS, CAPS, SIMCAP, FLOOD, NODES, UNITS } from "../data/index.js";
import { net, nearestNode, treeFrom, hav, TRUE_SLOW } from "./roadnet.js";

function rng(seed) {
  let a = seed >>> 0;
  return () => {
    a |= 0; a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const NEEDS = ["General", "General", "Trauma", "Trauma", "Maternity", "Cardiac"];
const LIMIT = 150; // minutes; beyond this a casualty counts as unserved
export const LEVELS = [1, 0.8, 0.6, 0.4, 0.2];

const stats = (xs) => {
  const v = xs.filter((x) => isFinite(x));
  const n = v.length || 1, m = v.reduce((a, b) => a + b, 0) / n;
  const sd = Math.sqrt(v.reduce((a, b) => a + (b - m) ** 2, 0) / Math.max(1, n - 1));
  return { m, ci: (1.96 * sd) / Math.sqrt(n) };
};

/* Travel-time tables between the places the sweep uses. For each source:
   full  = true minutes when the planner knows every break (broken roads avoided)
   blind = the route planned as if every road were open, and where it first hits a break */
function tables({ blocked, slow }) {
  const hosp = HOSPITALS.map((h) => nearestNode(h.lat, h.lng));
  const orig = FLOOD.map((f) => nearestNode(f.lat, f.lng));
  const bases = UNITS.filter((u) => u.type === "Ambulance").map((u) => nearestNode(NODES[u.node][0], NODES[u.node][1]));
  const dest = [...hosp, ...orig];
  const costFull = (e) => (blocked.has(e.i) ? Infinity : slow.has(e.i) ? e.min * TRUE_SLOW : e.min);
  const costBlind = (e) => e.min;
  const T = new Map();
  const ensure = (src) => {
    let t = T.get(src);
    if (t) return t;
    const f = treeFrom(src, costFull), b = treeFrom(src, costBlind);
    const blind = dest.map((d) => {
      if (!isFinite(b.dist[d])) return { plain: Infinity, min: Infinity, cut: -1, tcut: 0 };
      const es = [], ns = [d];
      for (let v = d; v !== src; v = b.prev[v]) { es.unshift(b.via[v]); ns.unshift(b.prev[v]); }
      let min = 0;
      for (let k = 0; k < es.length; k++) {
        if (blocked.has(es[k])) return { plain: b.dist[d], min: Infinity, cut: ns[k], tcut: min };
        min += slow.has(es[k]) ? net.edges[es[k]].min * TRUE_SLOW : net.edges[es[k]].min;
      }
      return { plain: b.dist[d], min, cut: -1, tcut: 0 };
    });
    t = { full: dest.map((d) => f.dist[d]), blind };
    T.set(src, t);
    return t;
  };
  /* Minutes actually driven. A crew without the road picture drives into the break, loses three minutes, and replans. */
  const trip = (src, di, knows) => {
    const t = ensure(src);
    if (knows) return t.full[di];
    const b = t.blind[di];
    return b.cut < 0 ? b.min : b.tcut + 3 + ensure(b.cut).full[di];
  };
  const guess = (src, di, knows) => (knows ? ensure(src).full[di] : ensure(src).blind[di].plain);
  return { hosp, orig, bases, ensure, trip, guess, sources: [...new Set([...orig, ...bases, ...hosp])] };
}

function oneRun(tb, seed, conn, policy, nSos, relay, retry) {
  const rand = rng(seed);
  const expo = (mean) => -Math.log(1 - rand()) * mean;
  const free = HOSPITALS.map((h) => Math.max(1, Math.round(SIMCAP[h.lvl] * (0.08 + rand() * 0.22))));
  let sum = 0, served = 0, unserved = 0, good = 0, delivered = 0;
  for (let i = 0; i < nSos; i++) {
    const oi = Math.floor(rand() * tb.orig.length), origin = tb.orig[oi], od = HOSPITALS.length + oi;
    const need = NEEDS[Math.floor(rand() * NEEDS.length)];
    const online = rand() < conn, rKnow = rand(), rCap = rand(), dN = expo(relay), dB = expo(retry);
    // Offline: Nexus relays device to device; the baseline waits for a call to get through.
    const delay = online ? 0 : policy === "nexus" ? dN : dB;
    if (delay <= 30) delivered++;
    const knows = policy === "nexus" && rKnow < conn;        // does the dispatcher hold the current road picture?
    const seesCapacity = policy === "nexus" && rCap < conn;

    let base = tb.bases[0], bb = Infinity;
    for (const b of tb.bases) {
      const c = policy === "nexus" ? tb.guess(b, od, knows) : hav(net.lat[b], net.lng[b], net.lat[origin], net.lng[origin]);
      if (c < bb) { bb = c; base = b; }
    }
    let t = delay + tb.trip(base, od, knows);

    const tried = new Set();
    let at = origin, ok = false, first = true;
    for (let hop = 0; hop < 4 && isFinite(t); hop++) {
      let pick = -1, best = Infinity;
      HOSPITALS.forEach((h, hi) => {
        if (tried.has(hi)) return;
        let c;
        if (policy === "nexus") {
          if (!CAPS[h.lvl].includes(need)) return;
          c = tb.guess(at, hi, knows) + (seesCapacity || hop > 0 ? (free[hi] > 0 ? 0 : 500) : 0);
        } else c = hav(net.lat[at], net.lng[at], h.lat, h.lng);
        if (c < best) { best = c; pick = hi; }
      });
      if (pick < 0) break;
      tried.add(pick);
      t += tb.trip(at, pick, knows);
      const fits = CAPS[HOSPITALS[pick].lvl].includes(need) && free[pick] > 0;
      if (first && fits) good++;
      first = false;
      if (fits) { free[pick]--; ok = true; break; }
      t += 6; at = tb.hosp[pick]; // turned away: lose six minutes, go on
    }
    if (ok && t <= LIMIT) { sum += t; served++; } else unserved++;
  }
  return { mean: served ? sum / served : NaN, unserved, quality: good / nSos, delivery: delivered / nSos };
}

/* Runs in slices so the page stays responsive: first the travel-time tables, then the runs. */
export function sweep({ sets, nSos = 40, runs = 30, seed = 2026, relay = 12, retry = 25 }, onProgress) {
  return new Promise((resolve) => {
    const tb = tables(sets);
    const jobs = [];
    LEVELS.forEach((c) => ["nexus", "base"].forEach((p) => jobs.push([c, p])));
    const out = [];
    let k = 0, j = 0;
    const prep = () => {
      const until = performance.now() + 40;
      while (k < tb.sources.length && performance.now() < until) tb.ensure(tb.sources[k++]);
      onProgress && onProgress((k / tb.sources.length) * 0.6);
      setTimeout(k < tb.sources.length ? prep : step, 0);
    };
    const step = () => {
      const [c, p] = jobs[j];
      const rs = [];
      for (let r = 0; r < runs; r++) rs.push(oneRun(tb, seed + r * 7919, c, p, nSos, relay, retry));
      out.push({ conn: c, policy: p, ttc: stats(rs.map((x) => x.mean)), unserved: stats(rs.map((x) => x.unserved)), quality: stats(rs.map((x) => x.quality)), delivery: stats(rs.map((x) => x.delivery)) });
      j++;
      onProgress && onProgress(0.6 + (j / jobs.length) * 0.4);
      if (j < jobs.length) setTimeout(step, 0); else resolve(out);
    };
    setTimeout(prep, 0);
  });
}
