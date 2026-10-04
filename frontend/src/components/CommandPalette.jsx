/* Ctrl/Cmd + K: jump to any screen, record or action. */
import { useEffect, useMemo, useRef, useState } from "react";
import { Search, CornerDownLeft, Zap, MapPin, LayoutDashboard, History } from "lucide-react";
import { useStore, actions } from "../state/store.js";
import { HOSPITALS, FLOOD, SLIDES, INFRA, SCENARIOS, CLOSURES, PHARMACIES } from "../data/index.js";
import { NAV } from "./Sidebar.jsx";
import { startDemo } from "./Demo.jsx";

function score(label, q) {
  const l = label.toLowerCase();
  const at = l.indexOf(q);
  if (at >= 0) return 100 - Math.min(at, 50);
  let i = 0;
  for (const ch of l) if (ch === q[i]) i++;
  return i === q.length ? 10 : -1;
}

export default function CommandPalette() {
  const open = useStore((s) => s.palette);
  const incidents = useStore((s) => s.incidents);
  const net = useStore((s) => s.net);
  const theme = useStore((s) => s.theme);
  const closures = useStore((s) => s.closures);
  const [q, setQ] = useState("");
  const [hi, setHi] = useState(0);
  const list = useRef(null);

  useEffect(() => { if (open) { setQ(""); setHi(0); } }, [open]);

  const items = useMemo(() => {
    const rec = (label, kind, sel, lat, lng) => ({ label, kind, icon: MapPin, run: () => { actions.showOnMap(lat, lng, 14); if (sel) actions.select(...sel); } });
    return [
      ...NAV.flat().filter(([id]) => id !== "assignment").map(([id, label]) => ({ label: `Go to ${label}`, kind: "Screen", icon: LayoutDashboard, run: () => actions.go(id) })),
      ...Object.entries(SCENARIOS).map(([k, v]) => ({ label: `Load replay: ${v.label}`, kind: "Replay", icon: History, run: () => actions.setScenario(k) })),
      { label: net ? "Cut the network" : "Restore the network", kind: "Action", icon: Zap, run: () => actions.setNet(!net) },
      { label: "Inject 5 test SOS requests", kind: "Action", icon: Zap, run: () => actions.injectLoad(5) },
      { label: "Acknowledge all open incidents", kind: "Action", icon: Zap, run: () => actions.ackAll() },
      { label: "Return all units to base", kind: "Action", icon: Zap, run: () => actions.returnAll() },
      { label: "Run the 3-minute demo", kind: "Action", icon: Zap, run: () => startDemo() },
      { label: theme === "dark" ? "Switch to light theme" : "Switch to dark theme", kind: "Action", icon: Zap, run: () => actions.setTheme(theme === "dark" ? "light" : "dark") },
      { label: "Play the replay from the start", kind: "Action", icon: Zap, run: () => { actions.setClock(0); actions.play(true); } },
      { label: "Show keyboard shortcuts", kind: "Action", icon: Zap, run: () => actions.setSheet(true) },
      ...Object.entries(CLOSURES).map(([id, c]) => ({ label: `${closures[id] ? "Reopen" : "Close"} segment: ${c.n}`, kind: "Road segment", icon: Zap, run: () => actions.toggleClosure(id) })),
      ...incidents.map((i) => rec(i.n, "Incident", ["incident", i.id], i.lat, i.lng)),
      ...HOSPITALS.map((h) => rec(h.n, "Hospital", ["hospital", h.id], h.lat, h.lng)),
      ...PHARMACIES.map((p) => rec(p.n, "Pharmacy", ["pharmacy", p.id], p.lat, p.lng)),
      { label: "Break a road on the map", kind: "Action", icon: Zap, run: () => { actions.go("routes"); actions.setPick("break"); } },
      { label: "Repair all reported breaks", kind: "Action", icon: Zap, run: () => actions.clearBreaks() },
      { label: "Switch between 3D terrain and flat map", kind: "Action", icon: Zap, run: () => actions.toggle3d() },
      ...FLOOD.map((f) => rec(f.n, "Flood-prone locality", ["flood", f.id], f.lat, f.lng)),
      ...SLIDES.map((f) => rec(f.n, "Landslide point", ["slide", f.id], f.lat, f.lng)),
      ...INFRA.filter((f) => f.lat).map((f) => rec(f.n, f.kind, ["infra", f.id], f.lat, f.lng))
    ];
  }, [incidents, net, theme, closures]);

  const query = q.trim().toLowerCase();
  const hits = (query ? items.map((x) => ({ x, s: score(x.label, query) })).filter((r) => r.s > 0).sort((a, b) => b.s - a.s).map((r) => r.x) : items.slice(0, 30)).slice(0, 40);
  const pick = (x) => { actions.setPalette(false); x.run(); };

  useEffect(() => { list.current?.querySelector(".hi")?.scrollIntoView({ block: "nearest" }); }, [hi]);
  if (!open) return null;
  return (
    <div className="overlay" onPointerDown={(e) => { if (e.target === e.currentTarget) actions.setPalette(false); }}>
      <div className="palette" role="dialog" aria-label="Search and commands">
        <div className="q">
          <Search />
          <input autoFocus value={q} placeholder="Search a place, hospital, incident, road or command" aria-label="Search"
            onChange={(e) => { setQ(e.target.value); setHi(0); }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") { e.preventDefault(); setHi((h) => Math.min(hits.length - 1, h + 1)); }
              if (e.key === "ArrowUp") { e.preventDefault(); setHi((h) => Math.max(0, h - 1)); }
              if (e.key === "Enter" && hits[hi]) pick(hits[hi]);
            }} />
          <kbd>Esc</kbd>
        </div>
        <div className="list" ref={list}>
          {hits.length ? hits.map((x, i) => (
            <button key={x.kind + x.label} className={i === hi ? "hi" : ""} onPointerMove={() => setHi(i)} onClick={() => pick(x)}><x.icon />{x.label}<small>{x.kind}</small></button>
          )) : <div className="empty">Nothing matches “{q}”. Try a locality, a hospital or a screen name.</div>}
        </div>
        <div className="foot"><span><kbd>↑</kbd> <kbd>↓</kbd> move</span><span><kbd><CornerDownLeft size={10} /></kbd> open</span><span><kbd>?</kbd> all shortcuts</span></div>
      </div>
    </div>
  );
}

const KEYS = [["Ctrl K or /", "Search and commands"], ["g then o", "Overview"], ["g then i", "Incidents"], ["g then m", "Map"], ["g then r", "Routes"], ["g then h", "Hospitals"], ["g then p", "Pharmacies"], ["g then s", "Citizen SOS"], ["g then a", "Audit trail"], ["t", "Switch theme"], ["n", "Cut or restore the network"], ["Space", "Play or pause the replay"], ["Esc", "Close the top layer"]];
export function Shortcuts() {
  const open = useStore((s) => s.sheet);
  if (!open) return null;
  return (
    <div className="overlay" onPointerDown={(e) => { if (e.target === e.currentTarget) actions.setSheet(false); }}>
      <div className="sheet" role="dialog" aria-label="Keyboard shortcuts">
        <h2>Keyboard shortcuts</h2>
        <dl>{KEYS.map(([k, d]) => [<dt key={k}><kbd>{k}</kbd></dt>, <dd key={k + "d"}>{d}</dd>])}</dl>
        <div className="rowx" style={{ marginTop: 16 }}><button className="btn" onClick={() => actions.setSheet(false)}>Close</button></div>
      </div>
    </div>
  );
}
