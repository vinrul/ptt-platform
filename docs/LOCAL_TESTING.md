# Local Testing

Panduan ini menjalankan dependency, migration, seed, backend, dispatcher web,
dan smoke test dalam satu alur development.

## Prerequisites

- Bun.
- Go.
- Docker atau Podman dengan dukungan Compose.

Install dependency workspace:

```bash
bun install
```

## One-Command Local Stack

Jalankan:

```bash
bun run local
```

Script akan:

1. Memilih Docker atau Podman yang tersedia.
2. Menjalankan PostgreSQL, Redis, dan Pgweb.
3. Menjalankan migration database.
4. Membuat seed user dan grup lokal secara idempotent.
5. Menjalankan API dan dispatcher web.
6. Menjalankan smoke test REST dan WebSocket.

URL development:

- Dispatcher: `http://localhost:5173`
- API: `http://localhost:8080`
- Pgweb: `http://localhost:8081`

Tekan `Ctrl+C` untuk menghentikan API dan dispatcher. Container PostgreSQL,
Redis, dan Pgweb tetap berjalan agar startup berikutnya lebih cepat.

## Local Seed

User seed:

| Username | Role |
| --- | --- |
| `admin` | `super_admin` |
| `dispatcher` | `dispatcher` |
| `field1` | `field_user` |
| `field2` | `field_user` |

Password default seluruh user:

```text
ptt-local-123
```

Override password dengan environment variable:

```bash
SEED_PASSWORD='password-lokal-baru' bun run local
```

Seed hanya berjalan saat `APP_ENV=local`, kecuali
`ALLOW_NON_LOCAL_SEED=true` diberikan secara eksplisit.

## Individual Commands

Menjalankan seed ulang:

```bash
bun run seed:local
```

Menjalankan smoke test terhadap API yang sudah aktif:

```bash
bun run smoke:local
```

Smoke test memeriksa:

- Health dan readiness.
- Login dan endpoint protected.
- User dan membership grup.
- Tiga koneksi WebSocket.
- Join grup.
- PTT grant, busy, dan stop.
- SOS create dan acknowledge.

Menghentikan dependency:

```bash
bun run docker:down
```

Untuk Podman:

```bash
bun run podman:down
```

## Custom PostgreSQL Credentials

`bun run local` mengikuti `POSTGRES_USER`, `POSTGRES_PASSWORD`, dan
`POSTGRES_DB`. Untuk URL host yang sepenuhnya custom, gunakan:

```bash
LOCAL_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable' bun run local
```

Redis host dapat dioverride dengan `LOCAL_REDIS_URL`.

## Android Device Test

Setelah `bun run local` aktif:

1. Pastikan komputer dan perangkat Android berada di jaringan LAN yang sama.
2. Gunakan IP LAN komputer sebagai server URL, bukan `localhost`.
3. Login perangkat pertama sebagai `field1`.
4. Login perangkat kedua sebagai `field2`.
5. Join keduanya ke `Default Patrol`.
6. Verifikasi GPS dan presence muncul di dispatcher.
7. Verifikasi SOS muncul dan dapat di-ack.
8. Verifikasi PTT dari perangkat A terdengar di perangkat B.
9. Verifikasi perangkat B menerima busy saat perangkat A sedang bicara.

Emulator Android menggunakan `http://10.0.2.2:8080`.
