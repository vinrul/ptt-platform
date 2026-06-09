# Deployment

Dokumen ini berisi checklist deployment local dan production.

## Development With Docker

Development disiapkan berbasis Docker Compose sejak awal. Minimal service yang
wajib berjalan untuk backend development:

- PostgreSQL.
- Redis.
- Pgweb untuk inspeksi database.

Service aplikasi (`api-server` dan `dispatcher-web`) disiapkan dalam profile
`app`, sehingga bisa diaktifkan setelah Dockerfile masing-masing tersedia.

## Local Services

Service local minimal:

- PostgreSQL.
- Redis.
- Pgweb.
- api-server.
- dispatcher-web.

Command target:

```bash
bun run local
```

Command ini mendeteksi Docker atau Podman, menjalankan dependency, migration,
seed, API, dispatcher web, dan smoke test. Panduan lengkap tersedia di
`docs/LOCAL_TESTING.md`.

Menjalankan dependency saja:

```bash
bun run docker:local
```

Run migration local:

```bash
bun run migrate:up
bun run migrate:status
```

Menjalankan seed dan smoke test secara terpisah:

```bash
bun run seed:local
bun run smoke:local
```

Menjalankan backend dalam container:

```bash
bun run docker:api
```

Menjalankan seluruh aplikasi dalam container:

```bash
bun run docker:local:app
```

Environment local:

```env
APP_ENV=local
API_PORT=8080
DATABASE_URL=postgres://ptt:ptt@postgres:5432/ptt_fleet?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=change-me-use-32-byte-random
POSTGRES_DB=ptt_fleet
POSTGRES_USER=ptt
POSTGRES_PASSWORD=ptt
PGWEB_PORT=8081
PUBLIC_API_URL=http://localhost:8080
PUBLIC_WS_URL=ws://localhost:8080/ws
```

Pgweb tersedia di `http://localhost:8081` dan memakai `DATABASE_URL` yang
langsung mengarah ke service `postgres`. Tidak ada setup koneksi manual.

Jika menjalankan backend langsung dari host dengan `go run`, gunakan override:

```env
DATABASE_URL=postgres://ptt:ptt@localhost:5432/ptt_fleet?sslmode=disable
REDIS_URL=redis://localhost:6379
```

Untuk environment `local`, backend juga menyediakan default host development
di atas jika `DATABASE_URL`, `REDIS_URL`, dan `JWT_SECRET` belum di-set.
Production tetap wajib mengisi semua secret dan URL secara eksplisit.

## Production Server

Minimum VPS:

- Ubuntu 22.04 atau 24.04.
- 2 vCPU.
- 4 GB RAM.
- 40 GB SSD.

Recommended:

- 4 vCPU jika user aktif mendekati 100.
- Volume backup terpisah atau object storage.

## Domain

Contoh:

```text
ptt.example.com      -> dispatcher
api.ptt.example.com  -> backend API + WebSocket
```

## Production Services

Compose production harus berisi:

- `api-server`.
- `dispatcher-web`.
- `postgres`.
- `redis`.
- `caddy`.

Caddy menjadi entrypoint HTTPS:

- `ptt.example.com` proxy ke dispatcher.
- `api.ptt.example.com` proxy ke api-server.
- WebSocket upgrade wajib diteruskan.

## Production Environment

Secret wajib dibuat manual di server:

```env
APP_ENV=production
API_PORT=8080
DATABASE_URL=postgres://ptt:<strong-password>@postgres:5432/ptt_fleet?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=<32-byte-random-or-more>
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_HOURS=720
PUBLIC_API_URL=https://api.ptt.example.com
PUBLIC_WS_URL=wss://api.ptt.example.com/ws
```

Jangan commit `.env` production.

## Deployment Steps

```bash
git clone <repo-url> ptt-fleet
cd ptt-fleet
cp .env.example .env
nano .env
docker compose -f infra/docker/docker-compose.prod.yml up -d postgres redis
docker compose -f infra/docker/docker-compose.prod.yml run --rm api-server ./migrate up
docker compose -f infra/docker/docker-compose.prod.yml up -d
```

## First Admin

Target command:

```bash
docker compose -f infra/docker/docker-compose.prod.yml exec api-server ./ptt-admin create-user \
  --role super_admin \
  --username admin
```

Jika command admin belum tersedia, buat seed script sementara yang hanya jalan
manual di production.

## Smoke Test Production

Checklist:

- Dispatcher terbuka via HTTPS.
- API `/healthz` return OK.
- API `/readyz` return OK.
- Login dispatcher berhasil.
- Android login berhasil.
- WebSocket connect via WSS.
- Presence online muncul.
- GPS muncul di MapTalks.
- SOS muncul dan bisa di-ack.
- PTT Android A ke Android B berjalan.
- User B menerima busy saat A bicara.

## Backup

Backup harian target:

```bash
pg_dump "$DATABASE_URL" > backup-$(date +%F).sql
```

Retention:

- Harian 7 hari.
- Mingguan 4 minggu.
- Bulanan 6 bulan.

Acceptance criteria backup:

- Backup file dibuat otomatis.
- Restore pernah diuji di environment non-production.
- Backup tidak disimpan hanya di disk VPS yang sama.

## Rollback

Sebelum migration production:

- Backup database.
- Catat image tag lama.
- Deploy dengan image tag, bukan `latest`.

Rollback aplikasi:

```bash
docker compose -f infra/docker/docker-compose.prod.yml pull
docker compose -f infra/docker/docker-compose.prod.yml up -d
```

Migration down hanya dipakai jika migration terbukti aman untuk di-rollback.
