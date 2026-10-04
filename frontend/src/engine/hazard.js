/* Pūrvasūchanā: rainfall what-if. Passability is inferred, not observed.
   The thresholds are illustrative; they are not published Garhwal values. */
import { EDGES, CLOSURES } from "../data/index.js";

export const THRESH = { waterlog: 80, hill: 150 };

/* Which documented weak segments would the planner treat as affected at this rainfall? */
export function inferFromRain(mm) {
  const out = new Set();
  EDGES.forEach(([, , , cls, cid]) => {
    if (!cid) return;
    const kind = CLOSURES[cid].kind;
    if (kind === "slow" && mm >= THRESH.waterlog) out.add(cid);
    if (kind === "closed" && cls === "m" && mm >= THRESH.hill) out.add(cid);
  });
  return [...out];
}
