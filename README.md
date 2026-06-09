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

## Perintah Local Target

```bash
cp .env.example .env
bun install
bun run docker:local
bun run migrate:up
bun run docker:api
bun run dev:api
bun run dev:web
```

Dispatcher development tersedia di `http://localhost:5173`. Vite mem-proxy
request `/api` dan `/ws` ke backend local port `8080`.

Android project berada di `apps/android-kotlin`. Emulator memakai server URL
`http://10.0.2.2:8080`; perangkat fisik memakai IP LAN mesin development.

Untuk menjalankan emulator, install debug APK, dan membuka app sekaligus:

```bash
bun run android:run
```

Development memakai Docker Compose sejak awal. Pada fase pertama, Compose local
menjalankan PostgreSQL, Redis, dan Pgweb. Pgweb tersedia di
`http://localhost:8081` dan langsung terhubung ke database PostgreSQL dari
Compose tanpa setup koneksi manual. Backend API bisa dijalankan lewat
`bun run docker:api` setelah Dockerfile backend tersedia. Setelah Dockerfile
semua aplikasi tersedia, `bun run docker:local:app` bisa menjalankan stack penuh
lewat Docker.

Untuk `bun run dev:api` dari host, backend memakai default local:

- `DATABASE_URL=postgres://ptt:ptt@localhost:5432/ptt_fleet?sslmode=disable`
- `REDIS_URL=redis://localhost:6379`

Jalankan `bun run docker:local` dulu agar PostgreSQL dan Redis tersedia.

Repo saat ini dirancang agar dibangun bertahap dari backend foundation, realtime
presence, GPS, SOS, lalu PTT audio Android-to-Android.
