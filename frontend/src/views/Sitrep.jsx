/* One-click situation report, laid out for print. */
import { Printer } from "lucide-react";
import { useStore, activeSet } from "../state/store.js";
import { SCENARIOS, CLOSURES, HOSPITALS, SRC } from "../data/index.js";

export default function Sitrep() {
  const s = useStore();
  const sc = SCENARIOS[s.scenario];
  const open = s.incidents.filter((i) => i.status !== "Closed");
  const closed = [...activeSet(s)];
  const notAccepting = HOSPITALS.filter((h) => s.hosp[h.id].divert || s.hosp[h.id].free === 0);
  const amb = s.units.filter((u) => u.type === "Ambulance");
  const srcKeys = [...new Set(open.flatMap((i) => i.s || []))].filter((k) => SRC[k]);
  return (
    <>
      <div className="page-head no-print">
        <div><h1>Situation report</h1><p>Compiled from the current state of the console. Print it or save it as a PDF from the print dialog.</p></div>
        <button className="btn primary" onClick={() => window.print()}><Printer />Print or save as PDF</button>
      </div>
      <article className="sitrep">
        <div className="muted">Nexus, Dehradun district. Generated {new Date().toLocaleString("en-IN", { dateStyle: "medium", timeStyle: "short" })} by {s.role.toLowerCase()}.</div>
        <h2>Situation report: {sc.label}</h2>
        <p className="dim" style={{ marginTop: 6 }}>{sc.sum} This is a replay of documented events; unit, bed and shelter figures are simulated.</p>

        <h3>Open incidents ({open.length})</h3>
        {open.length ? <table><thead><tr><th>Incident</th><th>Place</th><th>Severity</th><th>Status</th></tr></thead><tbody>{open.map((i) => <tr key={i.id}><td>{i.n}</td><td className="dim">{i.place}</td><td>{i.sev}</td><td className="dim">{i.status}{i.unit ? `, ${i.unit}` : ""}</td></tr>)}</tbody></table> : <p className="dim">None.</p>}

        <h3>Road segments closed or slow ({closed.length})</h3>
        {closed.length ? <table><thead><tr><th>Segment</th><th>State</th><th>Note</th></tr></thead><tbody>{closed.map((c) => <tr key={c}><td>{CLOSURES[c].n}</td><td>{CLOSURES[c].kind === "closed" ? "Closed" : "Slow"}{s.rainOn.includes(c) ? " (inferred from rainfall what-if)" : ""}</td><td className="dim">{CLOSURES[c].note}</td></tr>)}</tbody></table> : <p className="dim">None.</p>}

        <h3>Medical</h3>
        <p>{HOSPITALS.length - notAccepting.length} of {HOSPITALS.length} hospitals accepting. {notAccepting.length ? `Not accepting: ${notAccepting.map((h) => h.n).join("; ")}.` : ""} {Object.values(s.hosp).reduce((a, h) => a + h.inbound, 0)} patients inbound.</p>

        <h3>Response units and shelters</h3>
        <p>{amb.filter((u) => u.status === "Available").length} of {amb.length} ambulances available. {s.shelters.filter((x) => x.open && x.occ < x.cap).length} of {s.shelters.length} shelter sites have space, {s.shelters.filter((x) => x.open).reduce((a, x) => a + x.cap - x.occ, 0).toLocaleString("en-IN")} places free.</p>

        <h3>Alerts issued this session ({s.alerts.length})</h3>
        {s.alerts.length ? s.alerts.map((a) => <p key={a.id}><b style={{ fontWeight: 500 }}>{a.severity}, {a.area}.</b> {a.message}</p>) : <p className="dim">None.</p>}

        <h3>Sources for the incidents above</h3>
        {srcKeys.length ? srcKeys.map((k) => <p key={k} className="dim">{SRC[k].p}, {SRC[k].d}. {SRC[k].t}.</p>) : <p className="dim">No documented incidents are open.</p>}
      </article>
    </>
  );
}
