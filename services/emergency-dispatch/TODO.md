# emergency-dispatch: what is thin

- `GET /dispatch/registry` (Abhayasūchī) and `GET /dispatch/logistics` (Sambharaṇa) return the tier-3 stub envelopes from `MODULES[].stub`, labelled `source:"stub"`. No data behind them.
- Unit choice is the nearest available **ambulance** by drive time; SDRF, fire and boat crews are dispatched by hand from the console.
- If graph-engine is unreachable, drive times fall back to straight-line estimates (`estimator: "haversine"` in the assignment).
