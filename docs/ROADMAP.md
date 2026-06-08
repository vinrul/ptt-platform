# Roadmap

## MVP 0 - Repo Ready

- Root Bun workspace.
- `.env.example`.
- Docker Compose local for PostgreSQL and Redis.
- Docker Compose production draft.
- Caddyfile draft.
- Go + Gin backend skeleton.
- Documentation contracts: API, database, WebSocket.

## MVP 1 - API Ready

- Health and readiness endpoints.
- Database migration with goose.
- Auth login, refresh, logout, me.
- User CRUD.
- Group CRUD.
- Group membership.
- Seed admin, dispatcher, two field users, and default group.

## MVP 2 - Realtime Ready

- WebSocket endpoint with JWT auth.
- Connection manager.
- Heartbeat timeout.
- Presence online/offline.
- Dispatcher receives presence updates.
- Android can login and connect WebSocket.

## MVP 3 - GPS and SOS Ready

- Android sends `gps.update`.
- Backend persists GPS logs.
- Dispatcher updates MapTalks marker without reload.
- Android sends `sos.create`.
- Dispatcher receives `sos.created`.
- Dispatcher can acknowledge SOS.

## MVP 4 - PTT Audio Ready

- Android A and B join same group.
- A sends `ptt.start`.
- Server grants talk lock.
- A streams Opus binary frames.
- B receives and plays audio.
- B receives `ptt.busy` while A talks.
- A sends `ptt.stop`.
- Lock release works on stop and disconnect.

## MVP 5 - Admin and Live Ready

- Admin panel basic user/group management.
- Device list and last seen.
- Audit log list.
- Docker production.
- Caddy HTTPS.
- Backup script and restore test.
- VPS smoke test.

## Version 1.0

- GPS history playback.
- Audio recording per talk session.
- Dispatcher PTT from browser.
- Better monitoring dashboard.
- Redis pub/sub for multiple backend instances.

## Version 2.0

- WebRTC/SFU upgrade if needed.
- Multi-tenant.
- CCTV/go2rtc integration.
- Advanced reporting.
- Mobile device policy/hardware key support.
