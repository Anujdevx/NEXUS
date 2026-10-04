import { useState } from "react";
import { useStore, actions } from "../state/store.js";
import { NODES } from "../data/index.js";
import { Panel, Seg, Simulated } from "../components/ui.jsx";

export default function Responders() {
  const units = useStore((s) => s.units);
  const [type, setType] = useState("All");
  const types = ["All", ...new Set(units.map((u) => u.type))];
  const list = units.filter((u) => type === "All" || u.type === type);
  return (
    <>
      <div className="page-head">
        <div><h1>Responders<span lang="sa">रक्षक</span></h1><p>Live fleet positions are not public, so every unit here is simulated. Dispatching from an incident or a route changes these rows.</p></div>
        <div className="rowx"><Seg label="Unit type" options={types} value={type} onChange={setType} /><button className="btn" onClick={actions.returnAll}>Return all to base</button></div>
      </div>
      <Panel title={`${list.filter((u) => u.status === "Available").length} of ${list.length} available`} right={<Simulated />} flush>
        <div className="tscroll"><table>
          <thead><tr><th>Unit</th><th>Type</th><th>Base</th><th>Status</th><th>Task</th><th></th></tr></thead>
          <tbody>{list.map((u) => (
            <tr key={u.id} className="click" onClick={() => actions.select("unit", u.id)}>
              <td><b style={{ fontWeight: 500 }}>{u.id}</b></td><td className="dim">{u.type}</td><td className="dim">{NODES[u.node][2]}</td>
              <td className={u.status === "Available" ? "lv-ok" : "lv-warn"}>{u.status}</td><td className="dim">{u.task || <span className="muted">None</span>}</td>
              <td onClick={(e) => e.stopPropagation()} style={{ whiteSpace: "nowrap" }}>
                <button className="btn sm" onClick={() => actions.setUnit(u.id, u.status === "Available" ? "On mission" : "Available")}>{u.status === "Available" ? "Send on mission" : "Mark available"}</button>{" "}
                <button className="btn sm" onClick={() => actions.showOnMap(NODES[u.node][0], NODES[u.node][1], 14)}>Map</button>
              </td>
            </tr>))}</tbody>
        </table></div>
      </Panel>
    </>
  );
}
