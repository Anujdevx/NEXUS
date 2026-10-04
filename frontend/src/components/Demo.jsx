/* Guided demo: one SOS followed through the system, with captions. */
import { useEffect, useRef } from "react";
import { Pause, Play, X } from "lucide-react";
import { useStore, actions, getState } from "../state/store.js";

const STEPS = [
  { cap: "This is the shared picture: documented incidents and road closures for the selected replay.", run: () => { if (getState().role !== "Controller") actions.setRole("Controller"); if (!getState().net) actions.setNet(true); actions.go("overview"); } },
  { cap: "The towers go down. A citizen at Sahastradhara is about to press the one button.", run: () => { actions.go("sos"); actions.setNet(false); } },
  { cap: "With no network the request is held on the device. It is not lost.", run: () => actions.sendSos({ place: "sdr", hazard: "Flood", people: 4, injured: "Yes" }) },
  { cap: "A nearby phone finds an uplink. The request lands on the bus as one envelope.", run: () => actions.flushQueue("peer relay") },
  { cap: "Ārogya ranks hospitals by reach, capacity, specialty and inbound load, not by distance alone.", run: () => actions.go("hospitals") },
  { cap: "Mārga finds the shortest passable route on the road network. The baseline, in grey, does not know which roads are broken.", run: () => actions.go("routes") },
  { cap: "The controller approves. The ambulance and the hospital receive the same assignment.", run: () => actions.approveRoute() },
  { cap: "Every decision is on the audit trail, with who took it and when.", run: () => { actions.setNet(true); actions.go("audit"); } }
];
const HOLD = 5600;

export const startDemo = () => actions.setDemo({ step: 0, paused: false });

export default function Demo() {
  const demo = useStore((s) => s.demo);
  const ran = useRef(-1);
  useEffect(() => {
    if (!demo) { ran.current = -1; return; }
    if (ran.current !== demo.step) { ran.current = demo.step; STEPS[demo.step].run(); }
    if (demo.paused) return;
    const t = setTimeout(() => actions.setDemo(demo.step + 1 < STEPS.length ? { step: demo.step + 1, paused: false } : null), HOLD);
    return () => clearTimeout(t);
  }, [demo]);
  if (!demo) return null;
  return (
    <div className="caption" role="status">
      <span className="step">{demo.step + 1} of {STEPS.length}</span>
      <p>{STEPS[demo.step].cap}</p>
      <button className="icon-btn" aria-label={demo.paused ? "Resume the demo" : "Pause the demo"} onClick={() => actions.setDemo({ ...demo, paused: !demo.paused })}>{demo.paused ? <Play /> : <Pause />}</button>
      <button className="icon-btn" aria-label="Exit the demo" onClick={() => actions.setDemo(null)}><X /></button>
      <div className="bar"><i style={{ width: ((demo.step + 1) / STEPS.length) * 100 + "%" }} /></div>
    </div>
  );
}
