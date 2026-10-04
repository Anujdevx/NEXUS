/* One small store for the whole dashboard. Every action that changes state
   publishes an envelope on the Sūtra bus and writes an audit entry. */
import { useSyncExternalStore } from "react";
import { HOSPITALS, SIMCAP, SCENARIOS, SHELTERS, UNITS, NODES, CLOSURES, FLOOD, CAPS, PHARMACIES, MEDICINES } from "../data/index.js";
import { net, nearestNode, nearestEdge, edgesNear, edgeMid, shortest, distFrom, resolve, loadRoadnet, onNet, describe, hav, TRUE_SLOW } from "../engine/roadnet.js";
import { inferFromRain } from "../engine/hazard.js";

const load = (k, d) => { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : d; } catch { return d; } };
const save = (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* storage unavailable */ } };

const seedHosp = () => {
  const o = {};
  HOSPITALS.forEach((h, i) => {
    const cap = SIMCAP[h.lvl];
    o[h.id] = { cap, free: Math.max(1, Math.round(cap * (0.18 + ((i * 37) % 50) / 100))), inbound: 0, divert: false };
  });
  return o;
};
const scenarioIncidents = (id) =>
  SCENARIOS[id].inc.map((x, i) => ({ ...x, id: `${id}-${i + 1}`, status: "Open", unit: null, sim: false, when: SCENARIOS[id].short, at: (i + 1) * 10, log: [["Reported", SCENARIOS[id].short]] }));
/* Simulated pharmacy stock: deterministic, with some items out of stock. */
const seedStock = () => Object.fromEntries(PHARMACIES.map((p, i) => [p.id, Object.fromEntries(MEDICINES.map((m, j) => {
  const h = Math.abs(Math.sin((i + 1) * 12.9898 + (j + 1) * 78.233) * 43758.5453) % 1;
  return [m.id, h < 0.2 ? 0 : Math.round(m.base * (0.15 + h * 1.2))];
}))]));
const closureMap = (id) => Object.fromEntries(Object.keys(CLOSURES).map((c) => [c, SCENARIOS[id].closures.includes(c)]));

export const LENSES = {
  All: { flood: true, corridors: false, slides: true, closures: true, hospitals: true, pharmacies: true, shelters: true, units: true, incidents: true, buildings: true, graph: false },
  Flood: { flood: true, corridors: true, slides: false, closures: true, hospitals: false, shelters: true, units: false, incidents: true, buildings: false, graph: false },
  Landslide: { flood: false, corridors: false, slides: true, closures: true, hospitals: false, shelters: false, units: false, incidents: true, buildings: false, graph: false },
  Health: { flood: false, corridors: false, slides: false, closures: true, hospitals: true, pharmacies: true, shelters: false, units: true, incidents: true, buildings: false, graph: false },
  Roads: { flood: false, corridors: false, slides: true, closures: true, hospitals: true, pharmacies: true, shelters: false, units: false, incidents: false, buildings: false, graph: true }
};

const hashView = () => (location.hash.replace(/^#\/?/, "") || "overview");
const start = "s260720";
const firstAll = scenarioIncidents(start);
const DEFAULTS = { glass: "liquid", motion: "full", density: "comfortable" };
let state = {
  view: hashView(), scenario: start, incidents: firstAll, all: firstAll, clock: { t: firstAll.length * 10, max: firstAll.length * 10, playing: false, speed: 10 },
  rail: load("nexus.rail", false), settings: { ...DEFAULTS, ...load("nexus.settings", {}) }, palette: false, sheet: false, rain: 0, rainOn: [], reach: null, demo: null, breaks: [], pick: null, flood: { stage: 0, playing: false }, race: null, online: typeof navigator === "undefined" ? true : navigator.onLine, lang: load("nexus.lang", "en"), installable: false, stock: seedStock(), netInfo: { source: "none", nodes: 0, edges: 0, label: "Loading the road network" }, three: true,
  planner: { from: null, mode: "hospital", to: null, need: "Trauma", med: "ors" }, closures: closureMap(start), conf: 0.9,
  hosp: seedHosp(), shelters: SHELTERS.map((s) => ({ ...s, open: true })), units: UNITS.map((u) => ({ ...u })),
  audit: load("nexus.audit", []), bus: [], alerts: [], net: true, queue: load("nexus.queue", []), lastSos: null,
  role: "Controller", theme: load("nexus.theme", "dark"), layers: { ...LENSES.All }, lens: "All", base: "Matte",
  route: null, selected: null, focus: null, toast: null, district: null, navOpen: false
};

const subs = new Set();
const get = () => state;
function set(patch) { state = { ...state, ...(typeof patch === "function" ? patch(state) : patch) }; subs.forEach((f) => f()); }
export function useStore(sel = (s) => s) { return useSyncExternalStore((f) => { subs.add(f); return () => subs.delete(f); }, () => sel(state)); }
export const getState = get;

const now = () => new Date().toISOString();
const stamp = () => new Date().toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit" });
const ALLOWED = { Citizen: ["sos"], Responder: ["assignment", "map", "responders", "sos", "settings"] };
export const canSee = (role, view) => !ALLOWED[role] || ALLOWED[role].includes(view);
let seq = 0;
function publish(entity, type, extra = {}) {
  const env = { entity, type, geo: extra.geo ?? null, status: extra.status ?? null, capacity: extra.capacity ?? null, confidence: extra.confidence ?? 1, timestamp: now(), source: extra.source ?? "sutradhara" };
  set((s) => ({ bus: [{ ...env, _id: ++seq }, ...s.bus].slice(0, 120) }));
  return env;
}
function audit(action, detail) {
  set((s) => { const a = [{ id: Date.now() + "-" + ++seq, at: now(), actor: s.role, action, detail }, ...s.audit].slice(0, 300); save("nexus.audit", a); return { audit: a }; });
}
let toastTimer;
export function toast(msg) { clearTimeout(toastTimer); set({ toast: msg }); toastTimer = setTimeout(() => set({ toast: null }), 3200); }

export const activeSet = (s = state) => new Set(Object.keys(s.closures).filter((k) => s.closures[k]));

/* ---------- planning on the road network ---------- */
export const P = (id) => ({ key: "node:" + id, name: NODES[id][2], lat: NODES[id][0], lng: NODES[id][1] });
const asPoint = (x) => (typeof x === "string" ? P(x) : x);
/* Illustrative flood spread: water rises around the documented flood-prone localities,
   each at its own stage. Roads inside the water are broken; roads at its edge are slow. */
let floodMemo = { key: "", v: null };
export function floodState(stage = state.flood.stage) {
  const key = net.version + "|" + stage;
  if (floodMemo.key === key) return floodMemo.v;
  const blocked = new Set(), slow = new Set(), pools = [];
  if (stage > 0) FLOOD.forEach((f, i) => {
    const onset = 4 + ((i * 37) % 56);
    const r = Math.max(0, Math.min(1, (stage - onset) / 40)) * 0.6; // km
    if (r <= 0.02) return;
    pools.push({ lat: f.lat, lng: f.lng, r, name: f.n });
    edgesNear(f.lat, f.lng, r, true).forEach((e) => slow.add(e));
    edgesNear(f.lat, f.lng, r * 0.55, true).forEach((e) => { blocked.add(e); slow.delete(e); });
  });
  floodMemo = { key, v: { blocked, slow, pools } };
  return floodMemo.v;
}
/* Everything that is broken or slow right now: documented closures, reported breaks, flood water. */
export function currentSets() {
  const base = resolve([...activeSet()], state.breaks), f = floodState();
  f.blocked.forEach((e) => { base.blocked.add(e); base.slow.delete(e); });
  f.slow.forEach((e) => { if (!base.blocked.has(e)) base.slow.add(e); });
  return base;
}
const netCtx = () => ({ ...currentSets(), conf: state.conf });
const straight = (list, pt) => list.map((x) => ({ x, d: hav(pt.lat, pt.lng, x.lat, x.lng) })).sort((a, b) => a.d - b.d)[0].x;
const withNames = (r) => (r ? { ...r, blocked: [...new Set(r.crosses.map((e) => net.edges[e].name))], slow: r.steps.some((st) => st.slow) } : null);
/* Judge a route that was planned blind against the true state of the roads. */
function judge(path, ctx) {
  if (!path) return null;
  let min = 0; const crosses = [];
  path.edges.forEach((ei) => { const e = net.edges[ei]; if (ctx.blocked.has(ei)) crosses.push(ei); min += ctx.slow.has(ei) ? e.min * TRUE_SLOW : e.min; });
  return withNames({ ...path, min, crosses, steps: path.steps.map((st) => ({ ...st, cut: st.edges.some((e) => ctx.blocked.has(e)), slow: st.edges.some((e) => ctx.slow.has(e)) })) });
}
let memo = { key: "", rows: null };
/* Multi-constraint hospital assignment on the road network: reach, capacity, specialty, inbound load. */
export function rankOnNet(pt, need) {
  const key = [net.version, pt.lat, pt.lng, need, state.conf, state.flood.stage, [...activeSet()].join(), state.breaks.map((b) => b.lat + "," + b.lng).join(), JSON.stringify(state.hosp)].join("|");
  if (memo.key === key) return memo.rows;
  const ctx = netCtx(), src = nearestNode(pt.lat, pt.lng);
  const hard = distFrom(src, { ...ctx, hard: true });
  const rows = HOSPITALS.map((h) => {
    const st = state.hosp[h.id], t = hard[nearestNode(h.lat, h.lng)], reachable = isFinite(t);
    const capable = CAPS[h.lvl].includes(need), free = st.divert ? 0 : st.free;
    const cost = (reachable ? t : 999) + (reachable ? 0 : 240) + (capable ? 0 : 45) + (free > 0 ? 0 : 90) + (free > 0 && free < 3 ? 8 : 0) + st.inbound * 5;
    return { h, min: reachable ? t : Infinity, reachable, capable, free, inbound: st.inbound, cost };
  }).sort((a, b) => a.cost - b.cost);
  memo = { key, rows };
  return rows;
}
export const nearestHospital = (pt) => straight(HOSPITALS, pt);
/* Nearest pharmacy that actually has the medicine, by drive time. */
export function rankPharmacies(pt, med) {
  const hard = distFrom(nearestNode(pt.lat, pt.lng), { ...netCtx(), hard: true });
  return PHARMACIES.map((p) => { const t = hard[nearestNode(p.lat, p.lng)]; return { p, qty: state.stock[p.id][med], min: t, reachable: isFinite(t) }; })
    .sort((a, b) => (b.qty > 0 && b.reachable) - (a.qty > 0 && a.reachable) || a.min - b.min);
}
const unitDist = (pt) => { const d = distFrom(nearestNode(pt.lat, pt.lng), netCtx()); return (u) => d[nearestNode(NODES[u.node][0], NODES[u.node][1])]; };

/* ---------------- navigation and display ---------------- */
export const actions = {
  go(view) { if (!canSee(state.role, view)) view = ALLOWED[state.role][0]; location.hash = "#/" + view; set({ view, navOpen: false, palette: false }); },
  toggleNav() { set((s) => ({ navOpen: !s.navOpen })); },
  setTheme(theme) { save("nexus.theme", theme); set({ theme }); },
  setRole(role) { set({ role }); audit("Role changed", role); toast(`Signed in as ${role.toLowerCase()}`); if (role === "Citizen") actions.go("sos"); else if (role === "Responder") actions.go("assignment"); else if (!canSee(role, state.view) || state.view === "assignment") actions.go("overview"); },
  select(kind, id) { set({ selected: { kind, id } }); },
  closeDrawer() { set({ selected: null }); },
  focus(lat, lng, zoom = 14) { if (lat == null) return; set({ focus: { lat, lng, zoom, n: Date.now() } }); },
  showOnMap(lat, lng, zoom = 14) { if (state.view !== "overview" && state.view !== "map" && state.view !== "routes") actions.go("map"); setTimeout(() => actions.focus(lat, lng, zoom), 60); },
  toggleLayer(k) { set((s) => ({ layers: { ...s.layers, [k]: !s.layers[k] }, lens: "Custom" })); },
  setLens(name) { set({ lens: name, layers: { ...LENSES[name] } }); },
  setBase(base) { set({ base }); },
  pickDistrict(name) { set({ district: name === "Dehradun" ? null : name }); },

  /* ---------------- scenario ---------------- */
  setScenario(id) {
    const all = scenarioIncidents(id);
    set({ scenario: id, incidents: all, all, clock: { ...state.clock, t: all.length * 10, max: all.length * 10, playing: false }, rain: 0, rainOn: [], reach: null, breaks: [], pick: null, flood: { stage: 0, playing: false }, race: null, stock: seedStock(), closures: closureMap(id), route: null, selected: null, units: UNITS.map((u) => ({ ...u })), hosp: seedHosp(), queue: [], lastSos: null });
    publish("scenario", "loaded", { status: SCENARIOS[id].short });
    audit("Scenario loaded", SCENARIOS[id].label);
    toast(`Loaded: ${SCENARIOS[id].label}`);
  },

  /* ---------------- Mārga ---------------- */
  toggleClosure(cid) {
    const on = !state.closures[cid];
    set((s) => ({ closures: { ...s.closures, [cid]: on } }));
    publish("road_segment", "passability", { status: on ? CLOSURES[cid].kind : "open", geo: CLOSURES[cid].at, confidence: state.conf, source: "marga" });
    audit(on ? "Segment marked impassable" : "Segment reopened", CLOSURES[cid].n);
    actions.replan();
  },
  setConf(v) { set({ conf: v }); if (state.route) actions.computeRoute(state.route.req, true); },
  computeRoute(req, quiet, keepPrev) {
    req = { ...req, from: asPoint(req.from), to: req.to ? asPoint(req.to) : null };
    if (req.mode === "node") req.mode = "place";
    const ctx = netCtx();
    let dest = req.to, alloc = null, pharm = null, baseDest = null, baseFull = false;
    if (req.mode === "hospital") {
      alloc = rankOnNet(req.from, req.need);
      dest = { ...alloc[0].h, name: alloc[0].h.n }; const b = nearestHospital(req.from); baseDest = { ...b, name: b.n };
    } else if (req.mode === "pharmacy") {
      pharm = rankPharmacies(req.from, req.med);
      if (!(pharm[0].qty > 0 && pharm[0].reachable)) { toast("No reachable pharmacy has this medicine in stock"); set({ route: null }); return; }
      dest = { ...pharm[0].p, name: pharm[0].p.n }; const b = straight(PHARMACIES, req.from); baseDest = { ...b, name: b.n }; baseFull = state.stock[b.id][req.med] <= 0;
    } else if (req.mode === "shelter") {
      const d = distFrom(nearestNode(req.from.lat, req.from.lng), { ...ctx, hard: true });
      const t = (x) => d[nearestNode(x.lat, x.lng)];
      const open = state.shelters.filter((x) => x.open && x.occ < x.cap).sort((a, b) => t(a) - t(b));
      if (!open.length) { toast("No shelter has space. Open a site or free places first."); return; }
      dest = { ...open[0], name: open[0].n }; const b = straight(state.shelters, req.from); baseDest = { ...b, name: b.n }; baseFull = !(b.open && b.occ < b.cap);
    }
    if (!dest) return;
    const A = nearestNode(req.from.lat, req.from.lng);
    const nexus = withNames(shortest(A, nearestNode(dest.lat, dest.lng), ctx));
    const baseline = judge(shortest(A, nearestNode((baseDest || dest).lat, (baseDest || dest).lng), {}), ctx);
    const old = state.route;
    const prev = keepPrev && old && old.nexus && nexus && Math.abs(old.nexus.km - nexus.km) > 0.01 ? { coords: old.nexus.coords, km: old.nexus.km, min: old.nexus.min, dest: old.destLabel } : null;
    set({ race: null, route: { req, nexus, baseline, dest, destLabel: dest.name, baseLabel: (baseDest || dest).name, baseFull, alloc, pharm, prev, approved: false, n: Date.now() } });
    if (!quiet) { publish("route", "computed", { geo: [req.from.lat, req.from.lng], status: !nexus || nexus.blocked.length ? "cut_off" : "passable", confidence: ctx.conf, source: "marga" }); audit("Route computed", `${req.from.name} to ${dest.name}`); }
  },
  approveRoute() {
    const r = state.route; if (!r) return;
    const d = unitDist(r.req.from);
    const unit = state.units.filter((u) => u.status === "Available" && u.type === "Ambulance").sort((a, b) => d(a) - d(b))[0];
    if (!unit) { toast("No ambulance is available. Release a unit first."); return; }
    set((s) => ({
      route: { ...s.route, approved: true, unit: unit.id },
      units: s.units.map((u) => (u.id === unit.id ? { ...u, status: "On mission", task: `${r.req.from.name} to ${r.destLabel}` } : u)),
      hosp: r.alloc ? { ...s.hosp, [r.alloc[0].h.id]: { ...s.hosp[r.alloc[0].h.id], inbound: s.hosp[r.alloc[0].h.id].inbound + 1 } } : s.hosp,
      shelters: r.req.mode === "shelter" ? s.shelters.map((x) => (x.n === r.destLabel ? { ...x, occ: Math.min(x.cap, x.occ + (r.req.people || 10)) } : x)) : s.shelters
    }));
    publish("assignment", "dispatched", { geo: [r.req.from.lat, r.req.from.lng], status: unit.id, source: "dispatch" });
    audit("Route approved and dispatched", `${unit.id}: ${r.req.from.name} to ${r.destLabel}`);
    toast(`Dispatched ${unit.id} to ${r.req.from.name}`);
  },
  /* ---------- A to B planner, breaks, pharmacy stock ---------- */
  setPlanner(patch) { set((s) => ({ planner: { ...s.planner, ...patch }, pick: null })); actions.plan(); },
  plan(quiet, keepPrev) { const p = state.planner; if (!p.from || (p.mode === "place" && !p.to)) return; actions.computeRoute({ from: p.from, mode: p.mode, to: p.mode === "place" ? p.to : null, need: p.need, med: p.med, people: 10 }, quiet, keepPrev); },
  replan() { if (state.route) actions.computeRoute(state.route.req, true, true); },
  setPick(pick) { set({ pick }); if (pick) toast(pick === "break" ? "Click a road on the map to break it" : `Click the map to set ${pick === "a" ? "the start" : "the destination"}`); },
  mapClick(lat, lng) {
    const pick = state.pick; if (!pick) return false;
    if (pick === "break") { actions.addBreak(lat, lng); return true; }
    const pt = { key: `map:${lat.toFixed(5)},${lng.toFixed(5)}`, name: `Map point ${lat.toFixed(4)}, ${lng.toFixed(4)}`, lat, lng };
    actions.setPlanner(pick === "a" ? { from: pt } : { to: pt, mode: "place" });
    return true;
  },
  addBreak(lat, lng) {
    const e = nearestEdge(lat, lng);
    if (!e || e.dist > 0.6) { toast("No road within 600 m of that point"); return; }
    const [la, ln] = edgeMid(e);
    set((s) => ({ breaks: [...s.breaks, { lat: la, lng: ln, name: e.name }], pick: null }));
    publish("road_segment", "passability", { status: "closed", geo: [la, ln], confidence: state.conf, source: "marga/report" });
    audit("Road reported broken", e.name);
    actions.replan(); toast(`${e.name} marked broken. Rerouting.`);
  },
  breakStep(i) { const st = state.route?.nexus?.steps[i]; if (!st) return; const e = net.edges[st.edges[Math.floor(st.edges.length / 2)]]; const [la, ln] = edgeMid(e); actions.addBreak(la, ln); },
  removeBreak(i) { const b = state.breaks[i]; set((s) => ({ breaks: s.breaks.filter((_, k) => k !== i) })); audit("Road repaired", b.name); actions.replan(); },
  clearBreaks() { if (!state.breaks.length) return; set({ breaks: [] }); audit("All reported breaks cleared", ""); actions.replan(); toast("All reported breaks cleared"); },
  /* ---------- flood that spreads ---------- */
  setFlood(stage) { set((s) => ({ flood: { ...s.flood, stage } })); actions.replan(); },
  playFlood(on) { if (on && state.flood.stage >= 100) set((s) => ({ flood: { ...s.flood, stage: 0 } })); set((s) => ({ flood: { ...s.flood, playing: on } })); if (!on) actions.commitFlood(); },
  commitFlood() { const f = floodState(); publish("flood", "stage", { status: `${Math.round(state.flood.stage)}%`, confidence: 0.5, source: "purvasuchana" }); audit("Flood stage set", `${Math.round(state.flood.stage)}%: ${f.pools.length} localities under water, ${f.blocked.size} road segments broken`); },

  /* ---------- the race: Nexus against the nearest-hospital baseline, same start, same moment ---------- */
  startRace() {
    const r = state.route;
    if (!r || !r.nexus || !r.baseline) { toast("Plan a route first"); return; }
    const ctx = netCtx();
    const track = (nodes, edges, t0 = 0) => {
      const pts = [{ lat: net.lat[nodes[0]], lng: net.lng[nodes[0]], t: t0 }];
      let t = t0;
      edges.forEach((ei, k) => { const e = net.edges[ei]; t += ctx.slow.has(ei) ? e.min * TRUE_SLOW : e.min; pts.push({ lat: net.lat[nodes[k + 1]], lng: net.lng[nodes[k + 1]], t }); });
      return pts;
    };
    const go = (pts, events, from, to, why, lost) => {
      const last = pts[pts.length - 1];
      events.push({ t: last.t, text: why });
      const re = shortest(from, to, { ...ctx, hard: true });
      if (!re) { events.push({ t: last.t, text: "No open road leads there. Stuck." }); return false; }
      pts.push({ ...last, t: last.t + lost });
      track(re.nodes, re.edges, last.t + lost).slice(1).forEach((x) => pts.push(x));
      return true;
    };
    const nexus = { pts: track(r.nexus.nodes, r.nexus.edges), events: [], ok: r.nexus.blocked.length === 0 };
    if (!nexus.ok) nexus.events.push({ t: 0, text: "The destination is cut off for everyone." });
    const b = r.baseline, events = [];
    const cut = b.edges.findIndex((ei) => ctx.blocked.has(ei));
    let pts, ok = true, end = b.nodes[b.nodes.length - 1];
    if (cut < 0) pts = track(b.nodes, b.edges);
    else { pts = track(b.nodes.slice(0, cut + 1), b.edges.slice(0, cut)); ok = go(pts, events, b.nodes[cut], end, `Stopped at ${net.edges[b.edges[cut]].name}: the road is broken. Turning back.`, 3); }
    const row = r.alloc && r.alloc.find((x) => x.h.n === r.baseLabel);
    const turnedAway = r.baseLabel !== r.destLabel && (row ? !row.capable || row.free <= 0 : r.baseFull);
    if (ok && turnedAway) ok = go(pts, events, end, nearestNode(r.dest.lat, r.dest.lng), `${r.baseLabel} cannot take this case. Sent on to ${r.destLabel}.`, 5);
    const base = { pts, events, ok };
    nexus.total = nexus.ok ? nexus.pts[nexus.pts.length - 1].t : Infinity;
    base.total = ok ? pts[pts.length - 1].t : Infinity;
    const longest = Math.max(...[nexus.pts[nexus.pts.length - 1].t, pts[pts.length - 1].t], 1);
    set({ race: { t0: performance.now(), rate: longest / 12, nexus, base, n: Date.now() } });
    publish("race", "started", { geo: [r.req.from.lat, r.req.from.lng], source: "evaluation" });
    audit("Race run", `${r.req.from.name}: Nexus ${isFinite(nexus.total) ? Math.round(nexus.total) + " min" : "cut off"}, baseline ${isFinite(base.total) ? Math.round(base.total) + " min" : "stuck"}`);
  },
  stopRace() { set({ race: null }); },

  /* ---------- language, install ---------- */
  setLang(lang) { save("nexus.lang", lang); set({ lang }); },
  async install() { const ev = window.__nexusInstall; if (!ev) { toast("Use the browser menu: Install app, or Add to Home Screen"); return; } ev.prompt(); await ev.userChoice; window.__nexusInstall = null; set({ installable: false }); },

  toggle3d() { set((s) => ({ three: !s.three })); },
  adjustStock(pid, mid, delta) { set((s) => ({ stock: { ...s.stock, [pid]: { ...s.stock[pid], [mid]: Math.max(0, s.stock[pid][mid] + delta) } } })); const p = PHARMACIES.find((x) => x.id === pid); publish("pharmacy", "stock", { geo: [p.lat, p.lng], status: mid, capacity: { qty: state.stock[pid][mid] }, source: "arogya" }); },
  setStock(pid, mid, qty) { set((s) => ({ stock: { ...s.stock, [pid]: { ...s.stock[pid], [mid]: qty } } })); const p = PHARMACIES.find((x) => x.id === pid); const m = MEDICINES.find((x) => x.id === mid); publish("pharmacy", "stock", { geo: [p.lat, p.lng], status: mid, capacity: { qty }, source: "arogya" }); audit(qty ? "Pharmacy restocked" : "Pharmacy out of stock", `${p.n}: ${m.n}`); },
  reloadNet() { set({ netInfo: { ...state.netInfo, label: "Loading the road network" } }); loadRoadnet(); },
  clearRoute() { set({ route: null, race: null }); },

  /* ---------------- incidents ---------------- */
  ackIncident(id) { set((s) => ({ incidents: s.incidents.map((i) => (i.id === id && i.status === "Open" ? { ...i, status: "Acknowledged", log: [...(i.log || []), ["Acknowledged", stamp()]] } : i)) })); const i = state.incidents.find((x) => x.id === id); publish("incident", "acknowledged", { geo: [i.lat, i.lng] }); audit("Incident acknowledged", i.n); toast("Acknowledged"); },
  dispatchIncident(id) {
    const i = state.incidents.find((x) => x.id === id);
    const at = { name: i.place, lat: i.lat, lng: i.lng };
    const d = unitDist(at);
    const wantTeam = /Flood|Cloudburst|Rescue|Landslide/.test(i.t);
    const free = state.units.filter((u) => u.status === "Available");
    const pool = free.filter((u) => (wantTeam ? u.type !== "Ambulance" : u.type === "Ambulance"));
    const unit = (pool.length ? pool : free).sort((a, b) => d(a) - d(b))[0];
    if (!unit) { toast("No unit is available. Release a unit first."); return; }
    set((s) => ({ incidents: s.incidents.map((x) => (x.id === id ? { ...x, status: "Unit assigned", unit: unit.id, log: [...(x.log || []), [`${unit.id} assigned`, stamp()]] } : x)), units: s.units.map((u) => (u.id === unit.id ? { ...u, status: "On mission", task: i.n } : u)) }));
    actions.computeRoute({ from: P(unit.node), mode: "place", to: at, need: "General" }, true);
    publish("assignment", "dispatched", { geo: [i.lat, i.lng], status: unit.id, source: "dispatch" });
    audit("Unit dispatched", `${unit.id} to ${i.n}`);
    toast(`Dispatched ${unit.id}`);
  },
  closeIncident(id) {
    const i = state.incidents.find((x) => x.id === id);
    set((s) => ({ incidents: s.incidents.map((x) => (x.id === id ? { ...x, status: "Closed", log: [...(x.log || []), ["Closed", stamp()]] } : x)), units: s.units.map((u) => (u.id === i.unit ? { ...u, status: "Available", task: "" } : u)) }));
    publish("incident", "closed", { geo: [i.lat, i.lng] }); audit("Incident closed", i.n); toast("Incident closed");
  },

  /* ---------------- Ārogya, Āśraya, Rakṣaka ---------------- */
  toggleDivert(hid) { set((s) => ({ hosp: { ...s.hosp, [hid]: { ...s.hosp[hid], divert: !s.hosp[hid].divert } } })); const h = HOSPITALS.find((x) => x.id === hid); publish("hospital", "capacity", { geo: [h.lat, h.lng], status: state.hosp[hid].divert ? "diverting" : "accepting", capacity: { free: state.hosp[hid].free }, source: "arogya" }); audit(state.hosp[hid].divert ? "Hospital set to divert" : "Hospital accepting again", h.n); },
  adjustBeds(hid, delta) { set((s) => { const x = s.hosp[hid]; return { hosp: { ...s.hosp, [hid]: { ...x, free: Math.max(0, Math.min(x.cap, x.free + delta)) } } }; }); const h = HOSPITALS.find((x) => x.id === hid); publish("hospital", "capacity", { geo: [h.lat, h.lng], capacity: { free: state.hosp[hid].free }, source: "arogya" }); },
  adjustShelter(id, delta) { set((s) => ({ shelters: s.shelters.map((x) => (x.id === id ? { ...x, occ: Math.max(0, Math.min(x.cap, x.occ + delta)) } : x)) })); const sh = state.shelters.find((x) => x.id === id); publish("shelter", "occupancy", { geo: [sh.lat, sh.lng], capacity: { cap: sh.cap, occ: sh.occ }, status: sh.occ >= sh.cap ? "full" : "open", source: "ashraya" }); if (sh.occ >= sh.cap) audit("Shelter reached capacity", sh.n); },
  toggleShelter(id) { set((s) => ({ shelters: s.shelters.map((x) => (x.id === id ? { ...x, open: !x.open } : x)) })); const sh = state.shelters.find((x) => x.id === id); publish("shelter", "status", { geo: [sh.lat, sh.lng], status: sh.open ? "open" : "closed", source: "ashraya" }); audit(sh.open ? "Shelter opened" : "Shelter closed", sh.n); },
  setUnit(id, status) { set((s) => ({ units: s.units.map((u) => (u.id === id ? { ...u, status, task: status === "Available" ? "" : u.task || "Manual tasking" } : u)) })); const u = state.units.find((x) => x.id === id); publish("unit", "status", { geo: NODES[u.node].slice(0, 2), status, source: "rakshaka" }); audit("Unit status changed", `${id}: ${status}`); },

  /* ---------------- Pūrvasūchanā ---------------- */
  issueAlert({ area, severity, message }) {
    const env = publish("public_alert", "cap_alert", { status: severity, source: "purvasuchana" });
    set((s) => ({ alerts: [{ id: Date.now(), area, severity, message, at: env.timestamp, by: s.role }, ...s.alerts] }));
    audit("Public alert issued", `${severity}: ${area}`); toast("Alert issued");
  },

  /* ---------------- Āhvāna ---------------- */
  setNet(net) { set({ net }); publish("telecom", "connectivity", { status: net ? "up" : "down", source: "pranadhara" }); audit(net ? "Network restored" : "Network cut (test)", "Citizen device"); if (net && state.online && state.queue.length) actions.flushQueue("network restored"); },
  sendSos(form) {
    const place = FLOOD.find((f) => f.id === form.place);
    const sos = { id: "SOS-" + String(Date.now()).slice(-5), ...form, placeName: place.n, node: place.node, lat: place.lat, lng: place.lng, raised: now() };
    if (!state.net || !state.online) { set((s) => ({ queue: [...s.queue, sos], lastSos: { sos, stage: "held" } })); save("nexus.queue", state.queue); toast("No network. SOS held on the device."); return; }
    actions.deliverSos(sos, "direct");
  },
  flushQueue(via = "peer relay") { const q = state.queue; set({ queue: [] }); save("nexus.queue", []); q.forEach((s) => actions.deliverSos(s, via)); if (q.length) toast(`${q.length} held SOS delivered via ${via}`); },
  deliverSos(sos, via) {
    const need = sos.injured === "Yes" ? "Trauma" : "General";
    const at = { key: "flood:" + sos.place, name: sos.placeName, lat: sos.lat, lng: sos.lng };
    const alloc = rankOnNet(at, need);
    const base = { h: nearestHospital(at) };
    const d = unitDist(at);
    const unit = state.units.filter((u) => u.status === "Available" && u.type === "Ambulance").sort((a, b) => d(a) - d(b))[0] || null;
    const inc = { id: sos.id, t: "SOS", n: `SOS: ${sos.hazard}, ${sos.people} people`, place: sos.placeName, sev: sos.injured === "Yes" ? "High" : "Medium", lat: sos.lat, lng: sos.lng, node: sos.node, d: `Raised from a citizen device. Delivered by ${via}.`, s: [], sim: true, status: unit ? "Unit assigned" : "Open", unit: unit?.id || null, when: "just now", log: [["Raised on a citizen device", stamp()], [`Delivered by ${via}`, stamp()], ...(unit ? [[`${unit.id} assigned, going to ${alloc[0].h.n}`, stamp()]] : [])] };
    set((s) => ({
      incidents: [inc, ...s.incidents],
      units: unit ? s.units.map((u) => (u.id === unit.id ? { ...u, status: "On mission", task: inc.n } : u)) : s.units,
      hosp: { ...s.hosp, [alloc[0].h.id]: { ...s.hosp[alloc[0].h.id], inbound: s.hosp[alloc[0].h.id].inbound + 1 } },
      lastSos: { sos, stage: "dispatched", via, hospital: alloc[0], baseline: base, unit: unit?.id || null }
    }));
    set((s) => ({ planner: { ...s.planner, from: at, mode: "hospital", need } }));
    actions.computeRoute({ from: at, mode: "hospital", need }, true);
    publish("sos", "request", { geo: [sos.lat, sos.lng], status: via, capacity: { people: sos.people }, source: "ahvana" });
    publish("assignment", "dispatched", { geo: [sos.lat, sos.lng], status: unit?.id || "queued", source: "dispatch" });
    audit("SOS received and assigned", `${sos.placeName}: ${alloc[0].h.n}${unit ? ", " + unit.id : ""}`);
  },
  injectLoad(n = 5) {
    for (let i = 0; i < n; i++) {
      const f = FLOOD[Math.floor(Math.random() * FLOOD.length)];
      actions.deliverSos({ id: "SOS-" + Math.floor(10000 + Math.random() * 89999), place: f.id, placeName: f.n, node: f.node, lat: f.lat, lng: f.lng, hazard: ["Flood", "Landslide", "Trapped", "Medical"][i % 4], people: 1 + Math.floor(Math.random() * 6), injured: Math.random() < 0.4 ? "Yes" : "No", raised: now() }, "direct");
    }
    toast(`${n} simulated SOS requests injected`);
  },

  /* ---------------- shell, settings, replay clock ---------------- */
  setSetting(k, v) { set((s) => { const settings = { ...s.settings, [k]: v }; save("nexus.settings", settings); return { settings }; }); },
  toggleRail() { set((s) => { save("nexus.rail", !s.rail); return { rail: !s.rail }; }); },
  setPalette(v) { set({ palette: v }); },
  setSheet(v) { set({ sheet: v }); },
  setDemo(demo) { set({ demo }); },
  setClock(t) {
    const s = state;
    const cur = Object.fromEntries(s.incidents.map((i) => [i.id, i]));
    const all = s.all.map((i) => cur[i.id] || i);
    set({ all, clock: { ...s.clock, t }, incidents: [...s.incidents.filter((i) => i.sim), ...all.filter((i) => i.at <= t)] });
  },
  play(on) { if (on && state.clock.t >= state.clock.max) actions.setClock(0); set((s) => ({ clock: { ...s.clock, playing: on } })); },
  setSpeed(speed) { set((s) => ({ clock: { ...s.clock, speed } })); },

  /* ---------------- Pūrvasūchanā what-if, reach, bulk actions ---------------- */
  setRain(mm) {
    const want = inferFromRain(mm);
    set((s) => {
      const closures = { ...s.closures };
      const rainOn = s.rainOn.filter((c) => { if (want.includes(c)) return true; closures[c] = false; return false; });
      want.forEach((c) => { if (!closures[c]) { closures[c] = true; rainOn.push(c); } });
      return { rain: mm, closures, rainOn };
    });
    actions.replan();
  },
  commitRain() { publish("rainfall", "what_if", { status: `${state.rain} mm`, confidence: 0.5, source: "purvasuchana" }); audit("Rainfall what-if set", `${state.rain} mm, ${state.rainOn.length} segments inferred`); },
  setReach(id) { set({ reach: id }); if (id) { const h = HOSPITALS.find((x) => x.id === id); set({ selected: null }); actions.showOnMap(h.lat, h.lng, 11.5); } },
  ackAll() { const n = state.incidents.filter((i) => i.status === "Open").length; if (!n) { toast("Nothing is waiting for acknowledgement"); return; } set((s) => ({ incidents: s.incidents.map((i) => (i.status === "Open" ? { ...i, status: "Acknowledged", log: [...(i.log || []), ["Acknowledged", stamp()]] } : i)) })); audit("Incidents acknowledged", `${n} in one action`); toast(`${n} acknowledged`); },
  reassign(id) {
    const i = state.incidents.find((x) => x.id === id); if (!i?.unit) return;
    const old = i.unit;
    set((s) => ({ units: s.units.map((u) => (u.id === old ? { ...u, status: "Available", task: "" } : u)), incidents: s.incidents.map((x) => (x.id === id ? { ...x, unit: null, status: "Acknowledged" } : x)) }));
    set((s) => ({ units: s.units.map((u) => (u.id === old ? { ...u, status: "Held" } : u)) }));
    actions.dispatchIncident(id);
    set((s) => ({ units: s.units.map((u) => (u.id === old ? { ...u, status: "Available" } : u)) }));
  },
  returnAll() { set((s) => ({ units: s.units.map((u) => ({ ...u, status: "Available", task: "" })) })); publish("unit", "status", { status: "all_available", source: "rakshaka" }); audit("All units returned to base", `${state.units.length} units`); toast("All units are available"); },
  unitStep(id, step) { set((s) => ({ units: s.units.map((u) => (u.id === id ? (step === "Free" ? { ...u, status: "Available", task: "", step: null } : { ...u, step }) : u)) })); publish("unit", "progress", { status: step, source: "rakshaka" }); audit("Responder update", `${id}: ${step}`); toast(`${id}: ${step.toLowerCase()}`); },
  resetAll() { try { localStorage.removeItem("nexus.audit"); localStorage.removeItem("nexus.settings"); localStorage.removeItem("nexus.rail"); } catch { /* ignore */ } set({ audit: [], bus: [], alerts: [], settings: { ...DEFAULTS }, rail: false, net: true, role: "Controller" }); actions.setScenario(start); toast("Everything reset"); },

  clearAudit() { save("nexus.audit", []); set({ audit: [] }); toast("Audit trail cleared"); }
};

window.addEventListener("hashchange", () => { const v = hashView(); if (v !== state.view) set({ view: v }); });

/* Replay clock: sim minutes per real second = speed. */
let floodTick = 0;
setInterval(() => {
  const f = state.flood;
  if (f.playing) {
    const stage = Math.min(100, f.stage + 1.25);
    set((s) => ({ flood: { stage, playing: stage < 100 } }));
    if (++floodTick % 3 === 0 || stage >= 100) actions.replan();
    if (stage >= 100) actions.commitFlood();
  }
  const c = state.clock;
  if (!c.playing) return;
  const t = Math.min(c.max, c.t + c.speed * 0.25);
  actions.setClock(t);
  if (t >= c.max) set((s) => ({ clock: { ...s.clock, playing: false } }));
}, 250);

/* Road network: usable at once on the built-in graph, upgraded when real roads arrive. */
onNet(() => { set({ netInfo: describe() }); memo = { key: "", rows: null }; if (state.route) actions.computeRoute(state.route.req, true); });
loadRoadnet();

/* The device really going offline or coming back (Wi-Fi off, flight mode). */
if (typeof window !== "undefined") {
  window.addEventListener("offline", () => { set({ online: false }); toast("This device is offline. An SOS will be held until the network returns."); });
  window.addEventListener("online", () => { set({ online: true }); if (state.net && state.queue.length) actions.flushQueue("the network returning"); else toast("Back online"); });
  window.addEventListener("beforeinstallprompt", (e) => { e.preventDefault(); window.__nexusInstall = e; set({ installable: true }); });
  setTimeout(() => { if (state.queue.length && state.net && state.online) actions.flushQueue("the network returning"); }, 2500);
}
