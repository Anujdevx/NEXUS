# identity-access: what is thin

- The console never shows a login screen: it uses `/auth/demo-token` (only while `DEMO_MODE=true`). `/auth/login` works for the three seeded users.
- No registration, password reset or token refresh; tokens last 12 hours.
