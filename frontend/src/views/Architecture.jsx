import { useEffect, useState } from "react";
import { Play, Square } from "lucide-react";
import { useStore } from "../state/store.js";
import { Panel, Srcs } from "../components/ui.jsx";

const ROWS = [
  ["Producers", "Layer 1", [["Citizen app", "SOS, hazard reports"], ["Responder app", "Position, status"], ["Sensor feeds", "IMD, CWC, SACHET"], ["Facility systems", "Beds, shelters, stock"]]],
  ["Reasoning", "Layer 2", [["Hazard inference", "Rainfall, slope and crowd reports become passability"], ["Routing engine", "Shortest path with time-varying weights"], ["Allocation engine", "Reach, capacity, specialty, inbound load"], ["Dispatch orchestrator", "Emits the assignment event"]]],
  ["Consumers", "Layer 3", [["Ambulance client", "Assigned route and destination"], ["Hospital client", "Inbound patient manifest"], ["Shelter client", "Occupancy and inbound flow"], ["Sūtradhāra dashboard", "Operating picture and audit"]]]
];
/* [row, box] lit at each stage of one SOS; row -1 is the bus */
const TRACE = [
  [[0, 0], "The citizen presses SOS. The app builds one envelope.", "src/views/Sos.jsx, actions.sendSos"],
  [[-1, 0], "The envelope is published on the Sūtra bus.", "src/state/store.js, publish()"],
  [[1, 0], "Hazard inference marks which segments are believed closed.", "src/engine/hazard.js"],
  [[1, 1], "The router plans around them, weighted by report confidence.", "src/engine/roadnet.js"],
  [[1, 2], "Hospitals are ranked on reach, capacity, specialty and inbound load.", "src/state/store.js, rankOnNet()"],
  [[1, 3], "One assignment event is emitted.", "actions.deliverSos"],
  [[-1, 0], "The assignment travels on the same bus. Nothing talks point to point.", "src/state/store.js, publish()"],
  [[2, 0], "The ambulance receives its route and destination.", "src/views/Assignment.jsx"],
  [[2, 1], "The hospital sees one more inbound patient.", "src/views/Hospitals.jsx"],
  [[2, 3], "The control room sees it on the map and in the audit trail.", "src/views/Audit.jsx"]
];

export default function Architecture() {
  const bus = useStore((s) => s.bus);
  const [step, setStep] = useState(-1);
  useEffect(() => {
    if (step < 0) return;
    const t = setTimeout(() => setStep(step + 1 < TRACE.length ? step + 1 : -1), 1900);
    return () => clearTimeout(t);
  }, [step]);
  const at = step >= 0 ? TRACE[step] : null;
  const lit = (r, b) => at && at[0][0] === r && at[0][1] === b;
  return (
    <>
      <div className="page-head">
        <div><h1>Architecture</h1><p>A publish and subscribe bus, not point-to-point links. Ten domains talking directly would need 45 links; one bus needs 10.</p></div>
        <div className="rowx">{step < 0 ? <button className="btn primary" onClick={() => setStep(0)}><Play />Trace one SOS</button> : <button className="btn" onClick={() => setStep(-1)}><Square />Stop the trace</button>}</div>
      </div>
      <div className="arch">
        <div className="arch-row"><h3>{ROWS[0][0]}<small>{ROWS[0][1]}</small></h3>{ROWS[0][2].map(([b, d], i) => <div key={b} className={"arch-box" + (lit(0, i) ? " on" : "")}><b>{b}</b><small>{d}</small></div>)}</div>
        <div className={"arch-bus" + (at && at[0][0] === -1 ? " on" : "")}>
          <i className="pkt" />
          <div className="grow"><b style={{ font: "400 19px var(--serif)" }}>Sūtra, the event bus</b><div className="dim">Every message is one JSON envelope: entity, type, geo, status, capacity, confidence, timestamp, source.</div></div>
          <span className="muted">{bus.length} envelopes this session</span>
        </div>
        {ROWS.slice(1).map(([name, layer, boxes], r) => (
          <div className="arch-row" key={name}><h3>{name}<small>{layer}</small></h3>{boxes.map(([b, d], i) => <div key={b} className={"arch-box" + (lit(r + 1, i) ? " on" : "")}><b>{b}</b><small>{d}</small></div>)}</div>
        ))}
      </div>
      <div style={{ height: 16 }} />
      <Panel title={at ? `Step ${step + 1} of ${TRACE.length}` : "Follow one request"}>
        {at ? <><p style={{ font: "400 18px/1.4 var(--serif)" }}>{at[1]}</p><p className="muted" style={{ marginTop: 6 }}>Handled in {at[2]}</p></> : <p className="dim">Press “Trace one SOS” to watch a single request move from the citizen’s phone to the ambulance, the hospital and the control room, and to see which file handles each stage.</p>}
      </Panel>
      <Srcs ids={["synopsis"]} />
    </>
  );
}
