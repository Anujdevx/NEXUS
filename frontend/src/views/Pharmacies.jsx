/* Medicine availability across real pharmacies, and the nearest one that has what is needed. */
import { useState } from "react";
import { useStore, actions, rankPharmacies } from "../state/store.js";
import { PHARMACIES, MEDICINES, FLOOD } from "../data/index.js";
import { Panel, Seg, Simulated, Sourced, Srcs, fmtMin } from "../components/ui.jsx";

const PLACES = FLOOD.map((f) => ({ key: "flood:" + f.id, name: f.n, lat: f.lat, lng: f.lng }));

export default function Pharmacies() {
  const s = useStore();
  const [med, setMed] = useState("ors");
  const [show, setShow] = useState("All");
  const [q, setQ] = useState("");
  const [at, setAt] = useState(PLACES.find((p) => p.key === "flood:sdr").key);
  const m = MEDICINES.find((x) => x.id === med);
  const from = PLACES.find((p) => p.key === at);
  const ranked = rankPharmacies(from, med);
  const best = ranked[0].qty > 0 && ranked[0].reachable ? ranked[0] : null;
  const list = PHARMACIES.filter((p) => (show === "All" || (show === "In stock") === s.stock[p.id][med] > 0) && (p.n + p.loc).toLowerCase().includes(q.toLowerCase()));
  const total = PHARMACIES.reduce((a, p) => a + s.stock[p.id][med], 0);
  const out = PHARMACIES.filter((p) => s.stock[p.id][med] === 0).length;
  return (
    <>
      <div className="page-head">
        <div><h1>Pharmacies<span lang="sa">औषधि</span></h1><p>Shop names, addresses and hours are real. Stock levels are simulated, because no pharmacy publishes live inventory. In a live build each shop would publish its stock to the bus.</p></div>
      </div>
      <div className="grid2" style={{ gridTemplateColumns: "minmax(0,1fr) 380px" }}>
        <div className="stack">
          <Panel title="Medicine">
            <div className="chips">{MEDICINES.map((x) => <button key={x.id} className={x.id === med ? "on" : ""} aria-pressed={x.id === med} onClick={() => setMed(x.id)}>{x.n}</button>)}</div>
            <p className="dim" style={{ marginTop: 12 }}><b style={{ fontWeight: 500, color: "var(--tx)" }}>{m.n}.</b> {m.use}. {total.toLocaleString("en-IN")} {m.unit} across {PHARMACIES.length - out} shops; {out} shop{out === 1 ? " is" : "s are"} out of stock.</p>
          </Panel>
          <Panel title={`${list.length} pharmacies`} right={<div className="rowx"><input className="input" style={{ width: 180 }} placeholder="Filter by name or area" aria-label="Filter pharmacies" value={q} onChange={(e) => setQ(e.target.value)} /><Seg label="Stock filter" options={["All", "In stock", "Out of stock"]} value={show} onChange={setShow} /></div>} flush>
            {list.length ? (
              <div className="tscroll"><table>
                <thead><tr><th>Pharmacy <Sourced /></th><th>Hours</th><th>{m.n} <Simulated /></th><th></th></tr></thead>
                <tbody>{list.map((p) => { const qty = s.stock[p.id][med]; return (
                  <tr key={p.id} className="click" onClick={() => actions.select("pharmacy", p.id)}>
                    <td><b style={{ fontWeight: 500 }}>{p.n}</b><div className="muted">{p.loc}</div></td><td className="dim">{p.hrs}</td>
                    <td className={qty ? "" : "lv-bad"}>{qty ? `${qty} ${m.unit}` : "Out of stock"}</td>
                    <td onClick={(e) => e.stopPropagation()} style={{ whiteSpace: "nowrap" }}>
                      <button className="btn sm" aria-label={`Remove 5 at ${p.n}`} onClick={() => actions.adjustStock(p.id, med, -5)}>−5</button>{" "}
                      <button className="btn sm" aria-label={`Add 5 at ${p.n}`} onClick={() => actions.adjustStock(p.id, med, 5)}>+5</button>{" "}
                      <button className="btn sm" onClick={() => actions.setStock(p.id, med, qty ? 0 : m.base)}>{qty ? "Mark out" : "Restock"}</button>
                    </td>
                  </tr>); })}</tbody>
              </table></div>
            ) : <div className="empty">No pharmacy matches. Clear the filter or choose another medicine.</div>}
          </Panel>
          <Srcs ids={["maps"]} />
        </div>
        <div className="stack">
          <Panel title="Nearest with stock">
            <div className="stack" style={{ gap: 12 }}>
              <label className="field"><span>Needed at</span><select value={at} onChange={(e) => setAt(e.target.value)}>{PLACES.map((p) => <option key={p.key} value={p.key}>{p.name}</option>)}</select></label>
              {best ? <div className="note"><b style={{ color: "var(--tx)", fontWeight: 500 }}>{best.p.n}</b> has {best.qty} {m.unit}, {fmtMin(best.min)} away by the shortest open road.</div> : <div className="note">No reachable pharmacy has {m.n} in stock. Restock a shop, or repair a broken road.</div>}
              <button className="btn primary" disabled={!best} onClick={() => { actions.go("routes"); actions.setPlanner({ from, mode: "pharmacy", med }); }}>Show the route</button>
            </div>
          </Panel>
          <Panel title="By drive time" right={<Simulated label="Stock simulated" />} flush>
            <div className="rows">{ranked.slice(0, 7).map((x, k) => (
              <button key={x.p.id} onClick={() => actions.select("pharmacy", x.p.id)}>
                <span className="num" style={{ width: 18, color: "var(--tx3)" }}>{k + 1}</span>
                <span className="grow"><b>{x.p.n}</b><small>{x.reachable ? fmtMin(x.min) : "Cut off"}</small></span>
                <span className={x.qty ? "muted" : "lv-bad"}>{x.qty ? `${x.qty} ${m.unit}` : "Out of stock"}</span>
              </button>))}</div>
          </Panel>
        </div>
      </div>
    </>
  );
}
