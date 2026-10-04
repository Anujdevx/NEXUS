# The Sūtra envelope and every event

One schema is used on RabbitMQ, on the WebSocket fan-out and on the REST mirror (`POST /api/v1/events`).

```json
{
  "id": "uuid",
  "entity": "sos",                 // sos | assignment | incident | road_segment | route | hospital | unit | shelter | pharmacy
                                   // | flood | rainfall | river | hazard | public_alert | telecom | asset | cascade | scenario
                                   // | race | telemetry | user | media | hazard_report
  "type": "request",
  "geo": [30.3245, 78.0418],       // [lat, lng] or null
  "status": "dispatched",
  "capacity": { "free": 12 },      // or null
  "confidence": 0.9,               // 0..1
  "timestamp": "2026-10-05T09:30:00Z",
  "source": "sutradhara",          // producer; "simulated" or a SRC key where applicable
  "origin": "web-7f3a",            // client or service id: envelopes are never echoed back to their origin
  "payload": { }                   // the domain object, see the table
}
```

`id`, `origin` and `payload` are the three fields added to the console's original envelope. The console only adds them when it
is live, so the offline envelope shown on the SOS screen is unchanged.

**Transport.** One durable topic exchange `nexus.events`. Routing key = `<entity>.<type>`. Every service has a durable queue
`q.<service>`, manual acks, prefetch 32 and a dead-letter exchange `nexus.dlx` (queue `nexus.dead`). Publishers use confirms
and reconnect with backoff; while the broker is down, envelopes buffer in memory.

**Fan-out.** iot-broker binds `#`, writes every envelope to Postgres `event_log`, and pushes it to WebSocket clients:

| Role | Receives |
|---|---|
| Controller | everything |
| Responder | `assignment.*` and `unit.*` (for `?unit=<id>` when given), `road_segment.*`, `public_alert.*`, `route.*` |
| Citizen | `public_alert.*`, and `sos.*` / `assignment.*` whose `payload.client` is this client's id |

## Every `entity.type`

| Key | Produced by | Consumed by | `payload` |
|---|---|---|---|
| `scenario.loaded` | console, `scripts/seed.sh --reset` | graph-engine, traffic-control, emergency-dispatch, predictive-analytics, environmental-monitor | `{id}` scenario id. Every service reloads its seed state |
| `sos.request` | citizen-reporting | emergency-dispatch | `{id, place, placeName, hazard, people, injured, lat, lng, node, via, raised, client, incident}`; `incident` is the console's incident shape |
| `assignment.dispatched` | emergency-dispatch, console | citizen-reporting, emergency-dispatch, notification-gateway | `kind:"sos"`: `{incidentId, unit, hospital:{id,n,lvl,min,reachable,capable,free,inbound,cost}, baseline:{id,n}, alloc:[top 5], need, via, client, estimator, route}`. `kind:"approval"` (console): `{unit, hospital?, shelter?, people?, incidentId?, from, to}`. `kind:"approved"` (REST) |
| `incident.acknowledged`, `incident.closed` | citizen-reporting, console | citizen-reporting, emergency-dispatch | `{id, unit, status}` |
| `hospital.capacity` | emergency-dispatch, graph-engine, console | emergency-dispatch, console | `{id, free?, inbound?, divert?, cap?, reason?, asset?}` (absolute values) |
| `unit.status` | emergency-dispatch, console | emergency-dispatch, console | `{id, status, task?, step?}` or `{all:true}` |
| `unit.progress` | console | emergency-dispatch, console | `{id, step}` (`Free` releases the unit) |
| `shelter.occupancy`, `shelter.status` | emergency-dispatch, console | emergency-dispatch, console | `{id, occ?, open?, cap?}` |
| `pharmacy.stock` | emergency-dispatch, console | emergency-dispatch, console | `{id, med, qty}` |
| `road_segment.passability` | traffic-control, predictive-analytics, console | graph-engine, traffic-control, console | `{kind:"closure", id, active, inferred?}` or `{kind:"break", op:"add"\|"remove"\|"clear", lat, lng, name}` |
| `route.computed` | graph-engine | audit only | `{km, minutes, steps}` (published when a caller sets `publish:true`) |
| `flood.stage` | environmental-monitor, console | graph-engine, console | `{stage}` 0..100, confidence 0.5 (illustrative) |
| `rainfall.observed` | environmental-monitor | predictive-analytics | `{station, mm, source}` |
| `rainfall.what_if` | console | predictive-analytics | `{mm}`. Risk only; the console mirrors its own closure changes |
| `river.level` | environmental-monitor | predictive-analytics | `{river, level, danger, status:"below"\|"warning"\|"danger", source}` |
| `hazard.risk` | predictive-analytics | notification-gateway | `{mm?, level:"low"\|"moderate"\|"high"\|"severe", closures?, basis}` |
| `public_alert.cap_alert` | notification-gateway, console | console, notification-gateway | `{id, area, severity, message, message_hi, by, at, auto, cap}` (CAP document under `cap`) |
| `asset.status` | graph-engine | console | the Asset: `{id, name, type, status, load_percentage, failure_threshold, source:"simulated", ...}` |
| `cascade.computed` | graph-engine | console, notification-gateway | `{root, rootName, rootType, waves:[[id]], failed:[id], degraded:[id], reason}` |
| `telemetry.raw` | iot-broker, simulator (amqp) | graph-engine, environmental-monitor | `{asset_id, type, value, timestamp, source}` |
| `telecom.connectivity` | console, notification-gateway | audit only | `{tower?, area?}` or the SMS stub `{via, client, text, stub:true}` |
| `hazard_report.created` | citizen-reporting | audit only | the report |
| `media.uploaded` | media-storage | audit only | `{id, bytes, type, by}` |
| `user.login` | identity-access | audit only | `{id, role}` |
| `race.started` | console | audit only | none |

`telemetry.raw` types understood today: `power_load_pct`, `water_load_pct`, `telecom_load_pct` (any `*_load_pct`, matched by
`asset_id`), `rainfall_mm` (asset_id = station), `river_level_m` (asset_id = gauge) and `flood_stage` (0..100).
Everything the simulator sends carries `"source": "simulated"`.

## Hospital and cascade rules, in one place

- Hospital ranking cost: `drive_min (or 999+240 if cut off) + 45 if the specialty is missing + 90 if no free bed (diverting counts as 0 free) + 8 if under 3 free + 5 per inbound`.
- Cascade: BFS over the reverse of `DEPENDS_ON`. Each dependent gets `load += 100 * criticality`; above its `failure_threshold` it is FAILED and joins the next wave, otherwise DEGRADED. A FAILED or DEGRADED hospital publishes `hospital.capacity` with `divert:true`.
- A utility fails by itself when its load stays above `failure_threshold` for longer than `grace_period_ms`, or when `POST /api/v1/topology/assets/{id}/fail` is called.
