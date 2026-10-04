# iot-broker: what is thin

- Redis is not used here yet: the latest reading per asset (`GET /telemetry/latest`) is kept in memory.
- `GET /events` has `limit` and a routing-key prefix filter only; no time-range paging.
- The WebSocket hub filters by role and (for citizens) by `payload.client`. There is no per-unit subscription list for responders beyond `?unit=`.
