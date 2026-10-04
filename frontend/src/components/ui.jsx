import { useLayoutEffect, useRef } from "react";
import { SRC } from "../data/index.js";

export function Panel({ title, right, children, flush, className = "" }) {
  return (
    <section className={"panel " + className}>
      {title && (
        <header>
          <h2>{title}</h2>
          {right}
        </header>
      )}
      <div className={"body" + (flush ? " flush" : "")}>{children}</div>
    </section>
  );
}

export const Sourced = () => <span className="tag src" title="Traced to a named source">Sourced</span>;
export const Simulated = ({ label = "Simulated" }) => <span className="tag sim" title="Generated for the demo; not real data">{label}</span>;

export function Srcs({ ids = [] }) {
  const list = (Array.isArray(ids) ? ids : [ids]).filter((k) => SRC[k]);
  if (!list.length) return null;
  return (
    <div className="srcs">
      {list.map((k) => {
        const s = SRC[k];
        const label = `${s.p}, ${s.d}`;
        return s.u ? <a key={k} href={s.u} target="_blank" rel="noreferrer" title={s.t}>{label}</a> : <span key={k} title={s.t}>{label}</span>;
      })}
    </div>
  );
}

export function Switch({ on, onChange, label }) {
  return <button type="button" role="switch" aria-checked={on} aria-label={label} className={"switch" + (on ? " on" : "")} onClick={() => onChange(!on)} />;
}

/* Measure the selected child and slide a glass thumb under it. */
export function useThumb(dep) {
  const box = useRef(null);
  const thumb = useRef(null);
  useLayoutEffect(() => {
    const place = () => {
      const on = box.current?.querySelector(".on");
      const t = thumb.current;
      if (!t) return;
      if (!on) { t.classList.remove("ready"); return; }
      t.style.width = on.offsetWidth + "px";
      t.style.height = on.offsetHeight + "px";
      t.style.transform = `translate(${on.offsetLeft}px, ${on.offsetTop}px)`;
      t.classList.add("ready");
    };
    place();
    const ro = new ResizeObserver(place);
    if (box.current) ro.observe(box.current);
    return () => ro.disconnect();
  }, [dep]);
  return [box, thumb];
}

export function Seg({ options, value, onChange, label }) {
  const [box, thumb] = useThumb(value + options.join("|"));
  return (
    <div className="seg" role="group" aria-label={label} ref={box}>
      <i className="thumb" ref={thumb} aria-hidden="true" />
      {options.map((o) => (
        <button key={o} type="button" className={o === value ? "on" : ""} aria-pressed={o === value} onClick={() => onChange(o)}>{o}</button>
      ))}
    </div>
  );
}

export function Bar({ value, max, invert }) {
  const p = max ? Math.max(0, Math.min(1, value / max)) : 0;
  const load = invert ? 1 - p : p;
  return <div className="bar"><i className={load > 0.9 ? "bad" : load > 0.7 ? "warn" : ""} style={{ width: p * 100 + "%" }} /></div>;
}

export const fmtMin = (m) => (!isFinite(m) ? "cut off" : m < 60 ? `${Math.round(m)} min` : `${Math.floor(m / 60)} h ${Math.round(m % 60)} min`);
export const fmtTime = (iso) => new Date(iso).toLocaleTimeString("en-IN", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
