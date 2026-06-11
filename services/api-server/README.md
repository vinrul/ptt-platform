# API Server

Go + Gin backend untuk REST API dan realtime gateway PTT Fleet Platform.

Fitur yang tersedia:

- Health dan readiness check.
- Goose migration runner.
- Login JWT access token.
- Redis-backed login rate limit.
- Refresh token rotation dan logout revoke.
- Protected `/api/auth/me`.
- User CRUD dengan soft delete.
- Group CRUD dan membership.
- Role authorization untuk super admin, dispatcher, supervisor, dan field user.
- WebSocket `/ws?token=<jwt>` dengan JWT dan active-user validation.
- CORS dan WebSocket origin allowlist.
- HTTPS guard untuk production di belakang trusted reverse proxy.
- Connection registry, presence per user, heartbeat timeout, dan group join.
- Validasi dan persistence `gps.update` ke PostgreSQL.
- Broadcast `gps.updated` ke operator dan anggota yang sedang join pada grup
  pengirim.
- Create dan acknowledge SOS melalui WebSocket dengan audit log transaksional.
- Broadcast `sos.created` dan `sos.acked` terbatas ke role operator.
- Talk lock in-memory per grup dengan persistence metadata `talk_sessions`.
- Relay frame Opus binary tanpa decode, terbatas ke koneksi yang join grup sama.
- Disconnect speaker otomatis melepas talk lock.
- Device registry read-only untuk operator.
- Audit log list dengan filter actor/action dan pagination.

## Local Commands

Run migration from host after PostgreSQL is available:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/migrate down
```

Run API:

```bash
go run ./cmd/server
```

Seed user dan grup lokal:

```bash
go run ./cmd/seed
```

Seed bersifat idempotent, hanya diizinkan untuk `APP_ENV=local`, dan membuat
user `admin`, `dispatcher`, `field1`, serta `field2`. Password default adalah
`ptt-local-123` dan dapat dioverride melalui `SEED_PASSWORD`.

Run test:

```bash
GOCACHE=/tmp/ptt-go-build go test ./...
```

Test package WebSocket membuka listener localhost sementara melalui
`httptest.Server`.

## Manual Test Notes

1. Jalankan PostgreSQL dan Redis.
2. Jalankan migration.
3. Jalankan `go run ./cmd/seed` untuk membuat user dan grup local.
4. Login melalui `POST /api/auth/login`.
5. Pakai `Authorization: Bearer <access_token>` untuk endpoint protected.
6. Pastikan refresh merotasi token dan logout membuat refresh token tidak bisa
   dipakai lagi.
7. Connect ke `ws://localhost:8080/ws?token=<access_token>`.
8. Pastikan event pertama adalah `connection.ready`, lalu kirim `heartbeat`
   setiap 20-30 detik.
9. Kirim `gps.update` dari field user dan pastikan row masuk ke `gps_logs`.
10. Pastikan operator dan anggota grup aktif menerima `gps.updated`, sedangkan
    anggota grup lain tidak menerimanya.
11. Kirim `sos.create`, pastikan row `sos_events` dan audit `sos.create` dibuat.
12. Ack dari dispatcher, lalu pastikan status menjadi `ack` dan audit `sos.ack`
    dibuat.
13. Join dua client ke grup sama dan pastikan client kedua menerima `ptt.busy`
    saat speaker pertama aktif.
14. Kirim frame binary uplink dan pastikan listener menerima downlink dengan
    frame type `0x02`.
15. Login dari Android/web dan pastikan device muncul melalui `GET /api/devices`.
16. Pastikan `GET /api/audit-logs` menampilkan aksi auth, user, group, SOS, dan
    PTT sesuai role operator.

Kontrak REST tersedia di `docs/API.md`; kontrak realtime tersedia di
`docs/WEBSOCKET_PROTOCOL.md`.
