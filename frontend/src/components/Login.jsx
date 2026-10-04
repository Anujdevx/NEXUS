import { useState } from "react";
import { useStore, actions } from "../state/store.js";

/* Shown only when the backend has demo tokens off and nobody is signed in. Offline, or in demo mode, it never appears. */
export default function Login() {
  const need = useStore((s) => s.authRequired);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  if (!need) return null;
  const submit = async (e) => {
    e.preventDefault(); setBusy(true); setErr("");
    const msg = await actions.login(email.trim(), password);
    setBusy(false);
    if (msg) setErr(msg);
  };
  return (
    <div style={{ position: "fixed", inset: 0, display: "grid", placeItems: "center", zIndex: 60, background: "rgba(8,8,10,0.72)" }}>
      <form className="panel" onSubmit={submit} style={{ width: "min(380px, 92vw)" }} aria-label="Sign in">
        <header><h2>Sign in to Nexus</h2></header>
        <div className="body stack" style={{ gap: 14 }}>
          <p className="dim">Citizens see the SOS screen. Responders see their assignment. City administrators (Controllers) see everything.</p>
          <label className="field"><span>Email</span><input type="email" autoComplete="username" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} required /></label>
          <label className="field"><span>Password</span><input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required /></label>
          {err && <p className="lv-bad" role="alert">{err}</p>}
          <button className="btn primary" disabled={busy}>{busy ? "Signing in" : "Sign in"}</button>
        </div>
      </form>
    </div>
  );
}
