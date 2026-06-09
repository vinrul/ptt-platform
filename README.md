# PTT Fleet Platform

PTT Fleet Platform adalah sistem Push-to-Talk over IP sederhana untuk komunikasi
lapangan, GPS tracking realtime, SOS event, dan dispatcher web berbasis peta.

## Stack MVP

- Backend: Go + Gin.
- WebSocket: gorilla/websocket.
- Database: PostgreSQL.
- Cache/lock: Redis.
- Web dispatcher: Bun + React + TypeScript + Vite + MapTalks.
- Android: Kotlin native.
- Audio: Opus over WebSocket binary.
- Deployment: Docker Compose + Caddy.

## Dokumen Utama

- `AGENTS.md` - aturan kerja agent/developer.
- `PLAN.md` - rencana implementasi MVP.
- `docs/ARCHITECTURE.md` - desain arsitektur.
- `docs/API.md` - kontrak REST API.
- `docs/WEBSOCKET_PROTOCOL.md` - kontrak event realtime dan audio binary.
- `docs/DATABASE.md` - desain schema database.
- `docs/STRUCTURE.md` - struktur repo.
- `docs/ROADMAP.md` - milestone.
- `docs/DEPLOYMENT.md` - deployment dan backup.
- `docs/LOCAL_TESTING.md` - one-command stack, seed, dan smoke test.

## Perintah Local Target

```bash
bun install
bun run local
```

Perintah tersebut memilih Docker atau Podman, menjalankan dependency, migration,
seed lokal, API, dispatcher, lalu smoke test. Dispatcher tersedia di
`http://localhost:5173`; Pgweb tersedia di `http://localhost:8081` dan langsung
terhubung ke PostgreSQL.

User local tersedia sebagai `admin`, `dispatcher`, `field1`, dan `field2` dengan
password default `ptt-local-123`. Detail dan command terpisah tersedia di
`docs/LOCAL_TESTING.md`.

Android project berada di `apps/android-kotlin`. Emulator memakai server URL
`http://10.0.2.2:8080`; perangkat fisik memakai IP LAN mesin development.

Untuk menjalankan emulator, install debug APK, dan membuka app sekaligus:

```bash
bun run android:run
```

Development memakai Docker Compose sejak awal untuk PostgreSQL, Redis, dan
Pgweb. Backend API bisa dijalankan lewat `bun run docker:api`. Stack aplikasi
penuh dalam container tersedia melalui `bun run docker:local:app`.

Untuk `bun run dev:api` dari host, backend memakai default local:

- `DATABASE_URL=postgres://ptt:ptt@localhost:5432/ptt_fleet?sslmode=disable`
- `REDIS_URL=redis://localhost:6379`

Untuk alur manual, jalankan `bun run docker:local` dulu agar PostgreSQL dan Redis
tersedia.

Repo saat ini dirancang agar dibangun bertahap dari backend foundation, realtime
presence, GPS, SOS, lalu PTT audio Android-to-Android.
