# API Server

Go + Gin backend untuk REST API dan realtime gateway PTT Fleet Platform.

Fitur yang tersedia:

- Health dan readiness check.
- Goose migration runner.
- Login JWT access token.
- Refresh token rotation dan logout revoke.
- Protected `/api/auth/me`.
- User CRUD dengan soft delete.
- Group CRUD dan membership.
- Role authorization untuk super admin, dispatcher, supervisor, dan field user.
- WebSocket `/ws?token=<jwt>` dengan JWT dan active-user validation.
- Connection registry, presence per user, heartbeat timeout, dan group join.
- Validasi dan persistence `gps.update` ke PostgreSQL.
- Broadcast `gps.updated` terbatas ke role operator.

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

Run test:

```bash
GOCACHE=/tmp/ptt-go-build go test ./...
```

Test package WebSocket membuka listener localhost sementara melalui
`httptest.Server`.

## Manual Test Notes

1. Jalankan PostgreSQL dan Redis.
2. Jalankan migration.
3. Siapkan user dengan password bcrypt di tabel `users`; seed resmi ditambahkan
   pada Phase 13 Local Testing.
4. Login melalui `POST /api/auth/login`.
5. Pakai `Authorization: Bearer <access_token>` untuk endpoint protected.
6. Pastikan refresh merotasi token dan logout membuat refresh token tidak bisa
   dipakai lagi.
7. Connect ke `ws://localhost:8080/ws?token=<access_token>`.
8. Pastikan event pertama adalah `connection.ready`, lalu kirim `heartbeat`
   setiap 20-30 detik.
9. Kirim `gps.update` dari field user dan pastikan row masuk ke `gps_logs`.
10. Pastikan hanya koneksi operator menerima `gps.updated`.

Kontrak REST tersedia di `docs/API.md`; kontrak realtime tersedia di
`docs/WEBSOCKET_PROTOCOL.md`.
