import { useState } from "react";
import { useStore, actions } from "../state/store.js";
import { HOSPITALS, FLOOD, NODES } from "../data/index.js";
import { rankOnNet, nearestHospital, P } from "../state/store.js";
import { Panel, Seg, Bar, Simulated, Sourced, fmtMin } from "../components/ui.jsx";

const ORIGINS = [...new Map(FLOOD.map((f) => [f.node, f])).values()];

export default function Hospitals() {
  const s = useStore();
  const [own, setOwn] = useState("All");
  const [q, setQ] = useState("");
  const [from, setFrom] = useState("sdr");
  const [need, setNeed] = useState("Trauma");
  const list = HOSPITALS.filter((h) => (own === "All" || h.own === own) && (h.n + h.loc).toLowerCase().includes(q.toLowerCase()));
  const ranked = rankOnNet(P(from), need);
  const base = { h: nearestHospital(P(from)) };
  const baseRow = ranked.find((x) => x.h.id === base.h.id);
  const baseOk = baseRow.capable && baseRow.free > 0 && baseRow.reachable;
  return (
    <>
      <div className="page-head">
        <div><h1>Hospitals<span lang="sa">आरोग्य</span></h1><p>Names, addresses and phone numbers are real. Free beds and inbound load are simulated, because live hospital feeds are not public.</p></div>
      </div>
      <div className="grid2" style={{ gridTemplateColumns: "minmax(0,1fr) 380px" }}>
        <Panel title={`${list.length} hospitals`} right={<div className="rowx"><input className="input" style={{ width: 180 }} placeholder="Filter by name or area" aria-label="Filter hospitals" value={q} onChange={(e) => setQ(e.target.value)} /><Seg label="Ownership" options={["All", "Government", "Private", "Charitable"]} value={own} onChange={setOwn} /></div>} flush>
          {list.length ? (
            <div className="tscroll"><table>
              <thead><tr><th>Hospital</th><th>Level</th><th>Phone</th><th>Bed strength</th><th>Free now <Simulated /></th><th>Inbound</th><th>Accepting</th></tr></thead>
              <tbody>{list.map((h) => { const x = s.hosp[h.id]; const free = x.divert ? 0 : x.free; return (
                <tr key={h.id} className="click" onClick={() => actions.select("hospital", h.id)}>
                  <td><b style={{ fontWeight: 500 }}>{h.n}</b><div className="muted">{h.loc}</div></td>
                  <td className="dim">{h.own}<div className="muted">{h.lvl}</div></td>
                  <td className="dim" style={{ whiteSpace: "nowrap" }}>{h.ph || <span className="muted">Not listed</span>}</td>
                  <td>{h.beds ? <>{h.beds.toLocaleString("en-IN")} <Sourced /></> : <span className="muted">Not verified</span>}</td>
                  <td style={{ minWidth: 110 }}>{free} of {x.cap}<Bar value={x.cap - free} max={x.cap} /></td>
                  <td>{x.inbound}</td>
                  <td onClick={(e) => e.stopPropagation()}><button className={"btn sm" + (x.divert ? " danger" : "")} onClick={() => actions.toggleDivert(h.id)}>{x.divert ? "Diverting" : "Yes"}</button></td>
                </tr>); })}</tbody>
            </table></div>
          ) : <div className="empty">No hospital matches. Clear the filter to see all {HOSPITALS.length}.</div>}
        </Panel>
        <div className="stack">
          <Panel title="Assignment test">
            <div className="stack" style={{ gap: 12 }}>
              <label className="field"><span>Casualty at</span><select value={from} onChange={(e) => setFrom(e.target.value)}>{ORIGINS.map((f) => <option key={f.node} value={f.node}>{NODES[f.node][2]}</option>)}</select></label>
              <div className="field"><span>Clinical need</span><Seg label="Need" options={["Trauma", "Cardiac", "Maternity", "General"]} value={need} onChange={setNeed} /></div>
              <div className="note"><b style={{ color: "var(--tx)" }}>Nexus picks {ranked[0].h.n}</b>, {fmtMin(ranked[0].min)} away with {ranked[0].free} free.</div>
              <p className={baseOk ? "dim" : "lv-bad"}>Nearest-hospital baseline picks {base.h.n}{baseOk ? ", which also works here." : !baseRow.reachable ? ", but the road to it is closed." : !baseRow.capable ? `, which cannot take ${need.toLowerCase()} cases.` : ", which has no free bed."}</p>
              <button className="btn primary" onClick={() => { actions.go("routes"); actions.setPlanner({ from: P(from), mode: "hospital", need }); }}>Open this route</button>
            </div>
          </Panel>
          <Panel title="Ranking" flush>
            <div className="rows">
              {ranked.slice(0, 5).map((x, k) => (
                <button key={x.h.id} onClick={() => actions.select("hospital", x.h.id)}>
                  <span className="muted" style={{ width: 14 }}>{k + 1}</span>
                  <span className="grow"><b>{x.h.n}</b><small>{fmtMin(x.min)}{x.capable ? "" : ", wrong specialty"}{x.free ? "" : ", full"}{x.reachable ? "" : ", cut off"}</small></span>
                  <span className="dim">{x.free} free</span>
                </button>
              ))}
            </div>
          </Panel>
        </div>
      </div>
    </>
  );
}
