# notification-gateway: what is thin

- SMS fallback is a stub: it logs and publishes `telecom.connectivity` with `stub:true`. Nothing is sent to anyone.
- Alerts are CAP-style (`status: Exercise`), English and Hindi. There is no CAP feed, SACHET integration or per-area subscriber list.
- Hindi text exists for the default river message (from the console's `i18n.js`) and for power, water and telecom failures.
