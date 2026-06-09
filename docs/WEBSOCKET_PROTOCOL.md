# WebSocket Protocol

WebSocket dipakai untuk presence, GPS, SOS, PTT control event, dan audio binary.

Endpoint:

```text
GET /ws?token=<jwt>
```

JWT harus valid. Token invalid ditolak sebelum upgrade atau langsung close
dengan policy violation.

Server juga memeriksa ulang bahwa user masih ada dan berstatus `active` sebelum
melakukan upgrade. Role koneksi diambil dari database agar perubahan role segera
berlaku pada koneksi baru.

Browser wajib mengirim origin yang terdaftar dalam `CORS_ALLOWED_ORIGINS`.
Handshake dari origin lain ditolak dengan HTTP `403`. Android native boleh
terhubung tanpa header `Origin`.

## JSON Event Envelope

Semua event JSON:

```json
{
  "type": "event.name",
  "requestId": "optional-id",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {}
}
```

Rules:

- `type` wajib.
- `timestamp` wajib RFC3339.
- `payload` wajib object, boleh kosong.
- `requestId` optional untuk correlating request/response.
- Server timestamp memakai UTC.

## Server Event: connection.ready

Dikirim setelah koneksi berhasil.

```json
{
  "type": "connection.ready",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "connectionId": "uuid",
    "userId": "uuid",
    "role": "field_user"
  }
}
```

## Heartbeat

Client kirim setiap 20-30 detik:

```json
{
  "type": "heartbeat",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {}
}
```

Server disconnect jika heartbeat hilang lebih dari 90 detik.

Heartbeat tidak menghasilkan event response. Heartbeat yang valid memperpanjang
deadline koneksi selama 90 detik berikutnya.

## Presence

Server broadcast:

```json
{
  "type": "presence.updated",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "userId": "uuid",
    "status": "online",
    "lastSeenAt": "2026-06-08T12:00:00Z"
  }
}
```

Status:

- `online`
- `offline`

## Group Join

Client:

```json
{
  "type": "group.join",
  "requestId": "req-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "groupId": "uuid"
  }
}
```

Server:

```json
{
  "type": "group.joined",
  "requestId": "req-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "groupId": "uuid"
  }
}
```

Server hanya menerima join jika user terdaftar di `group_members`. Join yang
tidak diizinkan menghasilkan event `error` dengan code `forbidden`.

## Supported Events by Phase

Handler realtime saat ini menerima:

- `heartbeat`
- `group.join`
- `gps.update`
- `sos.create`
- `sos.ack` untuk role operator
- `ptt.start`
- `ptt.stop`

Audio Opus dikirim sebagai WebSocket binary setelah server mengirim
`ptt.granted`.

## GPS

Android client sends:

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

Server broadcasts to dispatcher:

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

Server menyimpan setiap update ke `gps_logs`, lalu mengirim `gps.updated` hanya
ke koneksi dengan role `super_admin`, `dispatcher`, atau `supervisor`.

Validation:

- `lat` between -90 and 90.
- `lng` between -180 and 180.
- `speed` tidak boleh negatif jika ada.
- `heading` harus antara 0 (inklusif) dan 360 (eksklusif) jika ada.
- `accuracy` tidak boleh negatif jika ada.

## SOS

Android client sends:

```json
{
  "type": "sos.create",
  "requestId": "req-sos-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "lat": -8.65,
    "lng": 115.21,
    "message": "Emergency"
  }
}
```

Server broadcasts:

```json
{
  "type": "sos.created",
  "requestId": "req-sos-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "id": "uuid",
    "userId": "uuid",
    "lat": -8.65,
    "lng": 115.21,
    "message": "Emergency",
    "status": "open",
    "createdAt": "2026-06-08T12:00:00Z"
  }
}
```

Create SOS dapat dikirim tanpa koordinat. Jika salah satu dari `lat` atau `lng`
diisi, keduanya wajib valid. Server menyimpan SOS dan audit `sos.create` dalam
satu transaksi, lalu broadcast hanya ke role operator.

Dispatcher acknowledges:

```json
{
  "type": "sos.ack",
  "requestId": "req-sos-ack-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "id": "uuid"
  }
}
```

Server broadcasts:

```json
{
  "type": "sos.acked",
  "requestId": "req-sos-ack-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "id": "uuid",
    "status": "ack",
    "acknowledgedBy": "uuid",
    "acknowledgedAt": "2026-06-08T12:00:00Z"
  }
}
```

Hanya `super_admin`, `dispatcher`, dan `supervisor` yang boleh mengirim
`sos.ack`. Ack hanya berhasil untuk SOS berstatus `open`. Perubahan status dan
audit `sos.ack` disimpan dalam satu transaksi.

## PTT Control

Speaker starts talk:

```json
{
  "type": "ptt.start",
  "requestId": "req-ptt-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "groupId": "uuid"
  }
}
```

Server grants:

```json
{
  "type": "ptt.granted",
  "requestId": "req-ptt-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "sessionId": "uuid",
    "groupId": "uuid"
  }
}
```

Server broadcasts:

```json
{
  "type": "ptt.started",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "sessionId": "uuid",
    "groupId": "uuid",
    "speakerUserId": "uuid"
  }
}
```

Server rejects busy:

```json
{
  "type": "ptt.busy",
  "requestId": "req-ptt-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "groupId": "uuid",
    "speakerUserId": "uuid"
  }
}
```

Speaker stops:

```json
{
  "type": "ptt.stop",
  "requestId": "req-ptt-stop-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "sessionId": "uuid"
  }
}
```

Server broadcasts:

```json
{
  "type": "ptt.stopped",
  "requestId": "req-ptt-stop-1",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "sessionId": "uuid",
    "groupId": "uuid",
    "speakerUserId": "uuid",
    "reason": "user_stop"
  }
}
```

Stop reasons:

- `user_stop`
- `disconnect`
- `timeout`
- `server_error`

## Audio Binary Envelope

Audio uses WebSocket binary message:

```text
[1 byte frame_type][16 byte session_uuid][8 byte sequence_be][opus_payload]
```

Header size: 25 bytes.

Fields:

- `frame_type`: unsigned byte.
- `session_uuid`: UUID talk session as 16 raw bytes.
- `sequence_be`: uint64 big-endian.
- `opus_payload`: Opus encoded audio frame.

Frame types:

- `0x01` audio uplink from speaker to server.
- `0x02` audio downlink from server to listener.
- `0x05` optional binary heartbeat.

Audio settings:

- Codec: Opus.
- Channels: mono.
- Frame duration: 20 ms.
- Sample rate: prefer 16000 Hz for MVP voice; 48000 Hz acceptable if Android
  Opus library requires it.

Server rules:

- Only current speaker may send `0x01` frames for a session.
- Server rewrites frame type from `0x01` to `0x02` before forwarding.
- Server forwards only to group listeners and dispatcher listeners that are
  allowed to monitor the group.
- Server does not decode Opus in MVP.
- Invalid session frame is dropped and logged.
- Sequence yang tidak berurutan dicatat sebagai debug log.
- Talk lock MVP disimpan in-memory sehingga deployment saat ini harus memakai
  satu instance API server.
- Disconnect koneksi speaker otomatis menghentikan session dengan reason
  `disconnect`.

## Error Event

```json
{
  "type": "error",
  "requestId": "optional-request-id",
  "timestamp": "2026-06-08T12:00:00Z",
  "payload": {
    "code": "validation_error",
    "message": "Human readable message",
    "details": {}
  }
}
```

Common codes:

- `validation_error`
- `unauthorized`
- `forbidden`
- `not_found`
- `group_not_joined`
- `ptt_busy`
- `server_error`

## Connection Lifecycle

- `connection.ready` selalu menjadi event pertama untuk koneksi valid.
- Presence dihitung per user, bukan per socket.
- `presence.updated` dengan status `online` dikirim saat koneksi pertama user
  aktif.
- `presence.updated` dengan status `offline` dikirim setelah koneksi terakhir
  user tertutup atau timeout.
- Presence hanya dikirim ke koneksi dengan role `super_admin`, `dispatcher`,
  atau `supervisor`.
