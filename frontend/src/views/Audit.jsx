import { useState } from "react";
import { useStore, actions, toast } from "../state/store.js";
import { Panel, fmtTime } from "../components/ui.jsx";

export default function Audit() {
  const audit = useStore((s) => s.audit);
  const [q, setQ] = useState("");
  const [show, setShow] = useState(false);
  const list = audit.filter((a) => (a.action + a.detail + a.actor).toLowerCase().includes(q.toLowerCase()));
  const json = JSON.stringify(audit, null, 2);
  const copy = async () => { try { await navigator.clipboard.writeText(json); toast("Audit trail copied as JSON"); } catch { setShow(true); toast("Clipboard blocked. Select the text below to copy."); } };
  return (
    <>
      <div className="page-head">
        <div><h1>Audit trail</h1><p>Every decision taken in this console, with who took it and when. Kept in this browser between visits.</p></div>
        <div className="rowx">
          <input className="input" style={{ width: 200 }} placeholder="Filter entries" aria-label="Filter audit entries" value={q} onChange={(e) => setQ(e.target.value)} />
          <button className="btn" disabled={!audit.length} onClick={copy}>Copy as JSON</button>
          <button className="btn" disabled={!audit.length} onClick={() => setShow((v) => !v)}>{show ? "Hide JSON" : "Show JSON"}</button>
          <button className="btn danger" disabled={!audit.length} onClick={actions.clearAudit}>Clear trail</button>
        </div>
      </div>
      {show && <pre className="json" style={{ marginBottom: 16 }}>{json}</pre>}
      <Panel flush>
        {list.length ? (
          <div className="tscroll"><table>
            <thead><tr><th>Time</th><th>Date</th><th>Actor</th><th>Action</th><th>Detail</th></tr></thead>
            <tbody>{list.map((a) => <tr key={a.id}><td>{fmtTime(a.at)}</td><td className="muted">{new Date(a.at).toLocaleDateString("en-IN", { day: "2-digit", month: "short" })}</td><td className="dim">{a.actor}</td><td>{a.action}</td><td className="dim">{a.detail}</td></tr>)}</tbody>
          </table></div>
        ) : <div className="empty">{audit.length ? "No entry matches that filter." : "No decisions logged yet. Dispatch a unit or approve a route and it is recorded here."}</div>}
      </Panel>
    </>
  );
}
