import { useState } from "react";
import { useStore, activeSet, actions, toast, currentSets } from "../state/store.js";
import { sweep, LEVELS } from "../engine/simulation.js";
import { SCENARIOS } from "../data/index.js";
import { Panel, Seg, Simulated, Srcs } from "../components/ui.jsx";

function LineChart({ rows, metric, unit }) {
  const W = 560, H = 250, L = 46, R = 14, T = 14, B = 34;
  const ser = (p) => LEVELS.map((c) => rows.find((r) => r.conn === c && r.policy === p)[metric]);
  const a = ser("nexus"), b = ser("base");
  const hi = Math.max(...[...a, ...b].map((v) => v.m + v.ci)) * 1.1 || 1;
  const x = (i) => L + (i * (W - L - R)) / (LEVELS.length - 1);
  const y = (v) => T + (1 - v / hi) * (H - T - B);
  const path = (s) => s.map((v, i) => `${i ? "L" : "M"}${x(i)},${y(v.m)}`).join(" ");
  const band = (s) => s.map((v, i) => `${i ? "L" : "M"}${x(i)},${y(v.m + v.ci)}`).join(" ") + s.map((v, i) => `L${x(s.length - 1 - i)},${y(Math.max(0, s[s.length - 1 - i].m - s[s.length - 1 - i].ci))}`).join(" ") + "Z";
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((t) => t * hi);
  return (
    <svg className="chart" viewBox={`0 0 ${W} ${H}`} width="100%" role="img" aria-label={`${metric} against connectivity`}>
      {ticks.map((t) => <g key={t}><line className="grid" x1={L} x2={W - R} y1={y(t)} y2={y(t)} /><text x={L - 8} y={y(t) + 4} textAnchor="end">{t.toFixed(hi < 3 ? 1 : 0)}</text></g>)}
      {LEVELS.map((c, i) => <text key={c} x={x(i)} y={H - 12} textAnchor="middle">{c * 100}%</text>)}
      <text x={W - R} y={H - 0.5} textAnchor="end">connectivity</text><text x={L} y={10}>{unit}</text>
      <path d={band(b)} fill="var(--tx3)" opacity="0.16" /><path d={band(a)} fill="var(--brass)" opacity="0.16" />
      <path d={path(b)} fill="none" stroke="var(--tx2)" strokeWidth="1.6" strokeDasharray="5 5" /><path d={path(a)} fill="none" stroke="var(--brass)" strokeWidth="2" />
      {a.map((v, i) => <circle key={i} cx={x(i)} cy={y(v.m)} r="3" fill="var(--brass)" />)}{b.map((v, i) => <circle key={i} cx={x(i)} cy={y(v.m)} r="3" fill="var(--panel)" stroke="var(--tx2)" />)}
    </svg>
  );
}

export default function Evaluation() {
  const s = useStore();
  const [nSos, setN] = useState("40");
  const [relay, setRelay] = useState("12");
  const [retry, setRetry] = useState("25");
  const [seed, setSeed] = useState(2026);
  const [rows, setRows] = useState(null);
  const [prog, setProg] = useState(null);
  const run = async () => {
    setProg(0);
    const out = await sweep({ sets: currentSets(), nSos: +nSos, runs: 30, seed: +seed || 1, relay: +relay, retry: +retry }, setProg);
    setRows(out); setProg(null);
    actions.go("evaluation");
  };
  const get = (c, p) => rows.find((r) => r.conn === c && r.policy === p);
  return (
    <>
      <div className="page-head"><div><h1>Evaluation</h1><p>The connectivity sweep from the evaluation plan, run on the loaded road network with identical seeds for both policies. The roads are real when OpenStreetMap is loaded; the SOS load, hospital capacity and connectivity are generated, so these are simulation results, not field results.</p></div></div>
      <div className="grid2" style={{ gridTemplateColumns: "330px minmax(0,1fr)" }}>
        <div className="stack">
          <Panel title="Run a sweep" right={<Simulated label="Simulated load" />}>
            <div className="stack" style={{ gap: 12 }}>
              <p className="dim">Runs on: {s.netInfo.label}, {s.netInfo.edges.toLocaleString("en-IN")} segments. Broken roads come from the current replay ({SCENARIOS[s.scenario].short}), any reported breaks and the flood stage. Thirty runs per setting.</p>
              <div className="field"><span>SOS requests per run</span><Seg label="SOS per run" options={["20", "40", "60"]} value={nSos} onChange={setN} /></div>
              <div className="field"><span>Mean relay delay when offline, Nexus (min)</span><Seg label="Relay delay" options={["6", "12", "20"]} value={relay} onChange={setRelay} /></div>
              <div className="field"><span>Mean retry delay when offline, baseline (min)</span><Seg label="Retry delay" options={["12", "25", "40"]} value={retry} onChange={setRetry} /></div>
              <label className="field"><span>Seed</span><input type="number" value={seed} onChange={(e) => setSeed(e.target.value)} /></label>
              <button className="btn primary" disabled={prog !== null} onClick={run}>{prog !== null ? "Running…" : rows ? "Run again" : "Run sweep"}</button>
              {prog !== null && <div className="progress"><i style={{ width: prog * 100 + "%" }} /></div>}
            </div>
          </Panel>
          <Panel title="Model">
            <p className="dim">Baseline: nearest hospital by straight line, with no capacity or road knowledge; its crew drives into a break, loses three minutes and replans. Nexus: shortest passable route and constrained assignment, but for each SOS it holds the current road picture, and sees capacity, only with probability equal to connectivity.</p>
            <p className="dim" style={{ marginTop: 8 }}>A casualty not in care within 150 minutes counts as unserved. Set both delays equal to see the advantage narrow as connectivity falls.</p>
            <Srcs ids={["synopsis"]} />
          </Panel>
        </div>
        <div className="stack">
          {rows ? (
            <>
              <div className="grid2">
                <Panel title="Mean time to care"><LineChart rows={rows} metric="ttc" unit="minutes" /><div className="legend"><span><i style={{ background: "var(--brass)" }} />Nexus</span><span><i style={{ background: "var(--tx2)" }} />Nearest-hospital baseline</span><span className="muted">Bands: 95% interval over 30 runs</span></div></Panel>
                <Panel title="Unserved per run"><LineChart rows={rows} metric="unserved" unit="casualties" /><div className="legend"><span><i style={{ background: "var(--brass)" }} />Nexus</span><span><i style={{ background: "var(--tx2)" }} />Baseline</span></div></Panel>
              </div>
              <Panel title="Figures" right={<div className="rowx"><button className="btn sm" onClick={() => { const csv = ["connectivity,ttc_nexus,ttc_baseline,unserved_nexus,unserved_baseline", ...LEVELS.map((c) => { const n = get(c, "nexus"), b = get(c, "base"); return [c * 100, n.ttc.m.toFixed(2), b.ttc.m.toFixed(2), n.unserved.m.toFixed(2), b.unserved.m.toFixed(2)].join(","); })].join("\n"); (navigator.clipboard ? navigator.clipboard.writeText(csv) : Promise.reject()).then(() => toast("Figures copied as CSV"), () => toast("Copying is blocked in this browser")); }}>Copy as CSV</button><Simulated label="Simulated load" /></div>} flush>
                <div className="tscroll"><table className="nowrap">
                  <thead><tr><th>Connectivity</th><th>Time to care, Nexus</th><th>Baseline</th><th>Unserved, Nexus</th><th>Baseline</th><th>Right first hospital, Nexus</th><th>Baseline</th><th>SOS in 30 min, Nexus</th><th>Baseline</th></tr></thead>
                  <tbody>{LEVELS.map((c) => { const n = get(c, "nexus"), b = get(c, "base"); return (
                    <tr key={c}><td>{c * 100}%</td><td>{n.ttc.m.toFixed(1)} ± {n.ttc.ci.toFixed(1)}</td><td className="dim">{b.ttc.m.toFixed(1)} ± {b.ttc.ci.toFixed(1)}</td><td>{n.unserved.m.toFixed(1)}</td><td className="dim">{b.unserved.m.toFixed(1)}</td><td>{Math.round(n.quality.m * 100)}%</td><td className="dim">{Math.round(b.quality.m * 100)}%</td><td>{Math.round(n.delivery.m * 100)}%</td><td className="dim">{Math.round(b.delivery.m * 100)}%</td></tr>); })}</tbody>
                </table></div>
              </Panel>
            </>
          ) : <Panel><div className="empty">No sweep has run yet. Set the assumptions on the left and press Run sweep.</div></Panel>}
        </div>
      </div>
    </>
  );
}
