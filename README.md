# PTT Fleet Platform

PTT Fleet Platform adalah sistem Push-to-Talk over IP sederhana untuk komunikasi
lapangan, GPS tracking realtime, SOS event, dan dispatcher web berbasis peta.

## Stack Version 1

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
- `docs/VERSION_2_PLAN.md` - rencana multi-tenant, scaling, dan evaluasi
  WebRTC/SFU.
- `docs/ARCHITECTURE.md` - desain arsitektur.
- `docs/API.md` - kontrak REST API.
- `docs/WEBSOCKET_PROTOCOL.md` - kontrak event realtime dan audio binary.
- `docs/DATABASE.md` - desain schema database.
- `docs/STRUCTURE.md` - struktur repo.
- `docs/ROADMAP.md` - milestone.
- `docs/DEPLOYMENT.md` - deployment dan backup.
- `docs/LOCAL_TESTING.md` - one-command stack, seed, dan smoke test.
- `docs/SECURITY.md` - baseline keamanan dan checklist production.

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

Build binary API production untuk Linux:

```bash
bun run build:api
```

Build dispatcher production sekaligus membuat ZIP deployment:

```bash
bun run build:web
```

Semua artefak deployment dihasilkan di folder `ptt-fleet/`.

Template native deployment tersedia untuk systemd dan Supervisor di
`infra/systemd` serta `infra/supervisor`.

Repo saat ini dirancang agar dibangun bertahap dari backend foundation, realtime
presence, GPS, SOS, lalu PTT audio Android-to-Android.

Fitur inti Version 1 sudah tersedia. Version 2 memprioritaskan isolasi
multi-tenant dan horizontal scaling. Audio Opus WebSocket tetap dipertahankan
sampai load test menunjukkan SFU diperlukan.
