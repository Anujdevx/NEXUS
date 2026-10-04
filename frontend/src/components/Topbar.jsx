import { useEffect, useState } from "react";
import { Search, Sun, Moon, Menu, ChevronDown, Play, Pause, Bell, RadioTower } from "lucide-react";
import { useStore, actions } from "../state/store.js";
import { SCENARIOS } from "../data/index.js";
import { useT } from "../i18n.js";

const ROLES = [["Controller", "Full operating picture and audit"], ["Responder", "Assignment, route and unit status"], ["Citizen", "The one-button SOS"]];
const SPEEDS = [1, 10, 60];

function ReplayBar() {
  const scenario = useStore((s) => s.scenario);
  const clock = useStore((s) => s.clock);
  return (
    <div className="replaybar" title="Incidents and closures are replayed from documented events. The order follows the reports; the minute marks are illustrative.">
      <i className="dot" />Replay
      <select value={scenario} onChange={(e) => actions.setScenario(e.target.value)} aria-label="Replay scenario">
        {Object.entries(SCENARIOS).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
      </select>
      <button aria-label={clock.playing ? "Pause the replay" : "Play the replay"} disabled={!clock.max} onClick={() => actions.play(!clock.playing)}>{clock.playing ? <Pause /> : <Play />}</button>
      <input type="range" min="0" max={clock.max || 1} step="1" value={Math.round(clock.t)} disabled={!clock.max} aria-label="Replay position" onChange={(e) => { actions.play(false); actions.setClock(+e.target.value); }} />
      <button className="spd" aria-label="Replay speed" onClick={() => actions.setSpeed(SPEEDS[(SPEEDS.indexOf(clock.speed) + 1) % SPEEDS.length])}>{clock.speed}x</button>
      <span className="t">T+{Math.round(clock.t)} min</span>
    </div>
  );
}

export default function Topbar() {
  const theme = useStore((s) => s.theme);
  const role = useStore((s) => s.role);
  const net = useStore((s) => s.net);
  const online = useStore((s) => s.online);
  const lang = useStore((s) => s.lang);
  const live = useStore((s) => s.live);
  const session = useStore((s) => s.session);
  const t = useT();
  const waiting = useStore((s) => s.incidents).filter((i) => i.status === "Open");
  const [menu, setMenu] = useState(null);
  useEffect(() => {
    const close = (e) => { if (!e.target.closest(".role, .bell")) setMenu(null); };
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, []);
  const controller = role === "Controller";
  return (
    <header className="top">
      <button className="icon-btn burger" aria-label="Open menu" onClick={actions.toggleNav}><Menu /></button>
      <div className="crumb">India / Uttarakhand / <b>Dehradun</b></div>
      {controller && <button className="find" onClick={() => actions.setPalette(true)} aria-label="Search and commands"><Search /><span>Search or run a command</span><kbd>Ctrl K</kbd></button>}
      <div className="top-right">
        {controller && <ReplayBar />}
        <span className="net" title={live ? "Connected to the Sūtra bus. Changes are shared with every service." : "Running on the local engine. No backend is connected."} style={{ cursor: "default" }}><i className="dot" style={{ background: live ? "var(--ok)" : "var(--tx3)" }} />{live ? "Live" : "Local"}</span>
        <button className={"net" + (net && online ? "" : " down")} onClick={() => actions.setNet(!net)} aria-pressed={!net} title={online ? "Test switch: cut or restore the mobile network" : "This device is really offline. Reconnect to deliver held requests."}><RadioTower /><span>{!online ? t("Device offline") : net ? t("Towers up") : t("Towers down")}</span></button>
        <button className="icon-btn lang" aria-label={lang === "hi" ? "Switch to English" : "हिंदी में बदलें"} title="English / हिंदी" onClick={() => actions.setLang(lang === "hi" ? "en" : "hi")}>{lang === "hi" ? "EN" : "हि"}</button>
        {controller && (
          <div className="role bell">
            <button className="icon-btn" aria-label={`Notifications, ${waiting.length} waiting`} aria-expanded={menu === "bell"} onClick={() => setMenu(menu === "bell" ? null : "bell")}><Bell />{waiting.length > 0 && <span className="n">{waiting.length}</span>}</button>
            {menu === "bell" && (
              <div className="menu" role="menu">
                {waiting.length ? waiting.slice(0, 6).map((i) => <button key={i.id} role="menuitem" onClick={() => { setMenu(null); actions.select("incident", i.id); }}><b>{i.n}</b><small>{i.place}, {i.sev.toLowerCase()} severity</small></button>) : <div className="empty" style={{ padding: 16 }}>Nothing is waiting for acknowledgement.</div>}
                {waiting.length > 0 && <div className="menu-foot"><button role="menuitem" onClick={() => { setMenu(null); actions.ackAll(); }}>Acknowledge all {waiting.length}</button></div>}
              </div>
            )}
          </div>
        )}
        <button className="icon-btn" aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} onClick={() => actions.setTheme(theme === "dark" ? "light" : "dark")}>{theme === "dark" ? <Sun /> : <Moon />}</button>
        <div className="role">
          <button onClick={() => setMenu(menu === "role" ? null : "role")} aria-haspopup="menu" aria-expanded={menu === "role"}>{role}<ChevronDown size={14} /></button>
          {menu === "role" && (
            <div className="menu" role="menu">
              {session
                ? <button role="menuitem" onClick={() => { setMenu(null); actions.logout(); }}>Sign out<small>{session.email}</small></button>
                : ROLES.map(([r, d]) => <button key={r} role="menuitem" onClick={() => { setMenu(null); actions.setRole(r); }}>{r}<small>{d}</small></button>)}
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
