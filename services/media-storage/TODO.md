# media-storage: what is thin

- The console has no attachment UI yet, so nothing in the demo uploads media; the endpoints are tested with curl and by `scripts/smoke.sh --e2e`.
- No virus scanning, thumbnails or expiry. Content types are an allow-list (images, audio, video, PDF), 10 MiB each.
