/* The map: MapLibre GL with real 3D terrain (AWS open elevation tiles),
   the road network every route is planned on, and every record as a marker. */
import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Plus, Minus, Maximize, Globe2, Layers, X, Ruler, Expand, Shrink, Mountain, Unlink, Play, Pause, Waves } from "lucide-react";
import { useStore, actions, LENSES, currentSets, floodState } from "../state/store.js";
import { HOSPITALS, PHARMACIES, FLOOD, SLIDES, INFRA, RIVERS, NODES, CLOSURES } from "../data/index.js";
import { net, nearestNode, distFrom, hav } from "../engine/roadnet.js";
import { Seg, Switch } from "./ui.jsx";

const CITY = [[77.93, 30.245], [78.19, 30.44]];
const DISTRICT = [[77.62, 30.03], [78.36, 30.72]];
const sub = (hosts, f) => hosts.map(f);
const BASES = {
  Matte: (t) => ({ tiles: sub(["a", "b", "c", "d"], (h) => `https://${h}.basemaps.cartocdn.com/${t === "light" ? "light_all" : "dark_all"}/{z}/{x}/{y}@2x.png`), credit: "© OpenStreetMap contributors, © CARTO", sat: -0.25, bright: t === "light" ? 1 : 0.92 }),
  Terrain: (t) => ({ tiles: sub(["a", "b", "c"], (h) => `https://${h}.tile.opentopomap.org/{z}/{x}/{y}.png`), credit: "© OpenStreetMap contributors, SRTM; style © OpenTopoMap (CC-BY-SA)", sat: -0.45, bright: t === "light" ? 1 : 0.62 }),
  Satellite: (t) => ({ tiles: ["https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"], credit: "Imagery © Esri", sat: -0.3, bright: t === "light" ? 1 : 0.78 })
};
const DEM = "https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png";
const LAYER_LIST = [
  ["incidents", "Incidents", "--crit"], ["closures", "Broken and slow roads", "--crit"], ["graph", "Road network and links", "--tx3"], ["flood", "Flood-prone localities", "--flood"],
  ["corridors", "River corridors", "--flood"], ["slides", "Landslide points", "--slide"], ["hospitals", "Hospitals", "--ok"], ["pharmacies", "Pharmacies", "--tx"],
  ["shelters", "Shelters", "--tx2"], ["units", "Response units", "--tx"], ["buildings", "Unsafe structures", "--crit"]
];
const tok = (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim();
const FC = (features = []) => ({ type: "FeatureCollection", features });
const line = (coords, props = {}) => ({ type: "Feature", properties: props, geometry: { type: "LineString", coordinates: coords } });
const multi = (lines, props = {}) => ({ type: "Feature", properties: props, geometry: { type: "MultiLineString", coordinates: lines } });
const ll = (c) => c.map(([la, ln]) => [ln, la]);
const ring = (lat, lng, km) => { const pts = []; for (let i = 0; i <= 40; i++) { const a = (i / 40) * 2 * Math.PI; pts.push([lng + (km / (111.32 * Math.cos((lat * Math.PI) / 180))) * Math.cos(a), lat + (km / 110.57) * Math.sin(a)]); } return pts; };
/* Position along a timed track at sim-minute t. */
function along(pts, t) {
  if (t <= pts[0].t) return pts[0];
  const last = pts[pts.length - 1];
  if (t >= last.t) return last;
  let lo = 0, hi = pts.length - 1;
  while (hi - lo > 1) { const mid = (lo + hi) >> 1; if (pts[mid].t <= t) lo = mid; else hi = mid; }
  const a = pts[lo], b = pts[hi], k = (t - a.t) / (b.t - a.t || 1);
  return { lat: a.lat + (b.lat - a.lat) * k, lng: a.lng + (b.lng - a.lng) * k };
}
const edgeLine = (ei) => { const e = net.edges[ei]; return [[net.lng[e.a], net.lat[e.a]], [net.lng[e.b], net.lat[e.b]]]; };

function pin(cls, inner, tip, onClick) {
  const d = document.createElement("div");
  d.className = "mk " + cls;
  d.innerHTML = `<i>${inner || ""}</i>`;
  d.title = tip;
  if (onClick) { d.tabIndex = 0; d.setAttribute("role", "button"); d.addEventListener("click", onClick); d.addEventListener("keydown", (e) => { if (e.key === "Enter") onClick(); }); }
  return d;
}

export default function MapView({ fitRoute = false, panelOpen = true, showNet = false }) {
  const el = useRef(null);
  const map = useRef(null);
  const marks = useRef({});
  const mover = useRef(null);
  const racer = useRef(null);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [showLayers, setShowLayers] = useState(panelOpen);
  const [full, setFull] = useState(false);
  const [measure, setMeasure] = useState(null);
  const s = useStore();
  const { layers, lens, base, theme, incidents, closures, breaks, route, focus, units, shelters, hosp, reach, conf, netInfo, three, pick, planner, settings, flood, race } = s;
  const measuring = useRef(false); measuring.current = measure !== null;

  /* ---------- create once ---------- */
  useEffect(() => {
    let m;
    try {
      const b = BASES.Matte(document.documentElement.dataset.theme);
      m = new maplibregl.Map({
        container: el.current, attributionControl: false, maxPitch: 78, center: [78.06, 30.335], zoom: 11.1,
        style: {
          version: 8,
          sources: { base: { type: "raster", tiles: b.tiles, tileSize: 256 }, dem: { type: "raster-dem", tiles: [DEM], encoding: "terrarium", tileSize: 256, maxzoom: 13 }, hill: { type: "raster-dem", tiles: [DEM], encoding: "terrarium", tileSize: 256, maxzoom: 13 } },
          layers: [{ id: "bg", type: "background", paint: { "background-color": tok("--panel") || "#17181c" } }, { id: "base", type: "raster", source: "base" }, { id: "hill", type: "hillshade", source: "hill", paint: { "hillshade-exaggeration": 0.5, "hillshade-shadow-color": "#000000", "hillshade-highlight-color": "#ffffff", "hillshade-accent-color": "#000000" } }]
        }
      });
    } catch { setFailed(true); return; }
    map.current = m;
    m.once("style.load", () => {
      const src = (id) => m.addSource(id, { type: "geojson", data: FC() });
      ["corr", "water", "net", "links", "reach", "cut", "slowed", "prev", "baseline", "route", "measure"].forEach(src);
      const L = (id, source, paint, layout = {}, filter) => m.addLayer({ id, type: "line", source, paint, layout: { "line-cap": "round", "line-join": "round", ...layout }, ...(filter ? { filter } : {}) });
      L("corr-wide", "corr", { "line-width": 12, "line-opacity": 0.16 }, {}, ["==", ["get", "fl"], 1]);
      L("corr", "corr", { "line-width": ["case", ["==", ["get", "w"], 2], 2.4, 1.4], "line-opacity": 0.85 });
      m.addLayer({ id: "water", type: "fill", source: "water", paint: { "fill-opacity": 0.34 } });
      m.addLayer({ id: "water-edge", type: "line", source: "water", paint: { "line-width": 1.2, "line-opacity": 0.8 } });
      L("net", "net", { "line-width": ["match", ["get", "hw"], "motorway", 2.2, "trunk", 2.2, "primary", 1.8, "secondary", 1.3, 0.8], "line-opacity": 0.55 });
      L("links", "links", { "line-width": 1, "line-opacity": 0.5, "line-dasharray": [1, 2.5] });
      L("reach", "reach", { "line-width": 2.6, "line-opacity": 0.9 });
      L("slowed", "slowed", { "line-width": 5, "line-dasharray": [0.4, 1.8] });
      L("cut", "cut", { "line-width": 5, "line-dasharray": [0.4, 1.8] });
      L("prev", "prev", { "line-width": 3, "line-opacity": 0.55, "line-dasharray": [1.5, 2] });
      L("baseline", "baseline", { "line-width": 3, "line-opacity": 0.9, "line-dasharray": [2, 2.2] });
      L("route-case", "route", { "line-width": 9, "line-opacity": 0.35, "line-color": "#000000" });
      L("route", "route", { "line-width": 4.6 });
      L("measure", "measure", { "line-width": 2, "line-dasharray": [2, 2] });
      setReady(true);
    });
    m.on("error", () => { /* tile and terrain fetch failures are expected offline */ });
    m.on("click", (e) => {
      if (e.originalEvent.target.closest && e.originalEvent.target.closest(".mk")) return;
      if (measuring.current) { setMeasure((p) => [...(p || []), [e.lngLat.lat, e.lngLat.lng]]); return; }
      actions.mapClick(e.lngLat.lat, e.lngLat.lng);
    });
    const ro = new ResizeObserver(() => m.resize());
    ro.observe(el.current);
    return () => { ro.disconnect(); cancelAnimationFrame(mover.current); cancelAnimationFrame(racer.current); m.remove(); map.current = null; };
  }, []);

  const setData = (id, data) => map.current?.getSource(id)?.setData(data);
  const sync = (key, list) => { (marks.current[key] || []).forEach((x) => x.remove()); marks.current[key] = list.map(([lat, lng, node]) => new maplibregl.Marker({ element: node }).setLngLat([lng, lat]).addTo(map.current)); };

  /* ---------- base tiles, colours, 3D ---------- */
  useEffect(() => {
    if (!ready) return;
    const m = map.current, b = BASES[base](theme);
    m.getSource("base").setTiles(b.tiles);
    m.setPaintProperty("base", "raster-saturation", b.sat);
    m.setPaintProperty("base", "raster-brightness-max", b.bright);
    m.setPaintProperty("bg", "background-color", tok("--panel"));
    const c = { "corr-wide": "--flood", corr: "--flood", net: "--tx3", links: "--tx3", slowed: "--slide", cut: "--crit", prev: "--tx3", baseline: "--tx2", route: "--brass", measure: "--tx" };
    Object.entries(c).forEach(([id, v]) => m.setPaintProperty(id, "line-color", tok(v)));
    m.setPaintProperty("water", "fill-color", tok("--flood"));
    m.setPaintProperty("water-edge", "line-color", tok("--flood"));
    m.setPaintProperty("reach", "line-color", ["match", ["get", "band"], "ok", tok("--ok"), "brass", tok("--brass"), "slide", tok("--slide"), tok("--crit")]);
  }, [ready, base, theme]);
  useEffect(() => {
    if (!ready) return;
    const m = map.current;
    try { m.setTerrain(three ? { source: "dem", exaggeration: 1.6 } : null); } catch { /* terrain needs the elevation tiles */ }
    m.easeTo({ pitch: three ? 58 : 0, bearing: three ? -18 : 0, duration: settings.motion === "full" ? 1200 : 0 });
  }, [ready, three]);

  /* ---------- static overlays ---------- */
  useEffect(() => {
    if (!ready) return;
    setData("corr", FC(layers.corridors ? RIVERS.map((r) => line(ll(r.p), { fl: r.fl ? 1 : 0, w: r.w || 1 })) : []));
    sync("flood", layers.flood ? FLOOD.map((f) => [f.lat, f.lng, pin("mk-flood", "", f.n, () => actions.select("flood", f.id))]) : []);
    sync("slides", layers.slides ? SLIDES.map((f) => [f.lat, f.lng, pin("mk-slide", "", f.n, () => actions.select("slide", f.id))]) : []);
    sync("buildings", layers.buildings ? INFRA.filter((i) => i.kind === "Building" && i.lat).map((f) => [f.lat, f.lng, pin("mk-bld", "", f.n, () => actions.select("infra", f.id))]) : []);
    sync("pharm", layers.pharmacies ? PHARMACIES.map((p) => [p.lat, p.lng, pin("mk-ph", "", p.n, () => actions.select("pharmacy", p.id))]) : []);
  }, [ready, layers.corridors, layers.flood, layers.slides, layers.buildings, layers.pharmacies]);

  /* ---------- the road network, and every point linked into it ---------- */
  useEffect(() => {
    if (!ready) return;
    if (!layers.graph && !showNet) { setData("net", FC()); setData("links", FC()); return; }
    const by = {};
    net.ways.forEach((w) => (by[w.hw] = by[w.hw] || []).push(w.coords));
    setData("net", FC(Object.entries(by).map(([hw, lines]) => multi(lines, { hw }))));
    const pts = [...HOSPITALS, ...PHARMACIES, ...shelters, ...FLOOD];
    setData("links", FC([multi(pts.map((p) => { const n = nearestNode(p.lat, p.lng); return [[p.lng, p.lat], [net.lng[n], net.lat[n]]]; }))]));
  }, [ready, layers.graph, showNet, netInfo, shelters]);

  /* ---------- live markers ---------- */
  useEffect(() => { if (ready) sync("hosp", layers.hospitals ? HOSPITALS.map((h) => { const st = hosp[h.id]; return [h.lat, h.lng, pin("mk-hosp" + (st.divert || st.free === 0 ? " full" : ""), "", h.n, () => actions.select("hospital", h.id))]; }) : []); }, [ready, layers.hospitals, hosp]);
  useEffect(() => { if (ready) sync("shel", layers.shelters ? shelters.map((x) => [x.lat, x.lng, pin("mk-shel" + (!x.open || x.occ >= x.cap ? " full" : ""), "", `${x.n} (simulated)`, () => actions.select("shelter", x.id))]) : []); }, [ready, layers.shelters, shelters]);
  useEffect(() => { if (ready) sync("units", layers.units ? units.map((u, i) => { const n = NODES[u.node]; return [n[0] - 0.0022 - (i % 3) * 0.0012, n[1] + 0.003 + (i % 2) * 0.002, pin("mk-unit" + (u.status !== "Available" ? " busy" : ""), "", `${u.id}, ${u.status.toLowerCase()} (simulated)`, () => actions.select("unit", u.id))]; }) : []); }, [ready, layers.units, units]);
  useEffect(() => { if (ready) sync("inc", layers.incidents ? incidents.map((i) => [i.lat, i.lng, pin(`mk-inc ${i.sev}${i.status === "Closed" ? " closed" : ""}${i.status === "Open" && i.sev === "High" ? " live" : ""}`, i.t === "SOS" ? "S" : "!", i.n, () => actions.select("incident", i.id))]) : []); }, [ready, layers.incidents, incidents]);

  /* ---------- broken and slow roads ---------- */
  useEffect(() => {
    if (!ready) return;
    const on = Object.keys(closures).filter((k) => closures[k]);
    const { blocked, slow } = currentSets();
    setData("water", FC(floodState().pools.map((w) => ({ type: "Feature", properties: {}, geometry: { type: "Polygon", coordinates: [ring(w.lat, w.lng, w.r)] } }))));
    setData("cut", FC(layers.closures && blocked.size ? [multi([...blocked].map(edgeLine))] : []));
    setData("slowed", FC(layers.closures && slow.size ? [multi([...slow].map(edgeLine))] : []));
    sync("clo", layers.closures ? [
      ...on.map((k) => { const c = CLOSURES[k]; return [c.at[0], c.at[1], pin("mk-x" + (c.kind === "slow" ? " slow" : ""), c.kind === "closed" ? "×" : "~", c.n, () => actions.select("closure", k))]; }),
      ...breaks.map((b, i) => [b.lat, b.lng, pin("mk-x user", "×", `${b.name}: reported broken. Click to repair.`, () => actions.removeBreak(i))])
    ] : []);
  }, [ready, layers.closures, closures, breaks, netInfo, flood.stage]);

  /* ---------- routes: the line draws itself, then a marker travels it ---------- */
  useEffect(() => {
    if (!ready) return;
    cancelAnimationFrame(mover.current);
    (marks.current.ab || []).forEach((x) => x.remove()); marks.current.ab = [];
    setData("prev", FC(route?.prev ? [line(ll(route.prev.coords))] : []));
    setData("baseline", FC(route?.baseline && route.baseline.coords.length > 1 ? [line(ll(route.baseline.coords))] : []));
    const A = route?.req.from || planner.from, B = route ? route.dest : planner.mode === "place" ? planner.to : null;
    const ends = [];
    if (A) ends.push([A.lat, A.lng, pin("mk-end", "A", `Start: ${A.name}`)]);
    if (B) ends.push([B.lat, B.lng, pin("mk-end b", "B", `Destination: ${B.name}`)]);
    sync("ab", ends);
    const coords = route?.nexus ? ll(route.nexus.coords) : [];
    if (coords.length < 2) { setData("route", FC()); return; }
    if (fitRoute) {
      const bb = coords.reduce((b, c) => b.extend(c), new maplibregl.LngLatBounds(coords[0], coords[0]));
      map.current.fitBounds(bb, { padding: { top: 90, bottom: 60, left: 70, right: 70 }, duration: settings.motion === "full" ? 900 : 0, pitch: three ? 50 : 0, maxZoom: 14.5 });
    }
    if (settings.motion !== "full") { setData("route", FC([line(coords)])); return; }
    if (race) { setData("route", FC([line(coords)])); return; }
    const dot = new maplibregl.Marker({ element: pin("mk-go", "", "Vehicle on the route") }).setLngLat(coords[0]).addTo(map.current);
    marks.current.ab.push(dot);
    const t0 = performance.now(), n = coords.length;
    let drawn = false;
    const frame = (t) => {
      const p = Math.min(1, (t - t0) / 1100);
      if (!drawn) { setData("route", FC([line(coords.slice(0, Math.max(2, Math.ceil(n * (1 - Math.pow(1 - p, 3))))))])); drawn = p === 1; }
      else dot.setLngLat(coords[Math.min(n - 1, Math.floor((((t - t0 - 1100) / 8000) % 1) * n))]);
      mover.current = requestAnimationFrame(frame);
    };
    mover.current = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(mover.current);
  }, [ready, route, planner.from, planner.to, planner.mode, fitRoute, !!race]);

  /* ---------- reach: roads coloured by drive time from one hospital ---------- */
  useEffect(() => {
    if (!ready) return;
    if (!reach) { setData("reach", FC()); return; }
    const h = HOSPITALS.find((x) => x.id === reach);
    const d = distFrom(nearestNode(h.lat, h.lng), { ...currentSets(), conf, hard: true });
    const by = { ok: [], brass: [], slide: [], crit: [] };
    net.edges.forEach((e) => { const t = Math.max(d[e.a], d[e.b]); by[t <= 15 ? "ok" : t <= 30 ? "brass" : t <= 45 ? "slide" : "crit"].push(edgeLine(e.i)); });
    setData("reach", FC(Object.entries(by).filter(([, l]) => l.length).map(([band, l]) => multi(l, { band }))));
  }, [ready, reach, closures, breaks, conf, netInfo, flood.stage]);

  /* ---------- the race: two vehicles, same start, same moment ---------- */
  useEffect(() => {
    if (!ready) return;
    cancelAnimationFrame(racer.current);
    (marks.current.race || []).forEach((x) => x.remove()); marks.current.race = [];
    if (!race) return;
    const a = new maplibregl.Marker({ element: pin("mk-race", "N", "Nexus") }).setLngLat([race.nexus.pts[0].lng, race.nexus.pts[0].lat]).addTo(map.current);
    const b = new maplibregl.Marker({ element: pin("mk-race base", "B", "Baseline") }).setLngLat([race.base.pts[0].lng, race.base.pts[0].lat]).addTo(map.current);
    marks.current.race = [a, b];
    const frame = () => {
      const t = ((performance.now() - race.t0) / 1000) * race.rate;
      const p = along(race.nexus.pts, t), q = along(race.base.pts, t);
      a.setLngLat([p.lng, p.lat]); b.setLngLat([q.lng, q.lat]);
      if (t < Math.max(race.nexus.pts[race.nexus.pts.length - 1].t, race.base.pts[race.base.pts.length - 1].t)) racer.current = requestAnimationFrame(frame);
    };
    racer.current = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(racer.current);
  }, [ready, race]);

  /* ---------- measuring ---------- */
  useEffect(() => {
    if (!ready) return;
    (marks.current.ms || []).forEach((x) => x.remove()); marks.current.ms = [];
    if (!measure || measure.length < 2) { setData("measure", FC()); return; }
    setData("measure", FC([line(ll(measure))]));
    const total = measure.slice(1).reduce((a, p, i) => a + hav(measure[i][0], measure[i][1], p[0], p[1]), 0);
    const last = measure[measure.length - 1];
    sync("ms", [[last[0], last[1], pin("mk-label", `${total.toFixed(1)} km`, "Straight-line distance")]]);
  }, [ready, measure]);

  useEffect(() => { if (ready && focus) map.current.flyTo({ center: [focus.lng, focus.lat], zoom: focus.zoom, duration: settings.motion === "full" ? 900 : 0 }); }, [ready, focus]);

  const fit = (b) => map.current?.fitBounds(b, { padding: 40, pitch: three ? 50 : 0, duration: 800 });
  return (
    <div className={"mapbox" + (full ? " full" : "") + (measure !== null || pick ? " measuring" : "")}>
      <div ref={el} className="gl" aria-label="Map of Dehradun district" />
      {failed && <div className="empty" style={{ position: "absolute", inset: 0, display: "grid", placeItems: "center" }}>This browser has WebGL switched off, so the 3D map cannot draw. Use a current Chrome, Edge, Safari or Firefox.</div>}
      <div className="map-top">
        <Seg label="Lens" options={Object.keys(LENSES)} value={lens} onChange={actions.setLens} />
        <Seg label="Base map" options={Object.keys(BASES)} value={base} onChange={actions.setBase} />
      </div>
      <div className="map-tools">
        <button className="icon-btn" aria-label="Zoom in" onClick={() => map.current?.zoomIn()}><Plus /></button>
        <button className="icon-btn" aria-label="Zoom out" onClick={() => map.current?.zoomOut()}><Minus /></button>
        <button className={"icon-btn" + (three ? " on" : "")} aria-label={three ? "Switch to flat map" : "Switch to 3D terrain"} aria-pressed={three} title="3D terrain" onClick={actions.toggle3d}><Mountain /></button>
        <button className="icon-btn" aria-label="Fit the city" title="Fit the city" onClick={() => fit(CITY)}><Maximize /></button>
        <button className="icon-btn" aria-label="Fit the district" title="Fit the district" onClick={() => fit(DISTRICT)}><Globe2 /></button>
        <button className={"icon-btn" + (pick === "break" ? " on" : "")} aria-label={pick === "break" ? "Stop breaking roads" : "Break a road"} title="Break a road: click any road to report it impassable" onClick={() => actions.setPick(pick === "break" ? null : "break")}><Unlink /></button>
        <button className={"icon-btn" + (measure !== null ? " on" : "")} aria-label={measure !== null ? "Stop measuring" : "Measure a distance"} title="Measure a distance: click points on the map" onClick={() => setMeasure(measure === null ? [] : null)}><Ruler /></button>
        <button className="icon-btn" aria-label={full ? "Leave full screen" : "Full screen"} title="Full screen" onClick={() => setFull((v) => !v)}>{full ? <Shrink /> : <Expand />}</button>
        <button className="icon-btn" aria-label="Show or hide layers" title="Layers" onClick={() => setShowLayers((v) => !v)}><Layers /></button>
      </div>
      {showLayers && (
        <div className="layers">
          <header>Map layers<button aria-label="Close layers" onClick={() => setShowLayers(false)}><X size={14} /></button></header>
          {LAYER_LIST.map(([k, label, color]) => (
            <label key={k}><i className="dot" style={{ background: `var(${color})` }} /><span>{label}</span><Switch on={!!layers[k]} onChange={() => actions.toggleLayer(k)} label={label} /></label>
          ))}
        </div>
      )}
      {reach && (
        <div className="reach-legend">
          <b style={{ fontWeight: 500 }}>Drive time from {HOSPITALS.find((x) => x.id === reach).n}</b>
          <span><i style={{ background: "var(--ok)" }} />15 min</span><span><i style={{ background: "var(--brass)" }} />30 min</span><span><i style={{ background: "var(--slide)" }} />45 min</span><span><i style={{ background: "var(--crit)" }} />beyond, or cut off</span>
          <button onClick={() => actions.setReach(null)}>Clear</button>
        </div>
      )}
      <div className="floodbar" title="Illustrative: water rises around the documented flood-prone localities. Not a hydraulic model.">
        <button aria-label={flood.playing ? "Pause the flood" : "Let the flood rise"} onClick={() => actions.playFlood(!flood.playing)}>{flood.playing ? <Pause /> : <Play />}</button>
        <Waves />
        <input type="range" min="0" max="100" step="1" value={Math.round(flood.stage)} aria-label="Flood stage" onChange={(e) => { actions.playFlood(false); actions.setFlood(+e.target.value); }} />
        <span>Flood {Math.round(flood.stage)}%</span>
      </div>
      <div className="map-foot">{BASES[base](theme).credit}. Terrain: AWS open elevation tiles. Roads: {netInfo.label}.</div>
    </div>
  );
}
