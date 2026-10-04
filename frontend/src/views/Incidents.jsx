import { useState } from "react";
import { useStore, actions, toast } from "../state/store.js";
import { Panel, Seg, Sourced, Simulated } from "../components/ui.jsx";

export default function Incidents() {
  const incidents = useStore((s) => s.incidents);
  const [sev, setSev] = useState("All");
  const [status, setStatus] = useState("All");
  const [q, setQ] = useState("");
  const [origin, setOrigin] = useState("All");
  const list = incidents.filter((i) => (sev === "All" || i.sev === sev) && (status === "All" || (status === "Open" ? i.status !== "Closed" : i.status === "Closed")) && (origin === "All" || (origin === "Simulated") === !!i.sim) && (!q.trim() || (i.n + " " + i.place + " " + i.t).toLowerCase().includes(q.trim().toLowerCase())));
  const copyCsv = () => {
    const csv = [["Incident", "Type", "Place", "Reported", "Severity", "Status", "Unit", "Origin"], ...list.map((i) => [i.n, i.t, i.place, i.when, i.sev, i.status, i.unit || "", i.sim ? "Simulated" : "Sourced"])].map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(",")).join("\n");
    (navigator.clipboard ? navigator.clipboard.writeText(csv) : Promise.reject()).then(() => toast(`${list.length} rows copied as CSV`), () => toast("Copying is blocked in this browser"));
  };
  return (
    <>
      <div className="page-head">
        <div><h1>Incidents</h1><p>Replay incidents are documented events with their sources. SOS requests raised in this session are simulated and marked.</p></div>
        <div className="rowx">
          <input className="input" style={{ width: 190 }} value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter by name or place" aria-label="Filter incidents" />
          <Seg label="Severity" options={["All", "High", "Medium"]} value={sev} onChange={setSev} />
          <Seg label="Status" options={["All", "Open", "Closed"]} value={status} onChange={setStatus} />
          <Seg label="Origin" options={["All", "Sourced", "Simulated"]} value={origin} onChange={setOrigin} />
          <button className="btn" onClick={actions.ackAll}>Acknowledge all</button>
          <button className="btn" onClick={copyCsv}>Copy as CSV</button>
          <button className="btn" onClick={() => actions.injectLoad(5)}>Inject 5 test SOS</button>
        </div>
      </div>
      <Panel flush>
        {list.length ? (
          <div className="tscroll"><table>
            <thead><tr><th>Incident</th><th>Type</th><th>Place</th><th>Reported</th><th>Severity</th><th>Status</th><th>Origin</th><th></th></tr></thead>
            <tbody>
              {list.map((i) => (
                <tr key={i.id} className="click" onClick={() => actions.select("incident", i.id)}>
                  <td><b style={{ fontWeight: 500 }}>{i.n}</b></td><td className="dim">{i.t}</td><td className="dim">{i.place}</td><td className="muted">{i.when}</td>
                  <td><span className={"sev " + i.sev}>{i.sev}</span></td><td>{i.status}{i.unit ? `, ${i.unit}` : ""}</td><td>{i.sim ? <Simulated /> : <Sourced />}</td>
                  <td onClick={(e) => e.stopPropagation()}>
                    {i.status === "Closed" ? null : i.unit ? <button className="btn sm" onClick={() => actions.closeIncident(i.id)}>Close</button> : <button className="btn sm" onClick={() => actions.dispatchIncident(i.id)}>Dispatch</button>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table></div>
        ) : <div className="empty">Nothing matches these filters. Clear a filter, load a replay, or inject test SOS requests.</div>}
      </Panel>
    </>
  );
}
