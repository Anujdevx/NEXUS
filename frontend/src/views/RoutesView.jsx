/* Mārga: pick A and B, get the shortest route on the road network.
   Break any road and the planner finds the next shortest way round. */
import { useEffect, useState } from "react";
import { Send, MapPin, ArrowLeftRight, Unlink, Wrench, Flag } from "lucide-react";
import { useStore, actions } from "../state/store.js";
import { CLOSURES, MEDICINES, FLOOD } from "../data/index.js";
import { allPoints } from "../points.js";
import { Panel, Seg, Switch, Srcs, Sourced, Simulated, fmtMin } from "../components/ui.jsx";
import MapView from "../components/MapView.jsx";

const MODES = { Place: "place", Hospital: "hospital", Pharmacy: "pharmacy", Shelter: "shelter" };
const label = (v) => Object.keys(MODES).find((k) => MODES[k] === v);

function PointSelect({ value, onChange, points, labelText, onPick, picking }) {
  const groups = [...new Set(points.map((p) => p.group))];
  const custom = value && !points.some((p) => p.key === value.key);
  return (
    <div className="field"><span>{labelText}</span>
      <div className="rowx" style={{ flexWrap: "nowrap" }}>
        <select style={{ flex: 1, minWidth: 0 }} value={value ? value.key : ""} onChange={(e) => onChange(points.find((p) => p.key === e.target.value))} aria-label={labelText}>
          {!value && <option value="">Choose a place</option>}
          {custom && <option value={value.key}>{value.name}</option>}
          {groups.map((g) => <optgroup key={g} label={g}>{points.filter((p) => p.group === g).map((p) => <option key={p.key} value={p.key}>{p.name}</option>)}</optgroup>)}
        </select>
        <button className={"icon-btn" + (picking ? " on" : "")} aria-label={`Pick ${labelText.toLowerCase()} on the map`} title="Pick on the map" onClick={onPick}><MapPin /></button>
      </div>
    </div>
  );
}

function RouteCard({ title, r, to, hint, tone, canBreak }) {
  if (!r) return <Panel title={title}><p className="muted">No road joins these two points.</p></Panel>;
  const cut = r.blocked.length > 0;
  return (
    <Panel title={title} right={<span className={cut ? "lv-bad" : r.slow ? "lv-warn" : "lv-ok"}>{cut ? "Crosses a broken road" : r.slow ? "Passable, slowed" : "Passable"}</span>}>
      <div className="rowx" style={{ alignItems: "baseline", gap: 14 }}>
        <span className="num" style={{ fontSize: 34, color: tone }}>{cut ? "cut off" : fmtMin(r.min)}</span>
        <span className="dim">{r.km.toFixed(1)} km to {to}</span>
      </div>
      {cut && <p className="lv-bad" style={{ marginTop: 6 }}>Broken at {r.blocked.join("; ")}. {canBreak ? "There is no other way in, so the planner still shows this road." : ""}</p>}
      <p className="muted" style={{ margin: "6px 0 8px" }}>{hint}</p>
      <div className="steps">
        {r.steps.length ? r.steps.map((st, i) => (
          <div key={i} className={st.cut ? "cut" : st.slow ? "slow" : ""}>
            <span className="grow">{st.name}</span><span className="muted">{st.km.toFixed(1)} km, {fmtMin(st.min)}</span>
            {canBreak && <button className="btn sm" title="Report this road impassable and reroute" onClick={() => actions.breakStep(i)}><Unlink />Break</button>}
          </div>
        )) : <div><span className="muted">Start and destination are at the same junction.</span></div>}
      </div>
    </Panel>
  );
}

/* The race: both policies leave the same place at the same moment. */
function Race({ race, destLabel, baseLabel }) {
  const [, tick] = useState(0);
  useEffect(() => { const id = setInterval(() => tick((n) => n + 1), 80); return () => clearInterval(id); }, [race]);
  const t = ((performance.now() - race.t0) / 1000) * race.rate;
  const lane = (x, name, cls, to) => {
    const end = x.pts[x.pts.length - 1].t, done = t >= end, now = Math.min(t, end);
    const said = x.events.filter((e) => e.t <= t);
    return (
      <div className={"lane " + cls}>
        <b style={{ fontWeight: 500 }}>{name}</b>
        <div className="track"><i style={{ width: (end ? now / end : 1) * 100 + "%" }} /></div>
        <span className="clock">{fmtMin(now)}</span>
        <small>{said.length ? said[said.length - 1].text : done ? (x.ok ? `Arrived at ${to}.` : "Could not get through.") : `On the way to ${to}.`}{done && x.ok && said.length ? ` Arrived after ${fmtMin(end)}.` : ""}</small>
      </div>
    );
  };
  const over = t >= Math.max(race.nexus.pts[race.nexus.pts.length - 1].t, race.base.pts[race.base.pts.length - 1].t);
  const gap = race.base.total - race.nexus.total;
  return (
    <div className="race">
      {lane(race.nexus, "Nexus", "", destLabel)}
      {lane(race.base, "Baseline", "base", race.base.events.some((e) => e.text.includes("Sent on")) ? destLabel : baseLabel)}
      {over && <p className="verdict">{!isFinite(race.nexus.total) ? "Neither can get through: the destination is cut off." : !isFinite(race.base.total) ? "Nexus arrives. The baseline never does." : gap > 0.5 ? `Nexus reaches care ${fmtMin(gap)} sooner.` : gap < -0.5 ? `The baseline is ${fmtMin(-gap)} quicker here: its nearer destination was open and able to take the case.` : "A tie. Nothing separates them on open roads. Break a road on the route, raise the flood, or set the nearest hospital to divert, then race again."}</p>}
    </div>
  );
}

export default function RoutesView() {
  const s = useStore();
  const p = s.planner, r = s.route;
  const points = allPoints(s);
  useEffect(() => { if (!p.from) actions.setPlanner({ from: points.find((x) => x.key === "flood:" + FLOOD.find((f) => f.id === "sdr").id) }); else if (!r) actions.plan(true); }, []); // eslint-disable-line
  const on = Object.keys(s.closures).filter((k) => s.closures[k]);
  const delta = r && r.prev && r.nexus ? { min: r.nexus.min - r.prev.min, km: r.nexus.km - r.prev.km } : null;
  return (
    <>
      <div className="page-head">
        <div><h1>Routes<span lang="sa">मार्ग</span></h1><p>Choose a start and a destination. The planner finds the shortest route on the road network. Break any road and it finds the next shortest way round.</p></div>
        <div className="muted" style={{ textAlign: "right" }}>{s.netInfo.label}<br />{s.netInfo.nodes.toLocaleString("en-IN")} junctions, {s.netInfo.edges.toLocaleString("en-IN")} road segments</div>
      </div>
      <div className="grid2" style={{ gridTemplateColumns: "370px minmax(0,1fr)" }}>
        <div className="stack">
          <Panel title="From A to B">
            <div className="stack" style={{ gap: 12 }}>
              <PointSelect labelText="Start (A)" value={p.from} points={points} onChange={(pt) => actions.setPlanner({ from: pt })} onPick={() => actions.setPick(s.pick === "a" ? null : "a")} picking={s.pick === "a"} />
              <div className="field"><span>Destination (B)</span><Seg label="Destination type" options={Object.keys(MODES)} value={label(p.mode)} onChange={(v) => actions.setPlanner({ mode: MODES[v] })} /></div>
              {p.mode === "place" && <PointSelect labelText="Destination place" value={p.to} points={points} onChange={(pt) => actions.setPlanner({ to: pt })} onPick={() => actions.setPick(s.pick === "b" ? null : "b")} picking={s.pick === "b"} />}
              {p.mode === "hospital" && <div className="field"><span>Clinical need. The best hospital is chosen for you.</span><Seg label="Need" options={["Trauma", "Cardiac", "Maternity", "General"]} value={p.need} onChange={(v) => actions.setPlanner({ need: v })} /></div>}
              {p.mode === "pharmacy" && <label className="field"><span>Medicine needed. The nearest pharmacy with stock is chosen.</span><select value={p.med} onChange={(e) => actions.setPlanner({ med: e.target.value })}>{MEDICINES.map((m) => <option key={m.id} value={m.id}>{m.n}</option>)}</select></label>}
              {p.mode === "shelter" && <p className="muted">The nearest open shelter with space is chosen.</p>}
              <label className="field"><span>Confidence in road reports: {Math.round(s.conf * 100)}%</span><input type="range" min="0" max="1" step="0.05" value={s.conf} onChange={(e) => actions.setConf(+e.target.value)} /></label>
              <div className="rowx">
                <button className="btn" disabled={!p.from || !p.to || p.mode !== "place"} onClick={() => actions.setPlanner({ from: p.to, to: p.from })}><ArrowLeftRight />Swap</button>
                <button className="btn" disabled={!r} onClick={actions.clearRoute}>Clear route</button>
              </div>
            </div>
          </Panel>
          <Panel title="Broken roads" right={<span className="muted">{on.length + s.breaks.length} active</span>} flush>
            <div className="rows">
              <div><span className="grow"><b>Report a broken road</b><small>Click any road on the map, or press Break beside a step of the route.</small></span><button className={"btn sm" + (s.pick === "break" ? " primary" : "")} onClick={() => actions.setPick(s.pick === "break" ? null : "break")}><Unlink />{s.pick === "break" ? "Cancel" : "Break a road"}</button></div>
              {s.breaks.map((b, i) => <div key={i}><span className="grow"><b>{b.name}</b><small>Reported in this session <Simulated /></small></span><button className="btn sm" onClick={() => actions.removeBreak(i)}><Wrench />Repair</button></div>)}
              {s.breaks.length > 1 && <div><span className="grow" /><button className="btn sm" onClick={actions.clearBreaks}>Repair all reported breaks</button></div>}
              {Object.entries(CLOSURES).map(([k, c]) => (
                <div key={k}>
                  <button className="grow" onClick={() => actions.select("closure", k)} style={{ textAlign: "left" }}><b>{c.n}</b><small>{c.kind === "closed" ? "Documented closure" : "Documented waterlogging"} <Sourced /></small></button>
                  <Switch on={s.closures[k]} onChange={() => actions.toggleClosure(k)} label={c.n} />
                </div>
              ))}
            </div>
          </Panel>
        </div>
        <div className="stack">
          <div style={{ height: 500 }}><MapView fitRoute showNet panelOpen={false} /></div>
          {r ? (
            <>
              {delta && <div className="note">Rerouted. The previous way, shown dashed, was {fmtMin(r.prev.min)} and {r.prev.km.toFixed(1)} km. The new shortest route is {delta.min >= 0 ? `${Math.round(delta.min)} min longer` : `${Math.round(-delta.min)} min shorter`} and {Math.abs(delta.km).toFixed(1)} km {delta.km >= 0 ? "longer" : "shorter"}.{r.prev.dest !== r.destLabel ? ` The destination changed from ${r.prev.dest} to ${r.destLabel}.` : ""}</div>}
              <div className="grid2">
                <RouteCard title="Shortest passable route" r={r.nexus} to={r.destLabel} tone="var(--brass)" canBreak hint={r.req.mode === "hospital" ? "Hospital chosen on reach, capacity, specialty and inbound load." : r.req.mode === "pharmacy" ? "Nearest pharmacy that has the medicine in stock." : r.req.mode === "shelter" ? "Nearest open shelter with space." : "A* search, with broken roads weighted by report confidence."} />
                <RouteCard title="Baseline, unaware of breaks" r={r.baseline} to={r.baseLabel} tone="var(--tx2)" hint={r.req.mode === "hospital" ? "Nearest hospital in a straight line, with no capacity or road check." : r.req.mode === "pharmacy" ? (r.baseFull ? "Nearest pharmacy in a straight line, which is out of stock." : "Nearest pharmacy in a straight line.") : r.baseFull ? "Nearest shelter, which is full or closed." : "The shortest path if every road were open."} />
              </div>
              <Panel title="Race" right={<div className="rowx">{s.race && <button className="btn sm" onClick={actions.stopRace}>Clear</button>}<button className="btn sm primary" disabled={!r.nexus || !r.baseline} onClick={actions.startRace}><Flag />{s.race ? "Race again" : "Start the race"}</button></div>}>
                {s.race ? <Race race={s.race} destLabel={r.destLabel} baseLabel={r.baseLabel} /> : <p className="dim">Two vehicles leave {r.req.from.name} at the same moment. One follows the Nexus route. The other follows the baseline, finds out about broken roads and full hospitals only when it gets there, and has to turn back.</p>}
              </Panel>
              <Panel>
                <div className="rowx" style={{ justifyContent: "space-between" }}>
                  <span className="dim">{r.approved ? `Dispatched ${r.unit}. Logged in the audit trail.` : `${r.req.from.name} to ${r.destLabel}. Approving assigns the nearest free ambulance.`}</span>
                  <button className="btn primary" disabled={r.approved || !r.nexus || r.nexus.blocked.length > 0} onClick={actions.approveRoute}><Send />{r.approved ? "Dispatched" : "Approve and dispatch"}</button>
                </div>
              </Panel>
              {r.alloc && (
                <Panel title="Why this hospital" flush>
                  <div className="tscroll"><table>
                    <thead><tr><th>Hospital</th><th>Drive</th><th>Reachable</th><th>Takes {r.req.need.toLowerCase()}</th><th>Free beds</th><th>Inbound</th><th>Cost</th></tr></thead>
                    <tbody>{r.alloc.slice(0, 6).map((x, k) => (
                      <tr key={x.h.id} className="click" onClick={() => actions.select("hospital", x.h.id)}>
                        <td>{k === 0 ? <b style={{ fontWeight: 500, color: "var(--brass)" }}>{x.h.n}</b> : x.h.n}</td><td>{fmtMin(x.min)}</td>
                        <td className={x.reachable ? "lv-ok" : "lv-bad"}>{x.reachable ? "Yes" : "No"}</td><td className={x.capable ? "lv-ok" : "lv-bad"}>{x.capable ? "Yes" : "No"}</td>
                        <td className={x.free ? "" : "lv-bad"}>{x.free}</td><td>{x.inbound}</td><td className="dim">{Math.round(x.cost)}</td>
                      </tr>))}</tbody>
                  </table></div>
                </Panel>
              )}
              {r.pharm && (
                <Panel title={`Pharmacies with ${MEDICINES.find((m) => m.id === r.req.med).n}`} right={<Simulated label="Stock simulated" />} flush>
                  <div className="tscroll"><table>
                    <thead><tr><th>Pharmacy</th><th>Drive</th><th>In stock</th><th>Hours</th></tr></thead>
                    <tbody>{r.pharm.slice(0, 6).map((x, k) => (
                      <tr key={x.p.id} className="click" onClick={() => actions.select("pharmacy", x.p.id)}>
                        <td>{k === 0 ? <b style={{ fontWeight: 500, color: "var(--brass)" }}>{x.p.n}</b> : x.p.n}</td><td>{fmtMin(x.min)}</td>
                        <td className={x.qty ? "lv-ok" : "lv-bad"}>{x.qty || "Out of stock"}</td><td className="dim">{x.p.hrs}</td>
                      </tr>))}</tbody>
                  </table></div>
                </Panel>
              )}
            </>
          ) : <Panel><div className="empty">{p.mode === "place" && !p.to ? "Choose a destination, or pick one on the map, and the shortest route appears." : "Choose a start and a destination."}</div></Panel>}
          <Srcs ids={["synopsis"]} />
        </div>
      </div>
    </>
  );
}
