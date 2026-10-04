/* The responder's view: one unit, its task, its route. */
import { useState } from "react";
import { useStore, actions } from "../state/store.js";
import { NODES } from "../data/index.js";
import { Panel, Simulated, fmtMin } from "../components/ui.jsx";
import MapView from "../components/MapView.jsx";

const STEPS = ["En route", "At scene", "Patient on board", "Handed over"];

export default function Assignment() {
  const units = useStore((s) => s.units);
  const route = useStore((s) => s.route);
  const busy = units.filter((u) => u.status !== "Available");
  const [pick, setPick] = useState(null);
  const unit = units.find((u) => u.id === pick) || busy[0] || units[0];
  const mine = route && route.unit === unit.id;
  const at = STEPS.indexOf(unit.step);
  return (
    <>
      <div className="page-head">
        <div><h1>My assignment</h1><p>What a crew sees: one task, the route chosen for it, and no radio confirmation needed.</p></div>
        <label className="field" style={{ minWidth: 200 }}><span>Signed in as unit</span><select value={unit.id} onChange={(e) => setPick(e.target.value)}>{units.map((u) => <option key={u.id} value={u.id}>{u.id}, {u.type.toLowerCase()}</option>)}</select></label>
      </div>
      <div className="grid2" style={{ gridTemplateColumns: "minmax(0, 380px) minmax(0, 1fr)" }}>
        <div className="stack">
          <Panel title={unit.id} right={<Simulated />}>
            <dl className="kv">
              <dt>Base</dt><dd>{NODES[unit.node][2]}</dd>
              <dt>Status</dt><dd className={unit.status === "Available" ? "lv-ok" : "lv-warn"}>{unit.status}{unit.step ? `, ${unit.step.toLowerCase()}` : ""}</dd>
              <dt>Task</dt><dd>{unit.task || <span className="muted">No task. You will be told here when the control room assigns one.</span>}</dd>
              {mine && <><dt>Route</dt><dd>{fmtMin(route.nexus.min)}, {route.nexus.km.toFixed(1)} km to {route.destLabel}</dd></>}
            </dl>
          </Panel>
          <Panel title="Report progress">
            {unit.status === "Available" ? <p className="dim">Nothing to report while you are available.</p> : (
              <div className="stack" style={{ gap: 8 }}>
                {STEPS.map((s, i) => <button key={s} className={"btn" + (i === at + 1 ? " primary" : "")} disabled={i !== at + 1} onClick={() => actions.unitStep(unit.id, s)}>{s}</button>)}
                <button className="btn" disabled={at < STEPS.length - 1} onClick={() => actions.unitStep(unit.id, "Free")}>Mark available</button>
              </div>
            )}
          </Panel>
        </div>
        <div style={{ minHeight: 520 }}><MapView panelOpen={false} fitRoute={!!mine} /></div>
      </div>
    </>
  );
}
