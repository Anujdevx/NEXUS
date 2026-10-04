/* The live connection to the Sūtra bus: a WebSocket with backoff. It hands every incoming envelope to
   `apply` (the store's applyRemote) and reports whether the bus is connected, so the Topbar chip can say
   Live or Local. With no backend nothing here throws: it just keeps retrying quietly. */
import { CLIENT_ID, LIVE_ENABLED, WS_URL } from "./config.js";
import * as api from "./client.js";

let ws = null;
let timer = null;
let attempt = 0;
let stopped = false;
let ctx = null;
const waiters = new Map(); // sos id -> resolve, for deliverSos waiting on its assignment

function wsUrl(token) {
  const base = /^wss?:/.test(WS_URL) ? WS_URL : (location.protocol === "https:" ? "wss://" : "ws://") + location.host + WS_URL;
  return `${base}?token=${encodeURIComponent(token)}&client=${CLIENT_ID}`;
}

async function connect() {
  if (stopped || !LIVE_ENABLED) return;
  clearTimeout(timer);
  let token;
  try { token = await api.ensureToken(ctx.getRole()); ctx.onLogin && ctx.onLogin(false); }
  catch (e) { if (e.login) { ctx.onLogin && ctx.onLogin(true); return; } return retry(); } // login required: wait for the person to sign in
  try { ws = new WebSocket(wsUrl(token)); } catch { return retry(); }
  const mine = ws;
  mine.onopen = () => { attempt = 0; ctx.onStatus(true); ctx.onConnected(); };
  mine.onmessage = (m) => { try { ctx.onEnvelope(JSON.parse(m.data)); } catch { /* ignore a malformed frame */ } };
  mine.onclose = () => { if (ws === mine) { ws = null; ctx.onStatus(false); retry(); } };
  mine.onerror = () => { try { mine.close(); } catch { /* already closed */ } };
}

function retry() {
  if (stopped) return;
  clearTimeout(timer);
  attempt = Math.min(attempt + 1, 6);
  timer = setTimeout(connect, Math.min(1000 * 2 ** (attempt - 1), 10000));
}

/* { getRole, onEnvelope, onStatus(bool), onConnected } */
export function startLive(c) { ctx = c; stopped = false; connect(); }
/* A role change needs a token for the new role, and the server filters the stream by role. */
export function reconnect() { if (!ctx) return; if (ws) { const w = ws; ws = null; try { w.close(); } catch { /* ignore */ } } ctx.onStatus(false); attempt = 0; connect(); }
export const connected = () => !!ws && ws.readyState === 1;

/* Resolves true when an assignment for this SOS arrives on the bus, false after the timeout. */
export function waitForAssignment(id, ms = 5000) {
  return new Promise((resolve) => {
    const t = setTimeout(() => { waiters.delete(id); resolve(false); }, ms);
    waiters.set(id, () => { clearTimeout(t); waiters.delete(id); resolve(true); });
  });
}
export function assignmentSeen(id) { const w = waiters.get(id); if (w) w(); }
