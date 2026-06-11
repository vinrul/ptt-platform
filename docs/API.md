# REST API Contract

Base URL local:

```text
http://localhost:8080
```

Base path:

```text
/api
```

## Authentication

Protected endpoint memakai header:

```text
Authorization: Bearer <access_token>
```

Access token TTL default: 15 menit.

Refresh token TTL default: 720 jam.

Role authorization untuk endpoint user dan group:

- `super_admin`: akses penuh user, group, dan membership.
- `dispatcher`: baca user/group serta create, update, dan disable `field_user`.
- `supervisor`: read-only user dan group.
- `field_user`: tidak boleh mengakses endpoint administrasi.

## Response Format

Success response boleh langsung mengembalikan resource:

```json
{
  "id": "uuid",
  "username": "dispatcher1"
}
```

List response:

```json
{
  "items": [],
  "page": 1,
  "pageSize": 50,
  "total": 0
}
```

Error response:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Human readable message",
    "details": {}
  }
}
```

## Public Endpoints

### GET /healthz

Checks process health.

Response:

```json
{
  "status": "ok"
}
```

### GET /readyz

Checks database readiness.

Response:

```json
{
  "status": "ready"
}
```

### POST /api/auth/login

Request:

```json
{
  "username": "admin",
  "password": "secret",
  "deviceName": "Android 1",
  "clientType": "android"
}
```

Response:

```json
{
  "accessToken": "jwt",
  "refreshToken": "token",
  "user": {
    "id": "uuid",
    "username": "admin",
    "fullName": "Admin",
    "role": "super_admin"
  }
}
```

Login dibatasi per kombinasi IP client dan username. Jika limit terlampaui:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
```

```json
{
  "error": {
    "code": "rate_limited",
    "message": "Too many login attempts. Try again later",
    "details": {}
  }
}
```

### POST /api/auth/refresh

Request:

```json
{
  "refreshToken": "token"
}
```

Response:

```json
{
  "accessToken": "jwt",
  "refreshToken": "new-token",
  "user": {
    "id": "uuid",
    "username": "admin",
    "fullName": "Admin",
    "role": "super_admin",
    "status": "active"
  }
}
```

Refresh token dirotasi setiap kali dipakai. Token lama langsung di-revoke.

## Authenticated Endpoints

### POST /api/auth/logout

Request:

```json
{
  "refreshToken": "token"
}
```

Response:

```json
{
  "ok": true
}
```

### GET /api/auth/me

Response:

```json
{
  "id": "uuid",
  "username": "dispatcher1",
  "fullName": "Dispatcher 1",
  "role": "dispatcher",
  "status": "active"
}
```

### POST /api/auth/change-password

Request:

```json
{
  "currentPassword": "old-password",
  "newPassword": "new-password"
}
```

Password baru minimal 8 karakter dan harus berbeda dari password lama. Setelah
berhasil, seluruh refresh token user dicabut sehingga perangkat harus login
ulang. Perubahan dicatat sebagai audit action `auth.password_changed`.

Response:

```json
{
  "ok": true
}
```

## Users

### GET /api/users

Query:

- `page`
- `pageSize`
- `role`
- `status`
- `q`

### POST /api/users

Request:

```json
{
  "username": "field1",
  "password": "secret",
  "fullName": "Field User 1",
  "role": "field_user"
}
```

Password minimal 8 karakter. Dispatcher hanya boleh membuat user dengan role
`field_user`.

### GET /api/users/:id

Returns user detail.

### PATCH /api/users/:id

Request:

```json
{
  "fullName": "Field User One",
  "status": "active",
  "role": "field_user"
}
```

Hanya super admin yang boleh mengubah role. Dispatcher hanya boleh mengubah
`field_user`.

### DELETE /api/users/:id

MVP behavior: soft delete by setting `status = disabled`.

User tidak boleh menonaktifkan akunnya sendiri.

### POST /api/users/:id/reset-password

Khusus role `super_admin`.

Request:

```json
{
  "newPassword": "new-password"
}
```

Password minimal 8 karakter. Setelah berhasil, seluruh refresh token target
dicabut dan perubahan dicatat sebagai audit action `user.password_reset`.

### GET /api/users/:id/gps-history

Mengambil riwayat lokasi user. Hanya role `super_admin` dan `dispatcher` yang
diizinkan.

Query parameter:

- `from`: waktu awal RFC3339, default 24 jam sebelum `to`.
- `to`: waktu akhir RFC3339, default waktu server saat request.
- `limit`: jumlah maksimum titik, default `200`, maksimum `1000`.

Response:

```json
{
  "user": {
    "id": "uuid",
    "username": "field1",
    "fullName": "Field User One",
    "role": "field_user",
    "status": "active"
  },
  "items": [
    {
      "userId": "uuid",
      "lat": -8.65,
      "lng": 115.2167,
      "speed": 1.2,
      "heading": 90,
      "accuracy": 8,
      "recordedAt": "2026-06-11T09:00:00Z"
    }
  ]
}
```

Item diurutkan dari lokasi terbaru ke lokasi terlama.

## Groups

### GET /api/groups

Response:

```json
{
  "items": []
}
```

Role operator menerima seluruh grup. Role `field_user` hanya menerima grup yang
terdaftar pada `group_members`, sehingga Android dapat memilih channel tanpa
melihat grup lain.

### POST /api/groups

Request:

```json
{
  "name": "Default Patrol",
  "description": "Default field patrol group"
}
```

### GET /api/groups/:id

Returns group detail beserta array `members`.

Role `field_user` hanya dapat membaca detail grup jika user tersebut terdaftar
sebagai anggota grup. Data ini digunakan Android untuk menampilkan target PTT
grup dan privat. Setiap member memuat `role` akun dan `roleInGroup`. Android
tidak menampilkan akun dengan role `super_admin` atau `dispatcher` sebagai
target PTT.

### PATCH /api/groups/:id

Request:

```json
{
  "name": "Patrol A",
  "description": "Updated description"
}
```

### DELETE /api/groups/:id

MVP behavior: reject delete if group still has active members.

### POST /api/groups/:id/members

Request:

```json
{
  "userId": "uuid",
  "roleInGroup": "member"
}
```

`roleInGroup` harus salah satu dari `member`, `dispatcher`, atau `supervisor`.

### DELETE /api/groups/:id/members/:userId

Removes member from group.

## Common Status Codes

- `400`: request atau filter tidak valid.
- `401`: access/refresh token tidak valid atau kedaluwarsa.
- `403`: role tidak memiliki izin atau akun disabled.
- `404`: resource tidak ditemukan.
- `409`: username/name duplicate, membership duplicate, atau group masih berisi member.

## Devices

### GET /api/devices

Returns registered devices and last seen.

### GET /api/devices/:id

Returns device detail.

Device management Phase 12 bersifat read-only untuk `super_admin`, `dispatcher`,
dan `supervisor`. Device dibuat saat login dan `lastSeenAt` diperbarui saat
refresh token berhasil.

## SOS

### GET /api/sos-events

Query:

- `status`
- `from`
- `to`
- `userId`

### POST /api/sos-events/:id/ack

Acknowledges SOS event.

Response:

```json
{
  "id": "uuid",
  "status": "ack",
  "acknowledgedBy": "uuid"
}
```

## Audit Logs

### GET /api/audit-logs

Query:

- `actorUserId`
- `action`
- `page`
- `pageSize`

Audit log bersifat read-only untuk seluruh role operator. Field user menerima
`403`.
- `action`
- `from`
- `to`
- `page`
- `pageSize`
