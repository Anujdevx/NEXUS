/* Thin fetch wrapper for the NEXUS backend: 2.5 s timeout, bearer token, and a `live` flag.
   Role switching stays instant: the console asks /auth/demo-token for a token per role (no login screen). */
import { API_URL, LIVE_ENABLED, TIMEOUT_MS } from "./config.js";

let token = null;
let tokenRole = null;
let live = false;
const listeners = new Set();

export const isLive = () => live;
export const onLiveChange = (f) => { listeners.add(f); return () => listeners.delete(f); };
function setLive(v) { if (v !== live) { live = v; listeners.forEach((f) => f(v)); } }

async function raw(method, path, body, timeout = TIMEOUT_MS, withAuth = true) {
  const ctl = new AbortController();
  const t = setTimeout(() => ctl.abort(), timeout);
  try {
    const res = await fetch(API_URL + path, {
      method, signal: ctl.signal,
      headers: { ...(body !== undefined ? { "Content-Type": "application/json" } : {}), ...(withAuth && token ? { Authorization: "Bearer " + token } : {}) },
      body: body !== undefined ? JSON.stringify(body) : undefined
    });
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch { /* not JSON: a proxy error page means the backend is down */ }
    if (!res.ok) { const e = new Error(data?.error?.message || res.statusText); e.status = res.status; e.data = data; throw e; }
    return data;
  } finally { clearTimeout(t); }
}

/* A request that fails at the network level (or comes back 502/503/504 from the gateway) means the backend is gone. */
async function call(method, path, body, opt = {}) {
  if (!LIVE_ENABLED) throw new Error("live mode is off");
  try {
    const r = await raw(method, path, body, opt.timeout);
    setLive(true);
    return r;
  } catch (e) {
    if (e.status === undefined || e.status >= 502) setLive(false);
    if (e.status === 401 && token) { token = null; tokenRole = null; } // expired: next call refreshes it
    throw e;
  }
}

export const get = (p, o) => call("GET", p, undefined, o);
export const post = (p, b, o) => call("POST", p, b ?? {}, o);
export const patch = (p, b, o) => call("PATCH", p, b ?? {}, o);

/* A bearer token for the console's current role. */
export async function ensureToken(role) {
  if (!LIVE_ENABLED) throw new Error("live mode is off");
  if (token && tokenRole === role) return token;
  try {
    const r = await raw("POST", "/auth/demo-token", { role }, TIMEOUT_MS, false);
    token = r.token; tokenRole = role;
    setLive(true);
    return token;
  } catch (e) {
    token = null; tokenRole = null;
    if (e.status === undefined || e.status >= 500) setLive(false);
    throw e;
  }
}
export const dropToken = () => { token = null; tokenRole = null; };

/* Does the backend answer? True after the token call or GET /auth/me succeeds. */
export async function probe(role) {
  try { await ensureToken(role); await raw("GET", "/auth/me"); setLive(true); return true; } catch { setLive(false); return false; }
}
