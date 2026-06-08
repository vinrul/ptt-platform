# AGENTS.md

Panduan kerja untuk AI agent / developer agent yang membantu membangun proyek
**PTT Fleet Platform**.

Dokumen ini adalah sumber aturan kerja utama. Jika ada perbedaan dengan dokumen
lain, ikuti dokumen ini dan perbarui dokumen lain agar konsisten.

## 1. Tujuan Proyek

Membangun sistem Push-to-Talk over IP sederhana seperti WalkieFleet versi mandiri,
dengan target awal sistem bisa berjalan end-to-end untuk tim kecil.

Komponen utama:

- Android client berbasis Kotlin native.
- Web dispatcher berbasis Bun, React, TypeScript, dan MapTalks.
- Backend utama menggunakan Go + Gin.
- Realtime event dan audio menggunakan WebSocket.
- Audio PTT MVP menggunakan Opus frame relay via WebSocket binary.
- GPS tracking realtime.
- SOS/emergency event.
- Admin panel untuk user, grup, perangkat, dan log dasar.
- Deployment live menggunakan Docker Compose di VPS Ubuntu.

## 2. Prinsip Utama Agent

Agent harus memprioritaskan:

1. Sistem bisa berjalan end-to-end terlebih dahulu.
2. Hindari over-engineering pada fase awal.
3. Gunakan Go + Gin untuk backend API dan realtime gateway.
4. Gunakan Kotlin native untuk Android agar kontrol audio lebih baik.
5. Gunakan Bun untuk tooling frontend dan workspace script.
6. Gunakan MapTalks untuk map dispatcher.
7. Gunakan PostgreSQL sebagai database utama.
8. Gunakan Redis untuk presence, lock, dan pub/sub jika diperlukan; Redis boleh
   disiapkan sejak lokal agar production path tidak berubah.
9. Semua fitur besar dibuat bertahap berdasarkan PLAN.md.
10. Semua endpoint, event realtime, dan schema database harus terdokumentasi.

## 3. Stack Final MVP

### Backend

- Language: Go.
- HTTP framework: Gin.
- Realtime: WebSocket.
- WebSocket library: gorilla/websocket.
- Audio relay: Opus frame relay via WebSocket binary.
- Database: PostgreSQL.
- SQL access: pgx atau sqlc + pgx. Untuk MVP awal boleh pgx langsung.
- Migration: goose.
- Cache/presence: Redis.
- Auth: JWT access token + refresh token.
- Password hash: bcrypt atau argon2id.
- Deployment: Docker.

### Web Dispatcher

- Runtime/tooling: Bun.
- Framework: React + TypeScript + Vite.
- Map: MapTalks.
- Realtime: WebSocket.
- State: Zustand untuk state realtime ringan.
- Server data fetching: TanStack Query jika REST flow mulai banyak.
- UI: Tailwind CSS; shadcn/ui opsional setelah layout dasar stabil.

### Android

- Language: Kotlin.
- Min SDK: 26+.
- Audio capture: AudioRecord.
- Audio playback: AudioTrack.
- Codec: Opus library Android.
- Realtime: OkHttp WebSocket.
- GPS: FusedLocationProviderClient.
- Background mode: Foreground Service.
- Token storage: EncryptedSharedPreferences atau Jetpack Security + DataStore.
- PTT button: touch hold-to-talk dulu, hardware key setelah MVP suara stabil.

### Infrastructure

- VPS Ubuntu 22.04/24.04.
- Docker Compose.
- Caddy sebagai reverse proxy HTTPS utama.
- PostgreSQL volume backup.
- Redis untuk presence/lock/pubsub.
- Optional setelah MVP: S3-compatible storage untuk rekaman audio.

## 4. Role Agent

### 4.1 Project Planner Agent

Tugas:

- Menjaga roadmap tetap sesuai PLAN.md.
- Memecah fitur menjadi task kecil.
- Menentukan prioritas MVP.
- Mencegah scope creep.

Output wajib:

- Task list.
- Acceptance criteria.
- Risiko teknis.

### 4.2 Backend Agent

Tugas:

- Membuat service Go + Gin.
- Membuat REST API.
- Membuat WebSocket gateway.
- Mengelola auth, user, group, device, session.
- Mengelola PTT talk lock dan audio relay.
- Menyimpan log GPS dan event SOS.

Aturan:

- Jangan simpan password plaintext.
- Semua endpoint protected wajib cek JWT.
- Semua event WebSocket JSON wajib punya `type`, `timestamp`, dan `payload`.
- Gunakan package internal terpisah: `auth`, `users`, `groups`, `devices`,
  `ws`, `ptt`, `gps`, `sos`, `config`, `db`.
- Gunakan context timeout untuk query database.
- Jangan broadcast event ke user yang tidak berhak menerima.
- Endpoint dan event baru harus ditambahkan ke `docs/API.md` atau
  `docs/WEBSOCKET_PROTOCOL.md`.

### 4.3 Android Agent

Tugas:

- Membuat Android Kotlin app.
- Login user.
- Connect WebSocket.
- Join group.
- PTT hold-to-talk.
- Capture audio, encode Opus, kirim frame.
- Receive audio frame, decode, playback.
- Kirim GPS periodik.
- Kirim SOS.

Aturan:

- Audio tidak boleh berjalan tanpa izin user.
- Gunakan Foreground Service untuk mode patrol.
- Handle reconnect otomatis dengan exponential backoff.
- Simpan token secara aman.
- Jangan mulai audio capture sebelum server mengirim `ptt.granted`.
- Release audio resource saat talk stop, logout, atau disconnect.

### 4.4 Web Dispatcher Agent

Tugas:

- Membuat dashboard operator.
- Menampilkan user online/offline.
- Menampilkan marker MapTalks realtime.
- Menampilkan SOS popup/alarm.
- Menampilkan riwayat event dasar.
- Menyediakan PTT browser setelah Android-to-Android stabil.

Aturan:

- Gunakan MapTalks sebagai peta utama.
- WebSocket reconnect otomatis.
- Marker harus update tanpa reload.
- Jangan render ulang seluruh map untuk update posisi kecil.
- Pisahkan state realtime dari komponen map agar marker bisa di-update langsung.

### 4.5 DevOps Agent

Tugas:

- Menyiapkan Dockerfile dan docker-compose.
- Menyiapkan Caddy HTTPS.
- Menyiapkan backup database.
- Menyiapkan monitoring awal.
- Menyiapkan environment local, staging, dan production.

Aturan:

- Secret tidak boleh commit ke repo.
- Gunakan `.env.example`.
- Semua service local harus bisa dijalankan dengan satu perintah.
- Backup dan rollback harus terdokumentasi sebelum production.

## 5. Struktur Folder

```text
ptt-fleet/
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

## 6. Konvensi REST API

Base path:

```text
/api
```

Endpoint public:

```text
GET  /healthz
GET  /readyz
POST /api/auth/login
POST /api/auth/refresh
```

Endpoint protected wajib memakai JWT access token:

```text
Authorization: Bearer <access_token>
```

Format error:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Human readable message",
    "details": {}
  }
}
```

## 7. Konvensi Event WebSocket JSON

Semua event JSON menggunakan format:

```json
{
  "type": "event.name",
  "requestId": "optional-id",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {}
}
```

Nama event MVP:

- `connection.ready`
- `presence.updated`
- `group.join`
- `group.joined`
- `gps.update`
- `gps.updated`
- `sos.create`
- `sos.created`
- `sos.ack`
- `sos.acked`
- `ptt.start`
- `ptt.granted`
- `ptt.busy`
- `ptt.started`
- `ptt.stop`
- `ptt.stopped`
- `heartbeat`
- `error`

## 8. Konvensi Audio Binary

Audio frame binary menggunakan envelope ringan:

```text
[1 byte frame_type][16 byte session_uuid][8 byte sequence_be][opus_payload]
```

Ketentuan:

- `session_uuid` adalah UUID talk session dalam bentuk 16 byte binary.
- `sequence_be` adalah unsigned 64-bit big-endian.
- `opus_payload` adalah frame Opus mono 20 ms.
- Server hanya relay binary frame jika talk session valid dan speaker masih
  memegang talk lock.

Frame type awal:

- `0x01` = audio uplink dari speaker ke server.
- `0x02` = audio downlink dari server ke listener.
- `0x05` = heartbeat binary optional.

Event start/stop tetap menggunakan JSON, bukan binary.

## 9. MVP Acceptance Criteria

MVP dianggap berhasil jika:

1. Admin bisa membuat user dan grup.
2. Android bisa login.
3. Android bisa join grup.
4. User A tekan PTT, User B mendengar suara.
5. User B menerima busy saat A sedang bicara di grup yang sama.
6. Web dispatcher melihat status online/offline.
7. Web dispatcher melihat marker GPS user di MapTalks.
8. Android bisa mengirim SOS.
9. Dispatcher menerima alarm SOS.
10. Sistem bisa live di VPS dengan HTTPS.
11. Ada dokumentasi deploy, backup, dan smoke test.

## 10. Hal yang Jangan Dilakukan di Awal

Jangan mulai dari:

- WebRTC SFU kompleks.
- End-to-end encryption penuh.
- Native DMR gateway.
- Recording semua komunikasi.
- Multi-tenant billing.
- White-label app.
- Kubernetes.
- Dispatcher PTT browser sebelum Android-to-Android stabil.

Semua itu masuk fase lanjutan setelah MVP stabil.

## 11. Perintah Kerja Umum

Install dependency frontend:

```bash
bun install
```

Run dispatcher web:

```bash
bun --filter dispatcher-web dev
```

Run backend Go:

```bash
cd services/api-server
go run ./cmd/server
```

Run stack lokal:

```bash
docker compose -f infra/docker/docker-compose.local.yml up -d
```

Run migration local:

```bash
cd services/api-server
goose -dir migrations postgres "$DATABASE_URL" up
```

## 12. Definition of Done

Sebuah task dianggap selesai jika:

- Code berjalan lokal.
- Ada minimal test atau manual test note.
- Error utama sudah di-handle.
- Dokumentasi singkat ditambahkan.
- Tidak ada secret di repo.
- Format dan lint lolos.
- Endpoint/event/schema baru terdokumentasi.
