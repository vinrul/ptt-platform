# Architecture

Dokumen ini menjelaskan arsitektur teknis MVP PTT Fleet Platform.

## High Level Diagram

```text
+---------------------+         +--------------------------+
| Android Kotlin App  |         | Web Dispatcher           |
| - Login             |         | Bun + React + MapTalks   |
| - PTT Button        |         | Operator Console         |
| - AudioRecord       |         | User + SOS Monitoring    |
| - AudioTrack        |         +------------+-------------+
| - GPS Tracking      |                      |
| - SOS               |                      |
+----------+----------+                      |
           | REST + WebSocket                | REST + WebSocket
           v                                 v
+----------------------------------------------------------+
| Go API + Realtime Server                                 |
| Gin HTTP Router                                          |
| gorilla/websocket Gateway                                |
|                                                          |
| - Auth                                                   |
| - User/Group/Device API                                  |
| - Presence Manager                                       |
| - GPS Handler                                            |
| - SOS Handler                                            |
| - PTT Talk Lock                                          |
| - Opus Audio Relay                                       |
+--------------------+--------------------+----------------+
                     |                    |
                     v                    v
             +---------------+     +---------------+
             | PostgreSQL    |     | Redis         |
             | Persistent DB |     | Presence/Lock |
             +---------------+     +---------------+
```

## Backend Responsibilities

Backend memakai Go + Gin untuk REST API dan endpoint WebSocket.

Tanggung jawab backend:

- Authenticate user dengan JWT.
- Manage user, group, device, dan membership.
- Maintain WebSocket connection registry.
- Broadcast presence, GPS, SOS, dan PTT state.
- Enforce talk lock per group.
- Relay Opus audio frame tanpa decode.
- Persist GPS logs, SOS events, talk sessions, refresh tokens, dan audit logs.

## WebSocket Connection Model

Setiap koneksi WebSocket memiliki metadata:

- connection id.
- user id.
- role.
- device id optional.
- joined group ids.
- last heartbeat timestamp.
- client type: `android` atau `dispatcher`.

Client connect ke:

```text
GET /ws?token=<jwt>
```

Setelah valid, server mengirim `connection.ready`.

## Data Flow PTT

1. Android speaker menekan tombol PTT.
2. App mengirim event JSON `ptt.start`.
3. Server cek membership dan talk lock grup.
4. Jika kosong, server membuat talk session dan mengirim `ptt.granted`.
5. Server broadcast `ptt.started` ke listener dan dispatcher.
6. Android speaker mulai capture audio dengan AudioRecord.
7. PCM di-encode ke Opus 20 ms.
8. Frame Opus dikirim via WebSocket binary.
9. Server validasi session dan relay frame ke listener dalam grup yang sama.
10. Listener decode Opus dan play via AudioTrack.
11. Saat tombol dilepas, app mengirim `ptt.stop`.
12. Server release lock, update talk session, broadcast `ptt.stopped`.

## Data Flow GPS

1. Android mengambil lokasi periodik.
2. Android mengirim `gps.update`.
3. Server validasi lat/lng.
4. Server menyimpan ke `gps_logs`.
5. Server broadcast `gps.updated` ke dispatcher.
6. Dispatcher update marker MapTalks tanpa reload map.

## Data Flow SOS

1. User tekan tombol SOS.
2. Android kirim `sos.create` dengan lokasi terakhir.
3. Server simpan event ke `sos_events`.
4. Server menulis audit log.
5. Server broadcast `sos.created` ke dispatcher.
6. Dispatcher tampilkan alarm dan zoom ke lokasi.
7. Dispatcher mengirim `sos.ack`.
8. Server update status dan broadcast `sos.acked`.

## Scaling Notes

MVP boleh berjalan single backend instance.

Jika perlu horizontal scaling:

- Presence pindah penuh ke Redis.
- Talk lock pindah ke Redis lock dengan TTL.
- Broadcast antar instance memakai Redis pub/sub.
- Audio relay tetap sebaiknya sticky per WebSocket connection atau pindah ke
  arsitektur media khusus.

## Non-MVP

Fitur berikut tidak masuk MVP:

- WebRTC/SFU.
- Audio recording penuh.
- End-to-end encryption.
- Multi-tenant billing.
- Kubernetes.
- Native DMR gateway.
