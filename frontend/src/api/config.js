/* Backend connection settings. All optional: with no backend the console runs on its local engine. */
export const API_URL = (import.meta.env.VITE_API_URL || "/api/v1").replace(/\/$/, "");
export const WS_URL = import.meta.env.VITE_WS_URL || "/ws/v1/live";
/* auto: use the backend when it answers, fall back silently when it does not. on: same, but say so in the console log. off: never call it. */
export const LIVE_MODE = import.meta.env.VITE_LIVE || "auto";
export const LIVE_ENABLED = LIVE_MODE !== "off" && typeof window !== "undefined";
/* One id per browser tab. Sent as the envelope origin so the bus never echoes our own events back. */
export const CLIENT_ID = "web-" + Math.random().toString(36).slice(2, 6);
export const TIMEOUT_MS = 2500;
