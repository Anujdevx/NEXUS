/* Visual effects: film grain, the refraction filter for liquid glass,
   the pointer-following sheen, the load sequence and number count-up. */
import { useEffect, useRef, useState } from "react";

export function Grain() { return <div className="grain" aria-hidden="true" />; }

/* The backdrop of a liquid-glass surface is bent through this slow-moving noise field. */
export function RefractDefs({ animate }) {
  return (
    <svg width="0" height="0" style={{ position: "absolute" }} aria-hidden="true">
      <filter id="nx-refract" x="-5%" y="-5%" width="110%" height="110%" colorInterpolationFilters="sRGB">
        <feTurbulence type="fractalNoise" baseFrequency="0.008 0.013" numOctaves="2" seed="7" result="n">
          {animate && <animate attributeName="baseFrequency" dur="16s" values="0.008 0.013;0.013 0.008;0.008 0.013" repeatCount="indefinite" />}
        </feTurbulence>
        <feGaussianBlur in="n" stdDeviation="2" result="nb" />
        <feDisplacementMap in="SourceGraphic" in2="nb" scale="16" xChannelSelector="R" yChannelSelector="G" />
      </filter>
    </svg>
  );
}

const SHEEN = ".top, .layers, .drawer, .palette, .phone, .sheet, .menu";
export function useSheen() {
  useEffect(() => {
    let last = null;
    const move = (e) => {
      const el = e.target.closest ? e.target.closest(SHEEN) : null;
      if (last && last !== el) last.style.setProperty("--hl", 0);
      if (el) {
        const r = el.getBoundingClientRect();
        el.style.setProperty("--mx", e.clientX - r.left + "px");
        el.style.setProperty("--my", e.clientY - r.top + "px");
        el.style.setProperty("--hl", 1);
      }
      last = el;
    };
    window.addEventListener("pointermove", move, { passive: true });
    return () => window.removeEventListener("pointermove", move);
  }, []);
}

export function Boot({ motion }) {
  const [on, setOn] = useState(() => {
    if (motion !== "full") return false;
    try { return !sessionStorage.getItem("nexus.booted"); } catch { return true; }
  });
  useEffect(() => {
    if (!on) return;
    const done = () => { setOn(false); try { sessionStorage.setItem("nexus.booted", "1"); } catch { /* ignore */ } };
    const t = setTimeout(done, 1650);
    window.addEventListener("keydown", done);
    window.addEventListener("pointerdown", done);
    return () => { clearTimeout(t); window.removeEventListener("keydown", done); window.removeEventListener("pointerdown", done); };
  }, [on]);
  if (!on) return null;
  return (
    <div className="boot" aria-hidden="true">
      <svg viewBox="0 0 32 32" width="92" height="92">
        <path d="M16 0.75a15.25 15.25 0 1 1 0 30.5a15.25 15.25 0 1 1 0 -30.5" fill="none" strokeWidth="0.6" />
        <text x="16" y="21.6" textAnchor="middle" fontFamily="Spectral, Georgia, serif" fontSize="16.5" fontWeight="300">N</text>
      </svg>
      <b>Nexus</b>
      <span>Coordination layer for disaster response</span>
    </div>
  );
}

export function CountUp({ value }) {
  const [v, setV] = useState(0);
  const from = useRef(0);
  useEffect(() => {
    const a = from.current, b = value;
    if (a === b) { setV(b); return; }
    const t0 = performance.now();
    let raf;
    const step = (t) => {
      const p = Math.min(1, (t - t0) / 600);
      setV(Math.round(a + (b - a) * (1 - Math.pow(1 - p, 3))));
      if (p < 1) raf = requestAnimationFrame(step); else from.current = b;
    };
    raf = requestAnimationFrame(step);
    return () => { cancelAnimationFrame(raf); from.current = b; };
  }, [value]);
  return v.toLocaleString("en-IN");
}
