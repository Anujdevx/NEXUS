import { useStore, actions } from "../state/store.js";
import { NODES } from "../data/index.js";
import { Panel, Bar, Simulated, Switch, Srcs } from "../components/ui.jsx";

export default function Shelters() {
  const shelters = useStore((s) => s.shelters);
  const free = shelters.filter((x) => x.open).reduce((a, x) => a + x.cap - x.occ, 0);
  return (
    <>
      <div className="page-head">
        <div><h1>Shelters<span lang="sa">आश्रय</span></h1><p>No verified public list of relief shelters exists for the district, so these sites and their occupancy are simulated. One has documented past use.</p></div>
        <div className="rowx"><span className="num">{free.toLocaleString("en-IN")}</span><span className="dim">places free across open sites</span></div>
      </div>
      <Panel title="Sites" right={<Simulated />} flush>
        <div className="tscroll"><table>
          <thead><tr><th>Site</th><th>Area</th><th>Occupancy</th><th></th><th>Status</th><th>Open</th></tr></thead>
          <tbody>{shelters.map((x) => (
            <tr key={x.id} className="click" onClick={() => actions.select("shelter", x.id)}>
              <td><b style={{ fontWeight: 500 }}>{x.n}</b>{x.doc && <div className="muted">Documented relief use, June 2013</div>}</td>
              <td className="dim">{NODES[x.node][2]}</td>
              <td style={{ minWidth: 170 }}>{x.occ} of {x.cap}<Bar value={x.occ} max={x.cap} /></td>
              <td onClick={(e) => e.stopPropagation()} style={{ whiteSpace: "nowrap" }}><button className="btn sm" onClick={() => actions.adjustShelter(x.id, -20)}>−20</button> <button className="btn sm" onClick={() => actions.adjustShelter(x.id, 20)}>+20</button></td>
              <td className={!x.open ? "muted" : x.occ >= x.cap ? "lv-bad" : "lv-ok"}>{!x.open ? "Closed" : x.occ >= x.cap ? "Full" : "Open"}</td>
              <td onClick={(e) => e.stopPropagation()}><Switch on={x.open} onChange={() => actions.toggleShelter(x.id)} label={`${x.n} open`} /></td>
            </tr>))}</tbody>
        </table></div>
      </Panel>
      <div className="note" style={{ marginTop: 16 }}>A shelter that fills changes the next routing decision. Fill Rajpur, then route a group from Rajpur to a shelter and watch the destination move.{" "}
        <button className="link" onClick={() => { actions.go("routes"); actions.computeRoute({ from: "rajp", mode: "shelter", need: "General", people: 10 }); }}>Try it</button>
      </div>
      <Srcs ids={["trib2013", "synopsis"]} />
    </>
  );
}
