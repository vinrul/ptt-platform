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

### Nginx + Cloudflare Origin SSL

Konfigurasi alternatif untuk domain tunggal `ptt.vinrul.my.id` tersedia di:

```text
infra/nginx/nginx.conf
```

Konfigurasi tersebut:

- Mendengarkan HTTP `80` dan HTTPS `443`.
- Mengarahkan HTTP ke HTTPS.
- Memakai Cloudflare Origin Certificate.
- Mengaktifkan Cloudflare Authenticated Origin Pull.
- Meneruskan web, REST API, dan WebSocket ke aplikasi pada
  `127.0.0.1:9910`.

Pasang konfigurasi:

```bash
sudo cp infra/nginx/nginx.conf \
  /etc/nginx/sites-available/ptt.vinrul.my.id.conf
sudo ln -s /etc/nginx/sites-available/ptt.vinrul.my.id.conf \
  /etc/nginx/sites-enabled/ptt.vinrul.my.id.conf
sudo nginx -t
sudo systemctl reload nginx
```

File sertifikat yang harus tersedia:

```text
/etc/ssl/vinrul_my_id_cert.pem
/etc/ssl/vinrul_my_id_key.pem
/etc/ssl/cloudflare.crt
```

Permission private key:

```bash
sudo chown root:root /etc/ssl/vinrul_my_id_key.pem
sudo chmod 600 /etc/ssl/vinrul_my_id_key.pem
```

Cloudflare:

- DNS `ptt.vinrul.my.id` harus berstatus proxied.
- SSL/TLS encryption mode memakai `Full (strict)`.
- Authenticated Origin Pulls harus aktif.

Karena `ssl_verify_client on`, akses HTTPS langsung ke origin tanpa sertifikat
client Cloudflare akan ditolak. Port aplikasi `9910` sebaiknya hanya bind ke
`127.0.0.1` dan tidak dibuka ke internet.

Konfigurasi web dispatcher statis tersedia di:

```text
infra/nginx/ptt-dispatcher.vinrul.my.id.conf
```

Konfigurasi tersebut melayani React SPA dari `/var/www/dist-ptt-fleet` melalui
`https://ptt-dispatcher.vinrul.my.id`, termasuk fallback route ke `index.html`
dan cache jangka panjang untuk aset Vite.

Pasang konfigurasi dispatcher:

```bash
sudo cp infra/nginx/ptt-dispatcher.vinrul.my.id.conf \
  /etc/nginx/sites-available/ptt-dispatcher.vinrul.my.id.conf
sudo ln -s /etc/nginx/sites-available/ptt-dispatcher.vinrul.my.id.conf \
  /etc/nginx/sites-enabled/ptt-dispatcher.vinrul.my.id.conf
sudo nginx -t
sudo systemctl reload nginx
```

DNS `ptt-dispatcher.vinrul.my.id` juga harus berstatus proxied di Cloudflare.

Environment API:

```env
CORS_ALLOWED_ORIGINS=https://ptt-dispatcher.vinrul.my.id
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
CORS_ALLOWED_ORIGINS=https://ptt.example.com
TRUSTED_PROXIES=172.16.0.0/12
LOGIN_RATE_LIMIT=10
LOGIN_RATE_WINDOW_SECONDS=60
PUBLIC_API_URL=https://api.ptt.example.com
PUBLIC_WS_URL=wss://api.ptt.example.com/ws
```

Jangan commit `.env` production.

`TRUSTED_PROXIES` harus disesuaikan dengan subnet network Docker pada server.
Jangan gunakan `0.0.0.0/0`. Checklist keamanan lengkap tersedia di
`docs/SECURITY.md`.

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

## Native API Build

Backend dapat dibuild sebagai binary Linux tanpa Docker:

```bash
bun run build:api
```

Default target:

```text
GOOS=linux
GOARCH=amd64
CGO_ENABLED=0
```

Bundle dihasilkan di:

```text
ptt-fleet/api-server/
  api-server
  migrate
  migrations/
```

Target dapat dioverride untuk server ARM64:

```bash
API_BUILD_GOARCH=arm64 bun run build:api
```

PowerShell:

```powershell
$env:API_BUILD_GOARCH = "arm64"
bun run build:api
```

## Dispatcher Web Build

Build production dispatcher dan ZIP deployment:

```bash
bun run build:web
```

Output:

```text
ptt-fleet/
  dist-ptt-fleet/
  dist-ptt-fleet.zip
```

ZIP berisi folder `dist-ptt-fleet`. Default endpoint yang ditanam saat build:

```text
VITE_API_URL=https://ptt.vinrul.my.id
VITE_WS_URL=wss://ptt.vinrul.my.id/ws
```

Endpoint dapat dioverride saat build menggunakan environment `VITE_API_URL`
dan `VITE_WS_URL`.

Deploy dispatcher:

```bash
sudo rm -rf /var/www/dist-ptt-fleet
sudo unzip ptt-fleet/dist-ptt-fleet.zip -d /var/www
sudo chown -R www-data:www-data /var/www/dist-ptt-fleet
sudo nginx -t
sudo systemctl reload nginx
```

## Native Server Layout

Contoh instalasi binary:

```bash
sudo useradd --system --home /opt/ptt-fleet --shell /usr/sbin/nologin ptt-fleet
sudo mkdir -p /opt/ptt-fleet/api-server /etc/ptt-fleet /var/log/ptt-fleet
sudo cp -a ptt-fleet/api-server/. /opt/ptt-fleet/api-server/
sudo chown -R ptt-fleet:ptt-fleet /opt/ptt-fleet /var/log/ptt-fleet
sudo chmod 755 /opt/ptt-fleet/api-server/api-server
sudo chmod 755 /opt/ptt-fleet/api-server/migrate
```

Pasang environment:

```bash
sudo cp infra/env/api-server.env.example /etc/ptt-fleet/api-server.env
sudo nano /etc/ptt-fleet/api-server.env
sudo chown root:ptt-fleet /etc/ptt-fleet/api-server.env
sudo chmod 640 /etc/ptt-fleet/api-server.env
```

Environment contoh memakai `API_PORT=9910` agar sesuai dengan upstream Nginx
`127.0.0.1:9910`. Jangan membuka port tersebut ke internet.

Jalankan migration sebelum restart aplikasi:

```bash
cd /opt/ptt-fleet/api-server
set -a
. /etc/ptt-fleet/api-server.env
set +a
./migrate up
```

## Run With systemd

Template:

```text
infra/systemd/ptt-fleet-api.service
```

Install:

```bash
sudo cp infra/systemd/ptt-fleet-api.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ptt-fleet-api
sudo systemctl status ptt-fleet-api
```

Log dan operasi:

```bash
sudo journalctl -u ptt-fleet-api -f
sudo systemctl restart ptt-fleet-api
sudo systemctl stop ptt-fleet-api
```

## Run With Supervisor

Gunakan Supervisor hanya jika systemd tidak dipakai untuk proses API yang sama.

Template:

```text
infra/supervisor/ptt-fleet-api.conf
infra/supervisor/run-with-env.sh
```

Install:

```bash
sudo cp infra/supervisor/run-with-env.sh \
  /opt/ptt-fleet/api-server/run-with-env.sh
sudo chmod 755 /opt/ptt-fleet/api-server/run-with-env.sh
sudo chown ptt-fleet:ptt-fleet /opt/ptt-fleet/api-server/run-with-env.sh

sudo cp infra/supervisor/ptt-fleet-api.conf \
  /etc/supervisor/conf.d/ptt-fleet-api.conf
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl status ptt-fleet-api
```

Log dan operasi:

```bash
sudo supervisorctl tail -f ptt-fleet-api
sudo supervisorctl restart ptt-fleet-api
sudo supervisorctl stop ptt-fleet-api
```

Setelah service aktif:

```bash
curl http://127.0.0.1:9910/healthz
curl http://127.0.0.1:9910/readyz
sudo nginx -t
sudo systemctl reload nginx
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
