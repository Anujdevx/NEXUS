import { useRef, useState } from "react";
import { Wifi, WifiOff, Smartphone, RadioTower } from "lucide-react";
import { useStore, actions } from "../state/store.js";
import { FLOOD } from "../data/index.js";
import { Panel, Simulated, fmtMin } from "../components/ui.jsx";
import { useT } from "../i18n.js";

const Chips = ({ options, value, onChange, t = (x) => x }) => <div className="chips">{options.map((o) => <button key={o} className={o === value ? "on" : ""} aria-pressed={o === value} onClick={() => onChange(o)}>{t(o)}</button>)}</div>;

export default function Sos() {
  const s = useStore();
  const t = useT();
  const up = s.net && s.online;
  const [step, setStep] = useState(0);
  const [place, setPlace] = useState("sdr");
  const [hazard, setHazard] = useState("Flood");
  const [people, setPeople] = useState("2");
  const [injured, setInjured] = useState("No");
  const last = s.lastSos;
  const send = () => { actions.sendSos({ place, hazard, people: people === "6+" ? 6 : +people, injured }); setStep(2); };
  const held = last?.stage === "held";
  const done = last?.stage === "dispatched";
  const [holding, setHolding] = useState(false);
  const [hops, setHops] = useState(0);
  const timer = useRef(null);
  const startHold = () => { setHolding(true); timer.current = setTimeout(() => { setHolding(false); setStep(1); }, 1000); };
  const cancelHold = () => { clearTimeout(timer.current); setHolding(false); };
  const relay = () => { setHops(1); setTimeout(() => setHops(2), 420); setTimeout(() => setHops(3), 840); setTimeout(() => { setHops(0); actions.flushQueue("peer relay"); }, 1300); };
  const relayed = done && last.via === "peer relay";
  const lit = (n) => relayed || hops >= n;
  return (
    <>
      <div className="page-head"><div><h1>Citizen SOS<span lang="sa">आह्वान</span></h1><p>One button, no forms. The request carries location, hazard and headcount. With no network it is held on the device and relayed peer to peer until a node has an uplink.</p></div></div>
      <div className="grid2" style={{ gridTemplateColumns: "340px minmax(0,1fr)" }}>
        <div className="stack">
          <div className="phone">
            <div className="bar-top"><span>{up ? t("Network available") : t("No network")}</span><span>{s.queue.length ? `${s.queue.length} ${t("held")}` : "Nexus"}</span></div>
            {step === 0 && (
              <div style={{ textAlign: "center" }}>
                <div className={"sos-hold" + (holding ? " holding" : "")}>
                  <svg viewBox="0 0 190 190" aria-hidden="true"><circle cx="95" cy="95" r="91" /><circle className="p" cx="95" cy="95" r="91" /></svg>
                  <button className="sos-btn" aria-label={t("Hold for one second to send SOS")} onPointerDown={startHold} onPointerUp={cancelHold} onPointerLeave={cancelHold} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setStep(1); } }}>SOS</button>
                </div>
                <p className="dim">{t("Hold for one second, so a pocket cannot send it. We ask three short questions after.")}</p>
                <label className="field" style={{ marginTop: 14, textAlign: "left" }}><span>{t("Device location (test)")}</span><select value={place} onChange={(e) => setPlace(e.target.value)}>{FLOOD.map((f) => <option key={f.id} value={f.id}>{f.n}</option>)}</select></label>
              </div>
            )}
            {step === 1 && (
              <div className="stack" style={{ gap: 14 }}>
                <div className="field"><span>{t("What is happening?")}</span><Chips t={t} options={["Flood", "Landslide", "Trapped", "Medical"]} value={hazard} onChange={setHazard} /></div>
                <div className="field"><span>{t("How many people?")}</span><Chips options={["1", "2", "3", "4", "5", "6+"]} value={people} onChange={setPeople} /></div>
                <div className="field"><span>{t("Is anyone injured?")}</span><Chips t={t} options={["No", "Yes"]} value={injured} onChange={setInjured} /></div>
                <div className="rowx"><button className="btn primary" style={{ flex: 1 }} onClick={send}>{t("Send request")}</button><button className="btn" onClick={() => setStep(0)}>{t("Back")}</button></div>
              </div>
            )}
            {step === 2 && (
              <div className="stack" style={{ gap: 12, textAlign: "center" }}>
                <p style={{ font: "400 22px var(--serif)" }}>{held ? t("Held on this device") : t("Help is assigned")}</p>
                <p className="dim">{held ? t("No network. The request will pass to nearby phones until one reaches a tower.") : done ? `${last.unit || t("The next free unit")} ${t("is coming. You are going to")} ${last.hospital.h.n}.` : ""}</p>
                <button className="btn" onClick={() => setStep(0)}>{t("Raise another")}</button>
              </div>
            )}
          </div>
          <Panel title="Test conditions" right={s.online ? <Simulated /> : <span className="tag">Really offline</span>}>
            <div className="stack" style={{ gap: 10 }}>
              <div className="rowx" style={{ justifyContent: "space-between" }}><span className="rowx">{up ? <Wifi size={15} /> : <WifiOff size={15} />}{!s.online ? "This device is offline" : s.net ? "Towers up" : "Towers down"}</span><button className="btn sm" onClick={() => actions.setNet(!s.net)}>{s.net ? "Cut the network" : "Restore the network"}</button></div>
              <div className="rowx" style={{ justifyContent: "space-between" }}><span className="dim">{s.queue.length} request{s.queue.length === 1 ? "" : "s"} held on devices</span><span className="rowx"><button className="btn sm" disabled={!s.queue.length || hops > 0} onClick={relay}>Peer finds an uplink</button><button className="btn sm" disabled={!s.queue.length || hops > 0} onClick={() => actions.flushQueue("SMS gateway")}>Send by SMS fallback</button></span></div>
              {(s.queue.length > 0 || relayed) && (
                <div className="relay" aria-label="Store, carry, forward">
                  <span className={"hop " + (s.queue.length || relayed ? "on" : "")} title="This phone"><Smartphone /></span><i className={"link" + (lit(1) ? " on" : "")} />
                  <span className={"hop" + (lit(1) ? " on" : " wait")} title="A nearby phone"><Smartphone /></span><i className={"link" + (lit(2) ? " on" : "")} />
                  <span className={"hop" + (lit(2) ? " on" : "")} title="A second phone"><Smartphone /></span><i className={"link" + (lit(3) ? " on" : "")} />
                  <span className={"hop" + (lit(3) ? " on" : "")} title="A tower with an uplink"><RadioTower /></span>
                </div>
              )}
            </div>
            <p className="muted" style={{ marginTop: 10 }}>To test it for real: run a production build, open this screen once, then switch off Wi-Fi. The page still opens, the SOS is held on the device, and it is delivered when Wi-Fi returns. There is no server yet, so delivery means handing the request to the in-browser bus.</p>
            <div className="rowx" style={{ marginTop: 10 }}><button className="btn sm" onClick={actions.install}>{t("Install this app")}</button></div>
          </Panel>
        </div>
        <div className="stack">
          <Panel title="What happened to the last request">
            {!last ? <div className="empty">Press the SOS button to follow one request through the bus.</div> : (
              <ol className="flow">
                <li className="done"><div><b>Raised at {last.sos.placeName}</b><small>{last.sos.hazard}, {last.sos.people} people, injured: {last.sos.injured.toLowerCase()}</small></div></li>
                <li className={done ? "done" : ""}><div><b>{held ? "Held on the device" : `Delivered by ${last.via}`}</b><small>{held ? "Store, carry, forward. Cut and restore the network, or let a peer find an uplink." : "Published to the Sūtra bus as one JSON envelope."}</small></div></li>
                <li className={done ? "done" : ""}><div><b>Hospital chosen{done ? `: ${last.hospital.h.n}` : ""}</b><small>{done ? `${fmtMin(last.hospital.min)} by road, ${last.hospital.free} free. Nearest-hospital baseline would send to ${last.baseline.h.n}.` : "Waiting for delivery."}</small></div></li>
                <li className={done && last.unit ? "done" : ""}><div><b>Unit assigned{done && last.unit ? `: ${last.unit}` : ""}</b><small>{done ? (last.unit ? "Ambulance and hospital both subscribe to the assignment event." : "No ambulance free; queued.") : "Waiting for delivery."}</small></div></li>
              </ol>
            )}
            {done && <div className="rowx"><button className="btn" onClick={() => actions.go("routes")}>See the route</button><button className="btn" onClick={() => actions.select("incident", last.sos.id)}>Open the incident</button></div>}
          </Panel>
          <Panel title="Envelope on the bus">
            <pre className="json">{JSON.stringify(s.bus.find((b) => b.entity === "sos") ? (({ _id, ...e }) => e)(s.bus.find((b) => b.entity === "sos")) : { entity: "sos", type: "request", geo: null, status: null, capacity: null, confidence: 1, timestamp: null, source: "ahvana" }, null, 2)}</pre>
          </Panel>
        </div>
      </div>
    </>
  );
}
