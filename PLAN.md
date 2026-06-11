# PLAN.md

# PTT Fleet Platform - Version 1 Project Plan

Dokumen ini adalah rencana kerja bertahap untuk membangun sistem Push-to-Talk
over IP sampai bisa diuji lokal dan live di VPS.

Implementasi fitur inti Version 1 telah tercapai. Pekerjaan Version 2 untuk
multi-tenant, horizontal scaling, dan evaluasi WebRTC/SFU dilanjutkan di
`docs/VERSION_2_PLAN.md`. Phase di bawah tetap menjadi checklist operasional V1,
terutama deployment, backup restore, monitoring, dan smoke test live.

Keputusan teknis final MVP:

- Backend: Go + Gin.
- WebSocket: gorilla/websocket.
- Database: PostgreSQL.
- Migration: goose.
- Cache/presence/lock: Redis.
- Android: Kotlin native.
- Dispatcher: Bun + React + TypeScript + Vite + MapTalks.
- Audio: Opus over WebSocket binary.
- Deployment: Docker Compose + Caddy.

## 0. Gambaran Produk

Produk yang dibuat adalah sistem komunikasi lapangan berbasis internet, mirip
WalkieFleet versi mandiri.

Komponen utama:

1. Android PTT App.
2. Backend API + Realtime Server.
3. Web Dispatcher dengan MapTalks.
4. Admin Management.
5. Database dan Infrastruktur Live.

Target awal:

- 10-50 user aktif.
- 1-3 dispatcher/operator.
- 1 server VPS.
- Komunikasi PTT grup.
- GPS realtime.
- SOS.
- Basic log.

Target lanjutan:

- 100-500 user.
- Multi group/channel lebih kompleks.
- Recording audio.
- Playback riwayat lokasi.
- WebRTC/SFU jika WebSocket audio tidak cukup.
- Integrasi CCTV atau go2rtc.

## 1. Urutan MVP Praktis

Urutan ini dipakai untuk menghindari scope creep dan memastikan sistem bisa
berjalan end-to-end secepat mungkin:

1. Docker development foundation.
2. Repo dan tooling.
3. Backend bootstrap + database migration.
4. Auth + user/group API.
5. WebSocket auth + presence.
6. Dispatcher shell + WebSocket connection.
7. Android login + WebSocket connection.
8. GPS realtime.
9. SOS event.
10. PTT audio Android-to-Android.
11. Admin panel dasar.
12. Docker production + deployment.

## Phase 1 - Docker Development Foundation

### Tujuan

Menyiapkan development environment berbasis Docker sejak awal, karena local dan
production sama-sama ditargetkan berjalan lewat Docker Compose.

### Task 1.1 - Environment File

Buat `.env.example` dan gunakan `.env` lokal dari file tersebut.

Acceptance criteria:

- `.env.example` berisi konfigurasi PostgreSQL, Redis, API, JWT, dan public URL.
- `.env` tidak di-commit.
- Compose local membaca env dari `.env`.

### Task 1.2 - Docker Compose Local

File:

```text
infra/docker/docker-compose.local.yml
```

Service awal:

- `postgres`
- `redis`

Service aplikasi disiapkan sebagai profile `app`:

- `api-server`
- `dispatcher-web`

Acceptance criteria:

- `docker compose -f infra/docker/docker-compose.local.yml --env-file .env up -d postgres redis` jalan.
- PostgreSQL punya healthcheck.
- Redis punya healthcheck.
- Volume data local tidak hilang saat container restart.

### Task 1.3 - Docker Compose Production Draft

File:

```text
infra/docker/docker-compose.prod.yml
```

Service:

- `postgres`
- `redis`
- `api-server`
- `dispatcher-web`
- `caddy`

Acceptance criteria:

- Production topology sudah jelas sejak awal.
- Secret production tetap lewat `.env`, bukan hardcoded.
- Caddyfile draft tersedia.

## Phase 2 - Repo dan Tooling

### Tujuan

Membuat struktur repo yang siap dikembangkan di atas foundation Docker.

### Task 2.1 - Buat Struktur Folder

```text
ptt-fleet/
  AGENTS.md
  PLAN.md
  README.md
  package.json
  bun.lockb
  .env.example
  apps/
    dispatcher-web/
    android-kotlin/
  services/
    api-server/
  packages/
    shared-types/
  infra/
    docker/
    caddy/
    nginx/
  docs/
```

Acceptance criteria:

- Struktur folder sesuai `docs/STRUCTURE.md`.
- Root workspace bisa dibaca oleh Bun.
- Tidak ada secret real di repo.

### Task 2.2 - Setup Bun Workspace

Root `package.json`:

```json
{
  "name": "ptt-fleet",
  "private": true,
  "workspaces": [
    "apps/*",
    "packages/*"
  ],
  "scripts": {
    "dev:web": "bun --filter dispatcher-web dev",
    "build:web": "bun --filter dispatcher-web build",
    "lint:web": "bun --filter dispatcher-web lint",
    "dev:api": "cd services/api-server && go run ./cmd/server",
    "docker:local": "docker compose -f infra/docker/docker-compose.local.yml --env-file .env up -d postgres redis",
    "docker:local:app": "docker compose -f infra/docker/docker-compose.local.yml --env-file .env --profile app up -d",
    "docker:down": "docker compose -f infra/docker/docker-compose.local.yml --env-file .env down"
  }
}
```

### Task 2.3 - Setup Environment

`.env.example` minimal:

```env
APP_ENV=local
API_PORT=8080
DATABASE_URL=postgres://ptt:ptt@postgres:5432/ptt_fleet?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=change-me-use-32-byte-random
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_HOURS=720
POSTGRES_DB=ptt_fleet
POSTGRES_USER=ptt
POSTGRES_PASSWORD=ptt
PUBLIC_API_URL=http://localhost:8080
PUBLIC_WS_URL=ws://localhost:8080/ws
```

## Phase 3 - Backend Bootstrap

### Tujuan

Membuat service Go + Gin yang bisa start, membaca env, connect database, dan
menjawab health check.

### Task 3.1 - Go Module

Struktur:

```text
services/api-server/
  cmd/server/main.go
  internal/
    config/
    db/
    httpserver/
  migrations/
  go.mod
```

Dependency awal:

- `github.com/gin-gonic/gin`
- `github.com/gorilla/websocket`
- `github.com/jackc/pgx/v5`
- `github.com/redis/go-redis/v9`
- `github.com/golang-jwt/jwt/v5`
- `golang.org/x/crypto`

Acceptance criteria:

- `go run ./cmd/server` jalan.
- `GET /healthz` return OK.
- `GET /readyz` cek koneksi PostgreSQL.

### Task 3.2 - Config Loader

Ambil konfigurasi dari env:

- `APP_ENV`
- `API_PORT`
- `DATABASE_URL`
- `REDIS_URL`
- `JWT_SECRET`
- `JWT_ACCESS_TTL_MINUTES`
- `JWT_REFRESH_TTL_HOURS`

Acceptance criteria:

- Config gagal cepat jika env penting kosong.
- Local default hanya dipakai untuk non-secret.

## Phase 4 - Database Design dan Migration

### Tujuan

Membuat schema database MVP yang cukup untuk auth, group, GPS, SOS, dan talk
session.

### Task 4.1 - Migration Tool

Gunakan goose.

Command local:

```bash
cd services/api-server
goose -dir migrations postgres "$DATABASE_URL" up
```

Acceptance criteria:

- Migration up jalan.
- Migration down tersedia untuk development.

### Task 4.2 - Tabel MVP

Tabel:

- `users`
- `devices`
- `groups`
- `group_members`
- `gps_logs`
- `sos_events`
- `talk_sessions`
- `refresh_tokens`
- `audit_logs`

Acceptance criteria:

- Schema ada di migration.
- Index GPS dan audit log sudah dibuat.
- Constraint role/status dibuat eksplisit.

Detail schema dijelaskan di `docs/DATABASE.md`.

## Phase 5 - Auth, User, dan Group API

### Tujuan

Membuat API dasar untuk login dan manajemen MVP.

### Task 4.1 - Auth API

Endpoint:

```text
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET  /api/auth/me
```

Acceptance criteria:

- Login dengan username/password valid menghasilkan access token dan refresh token.
- Password disimpan sebagai hash.
- Refresh token bisa di-revoke saat logout.
- Endpoint `/me` protected JWT.

### Task 4.2 - User API

Endpoint:

```text
GET    /api/users
POST   /api/users
GET    /api/users/:id
PATCH  /api/users/:id
DELETE /api/users/:id
```

Acceptance criteria:

- Super admin bisa CRUD user.
- Dispatcher tidak bisa membuat super admin.
- Delete MVP boleh soft delete via status `disabled`.

### Task 4.3 - Group API

Endpoint:

```text
GET    /api/groups
POST   /api/groups
GET    /api/groups/:id
PATCH  /api/groups/:id
DELETE /api/groups/:id
POST   /api/groups/:id/members
DELETE /api/groups/:id/members/:userId
```

Acceptance criteria:

- User bisa dimasukkan ke grup.
- Membership dipakai oleh WebSocket untuk validasi join dan PTT.

## Phase 6 - WebSocket Realtime Foundation

### Tujuan

Membuat koneksi realtime yang aman dan dipakai Android serta dispatcher.

### Task 5.1 - WebSocket Endpoint

Endpoint:

```text
GET /ws?token=<jwt>
```

Acceptance criteria:

- JWT invalid ditolak.
- Koneksi valid mendapat `connection.ready`.
- Server menyimpan connection id, user id, role, joined groups, dan last heartbeat.

### Task 5.2 - Event Envelope

Semua JSON event mengikuti `docs/WEBSOCKET_PROTOCOL.md`.

Acceptance criteria:

- Event tanpa `type` ditolak.
- Event error memakai `type: "error"`.
- Timestamp server memakai RFC3339 UTC.

### Task 5.3 - Presence

Event:

- Server broadcast `presence.updated` saat user online/offline.

Acceptance criteria:

- Dispatcher menerima status online/offline tanpa reload.
- Disconnect atau heartbeat timeout mengubah presence menjadi offline.

### Task 5.4 - Heartbeat

Client kirim setiap 20-30 detik:

```json
{
  "type": "heartbeat",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {}
}
```

Acceptance criteria:

- Server disconnect jika tidak ada heartbeat lebih dari 90 detik.

## Phase 7 - Dispatcher Shell

### Tujuan

Membuat web dispatcher awal yang bisa login, connect WebSocket, dan menampilkan
presence.

### Task 6.1 - Setup Vite React

```bash
bun create vite apps/dispatcher-web --template react-ts
```

Install:

```bash
bun add maptalks zustand
```

Acceptance criteria:

- `bun --filter dispatcher-web dev` jalan.
- Halaman login tersedia.
- Setelah login, dispatcher connect ke WebSocket.

### Task 6.2 - Layout Dispatcher

Layout:

```text
+------------------------------------------------+
| Top Bar: status, operator, group selector      |
+----------------------+-------------------------+
| Left Panel           | MapTalks Map            |
| - User list          | - User markers          |
| - Online/offline     | - SOS markers           |
| - Active speaker     |                         |
+----------------------+-------------------------+
| Bottom: PTT/logs/alerts                        |
+------------------------------------------------+
```

Acceptance criteria:

- MapTalks tampil.
- User list bisa menerima update presence.

## Phase 8 - Android Foundation

### Tujuan

Membuat Android app awal untuk login dan koneksi WebSocket.

### Task 7.1 - Project Setup

Module:

```text
apps/android-kotlin/
```

Package:

```text
id.nuwiarul.pttfleet
```

### Task 7.2 - Permission

Permission:

- `INTERNET`
- `RECORD_AUDIO`
- `ACCESS_FINE_LOCATION`
- `ACCESS_COARSE_LOCATION`
- `FOREGROUND_SERVICE`
- `POST_NOTIFICATIONS`
- `WAKE_LOCK`

### Task 7.3 - Login + Token Storage

Acceptance criteria:

- User bisa input server URL, username, password.
- Access token dan refresh token tersimpan aman.
- App bisa connect ke `/ws`.

## Phase 9 - GPS Realtime

### Tujuan

Mengirim GPS dari Android dan menampilkan marker di dispatcher.

### Task 8.1 - GPS Event

Android kirim:

```json
{
  "type": "gps.update",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "lat": -8.65,
    "lng": 115.21,
    "speed": 12.5,
    "heading": 90,
    "accuracy": 8
  }
}
```

Server broadcast:

```json
{
  "type": "gps.updated",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "userId": "uuid",
    "lat": -8.65,
    "lng": 115.21,
    "speed": 12.5,
    "heading": 90,
    "accuracy": 8,
    "recordedAt": "2026-06-08T12:00:00Z"
  }
}
```

Acceptance criteria:

- GPS tersimpan ke PostgreSQL.
- Dispatcher update marker tanpa reload map.
- Validasi lat/lng dilakukan server.

## Phase 10 - SOS Event

### Tujuan

Membuat emergency flow dari Android ke dispatcher.

### Task 9.1 - Create SOS

Android kirim `sos.create`, server simpan dan broadcast `sos.created`.

Acceptance criteria:

- SOS muncul di dispatcher.
- Dispatcher zoom ke lokasi SOS.
- SOS bisa di-ack oleh dispatcher.
- Audit log mencatat create dan ack.

## Phase 11 - PTT Audio MVP

### Tujuan

Membuat komunikasi suara Android-to-Android berjalan dalam grup.

### Task 10.1 - Talk Lock

Aturan:

- Satu grup hanya boleh punya satu speaker aktif.
- Jika user lain tekan PTT saat channel sibuk, server kirim `ptt.busy`.
- Lock MVP boleh di-memory untuk single instance, lalu dipindah ke Redis sebelum
  production multi-instance.

Acceptance criteria:

- `ptt.start` menghasilkan `ptt.granted` jika channel kosong.
- `ptt.start` menghasilkan `ptt.busy` jika channel dipakai.
- Disconnect speaker otomatis release lock.

### Task 10.2 - Audio Binary Relay

Format:

```text
[1 byte frame_type][16 byte session_uuid][8 byte sequence_be][opus_payload]
```

Acceptance criteria:

- Server relay frame uplink `0x01` menjadi downlink `0x02`.
- Server hanya relay ke listener dalam grup yang sama.
- Frame dari session invalid ditolak.
- Sequence gap dicatat sebagai log debug.

### Task 10.3 - Manual Test

Skenario:

1. Login Android A.
2. Login Android B.
3. Keduanya join group yang sama.
4. A tekan PTT.
5. B mendengar suara.
6. B tekan PTT saat A bicara, B menerima busy.
7. A lepas PTT.
8. B bisa bicara.

## Phase 12 - Admin Panel Dasar

### Tujuan

Mengelola user, group, device, dan audit log tanpa akses database langsung.

Task:

- User management.
- Group management.
- Device management read-only.
- Audit log list.

Acceptance criteria:

- Admin bisa membuat user field.
- Admin bisa assign user ke group.
- Dispatcher tidak punya akses penuh super admin.

## Phase 13 - Local Testing

### Tujuan

Memastikan semua komponen berjalan lokal sebelum live.

Task:

- Docker Compose local untuk PostgreSQL dan Redis.
- Seed admin, dispatcher, 2 field users, 1 group default.
- Test API dengan curl/Bruno/Postman.
- Test WebSocket dengan client sederhana.
- Test Android minimal 2 HP.

Acceptance criteria:

- Local stack bisa start dengan satu perintah.
- Smoke test local terdokumentasi.

## Phase 14 - Security Hardening

### Tujuan

Menyiapkan sistem agar aman untuk production awal.

Task:

- HTTPS wajib.
- JWT secret minimal 32 byte random.
- Password hash bcrypt/argon2id.
- Rate limit login.
- WebSocket auth.
- Validasi input event.
- Role permission.
- CORS dibatasi.

Acceptance criteria:

- Secret tidak ada di repo.
- Protected endpoint gagal tanpa JWT.
- Login brute force sederhana dibatasi.

## Phase 15 - Deployment Live

### Tujuan

Menjalankan sistem di VPS production.

Task:

- Dockerfile api-server.
- Dockerfile dispatcher-web.
- `infra/docker/docker-compose.prod.yml`.
- `infra/caddy/Caddyfile`.
- Migration production.
- Admin pertama.
- Smoke test live.

Acceptance criteria:

- Dispatcher bisa dibuka via HTTPS.
- API bisa diakses via HTTPS.
- Android bisa login.
- WebSocket connect.
- GPS muncul.
- SOS muncul.
- PTT A ke B jalan.

## Phase 16 - Monitoring dan Backup

### Tujuan

Sistem bisa dipelihara setelah live.

Task:

- Health endpoint.
- Log service.
- Backup harian PostgreSQL.
- Retention policy.
- Uptime Kuma atau monitoring ringan.
- Disk usage alert.

Acceptance criteria:

- Backup berhasil direstore di environment test.
- Health check dipantau.

## Phase 17 - Optimasi Setelah MVP

Masuk setelah MVP stabil:

- Redis pub/sub untuk horizontal scaling.
- Audio recording.
- GPS history playback.
- Dispatcher PTT browser.
- WebRTC/SFU upgrade.
- CCTV/go2rtc integration.
- Multi-tenant.

## Milestone Ringkas

### Milestone 1 - Repo Ready

- Struktur repo selesai.
- Bun workspace siap.
- Go service bootstrap.
- Docker local PostgreSQL + Redis.

### Milestone 2 - API Ready

- Auth jalan.
- CRUD user/group jalan.
- Database migration jalan.

### Milestone 3 - Realtime Ready

- WebSocket auth jalan.
- Presence, GPS, SOS jalan.

### Milestone 4 - Client Ready

- Android login + WS.
- Dispatcher login + WS.
- MapTalks marker realtime.

### Milestone 5 - PTT MVP Ready

- Android A bicara ke Android B.
- Talk lock jalan.
- Busy state jalan.

### Milestone 6 - Live Ready

- Docker production.
- HTTPS.
- Backup.
- Smoke test selesai.

## Risiko Teknis

### Risiko 1 - Audio Latency

Mitigasi:

- Gunakan Opus frame 20 ms.
- Broadcast langsung dari memory.
- Jangan simpan audio sebelum broadcast.
- Hindari payload JSON untuk audio.

### Risiko 2 - Android Background Kill

Mitigasi:

- Gunakan Foreground Service.
- Minta user whitelist battery optimization.
- Tampilkan status koneksi jelas.

### Risiko 3 - GPS Boros Baterai

Mitigasi:

- Interval adaptif.
- Update cepat saat bergerak, lambat saat diam.
- Jangan update marker terlalu sering jika koordinat tidak berubah berarti.

### Risiko 4 - Jaringan Seluler Jelek

Mitigasi:

- Reconnect otomatis.
- Playback buffer kecil.
- Timeout dan retry jelas.

### Risiko 5 - Banyak User Bicara Bersamaan

Mitigasi:

- Talk lock per group.
- Busy state jelas.
- Queue talk optional setelah MVP.
