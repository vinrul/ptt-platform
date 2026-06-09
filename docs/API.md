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

## Groups

### GET /api/groups

Response:

```json
{
  "items": []
}
```

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

### PATCH /api/devices/:id

Request:

```json
{
  "deviceName": "Android Patrol 1",
  "status": "active"
}
```

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
- `from`
- `to`
- `page`
- `pageSize`
