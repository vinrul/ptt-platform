# PTT Fleet Platform Version 2 Plan

Version 2 meningkatkan platform dari instalasi satu organisasi menjadi layanan
multi-tenant yang dapat diskalakan. Audio Opus over WebSocket tetap menjadi
transport awal sampai hasil pengukuran menunjukkan WebRTC/SFU memang diperlukan.

## Sasaran

- Satu deployment melayani banyak organisasi dengan isolasi data ketat.
- Setiap tenant mengelola user, grup, perangkat, GPS, SOS, dan audit sendiri.
- Backend dapat dijalankan lebih dari satu instance.
- PTT tetap berfungsi saat instance API bertambah.
- Keputusan WebRTC/SFU dibuat berdasarkan metrik, bukan asumsi.

## Prinsip Migrasi

- Semua data V1 dipindahkan ke satu tenant default.
- `tenant_id` wajib tersedia dalam JWT dan seluruh resource tenant.
- Query tenant tidak boleh hanya mengandalkan filter dari client.
- Unique constraint yang sebelumnya global menjadi tenant-scoped jika sesuai.
- Migrasi schema dilakukan bertahap: nullable, backfill, constraint, lalu
  enforcement aplikasi.
- Billing belum masuk scope awal V2.

## Phase V2.0 - Baseline V1

Task:

- Tandai release V1 dan simpan hasil smoke test.
- Ukur koneksi WebSocket aktif, frame audio/detik, bandwidth keluar, packet gap,
  reconnect, latency grant PTT, dan penggunaan CPU/memory.
- Pastikan backup PostgreSQL dapat direstore.
- Tambahkan correlation ID dan structured logging tenant-ready.

Acceptance criteria:

- Dashboard atau log dapat menunjukkan metrik per koneksi dan grup.
- Restore backup diuji pada environment terpisah.
- Ada angka baseline untuk 10, 50, dan 100 koneksi simulasi.

## Phase V2.1 - Tenant Foundation

Schema awal:

```text
tenants
  id
  slug
  name
  status
  settings
  created_at
  updated_at

tenant_memberships
  tenant_id
  user_id
  role
  status
  created_at
```

Keputusan role:

- `platform_admin`: mengelola seluruh platform, bukan anggota operasional tenant.
- `tenant_admin`: mengelola tenant sendiri.
- `dispatcher`, `supervisor`, dan `field_user`: role operasional tenant.

Task:

- Tambahkan tenant default untuk data V1.
- Tambahkan `tenant_id` pada groups, devices, GPS, SOS, talk sessions, refresh
  tokens, dan audit logs.
- Putuskan apakah satu identitas dapat menjadi anggota beberapa tenant. Default
  V2: boleh, melalui `tenant_memberships`.
- Pertahankan `users.username` unik global pada migrasi awal agar login V1 tetap
  kompatibel. Alias/login name per tenant menjadi fitur lanjutan jika dibutuhkan.
- Pindahkan role operasional dari `users.role` ke `tenant_memberships.role`.
  Platform role disimpan terpisah dan tidak memberi akses otomatis ke data tenant.
- Tambahkan foreign key dan index tenant pada seluruh query utama.

Acceptance criteria:

- Semua data V1 berhasil dibackfill ke tenant default.
- Tidak ada resource operasional tanpa tenant.
- Query lint/test membuktikan tenant A tidak dapat membaca ID milik tenant B.
- Migration up dan rollback development terdokumentasi.

## Phase V2.2 - Tenant-Aware Auth dan API

Task:

- Access token memuat `tenantId`, membership ID, dan tenant role aktif.
- Login memilih tenant jika user memiliki lebih dari satu membership.
- Tambahkan endpoint tenant list/switch.
- Semua repository menerima tenant context secara eksplisit.
- Admin tenant hanya dapat mengelola resource tenant sendiri.
- Platform admin memakai endpoint dan UI terpisah.
- Rate limit, audit log, dan device session menyertakan tenant.

Acceptance criteria:

- Pergantian tenant menerbitkan token baru.
- ID resource tenant lain menghasilkan `404` atau `403` tanpa membocorkan data.
- Super admin V1 dimigrasikan menjadi tenant admin pada tenant default.
- Test authorization mencakup seluruh endpoint protected.

## Phase V2.3 - Tenant-Aware Realtime

Task:

- Connection WebSocket menyimpan tenant ID dari token.
- Group join memvalidasi tenant dan membership.
- Presence, GPS, SOS, PTT, FCM wakeup, dan binary audio dibatasi tenant.
- Redis key memakai namespace tenant:

```text
tenant:{tenantId}:presence:{userId}
tenant:{tenantId}:ptt-lock:{groupId}
tenant:{tenantId}:ptt-queue:{groupId}
```

- Gunakan Redis pub/sub atau streams untuk event antar instance.
- Gunakan sticky routing untuk koneksi audio selama relay WebSocket masih aktif.

Acceptance criteria:

- Event tenant A tidak pernah diterima koneksi tenant B.
- Talk lock dan queue konsisten pada minimal dua instance API.
- Disconnect satu instance melepas lock melalui TTL/fencing yang aman.
- FCM direct PTT hanya menargetkan device tenant aktif.

## Phase V2.4 - Tenant Administration

Task:

- Platform console untuk membuat, menonaktifkan, dan melihat health tenant.
- Tenant console untuk branding ringan, retention, user limit, dan kebijakan GPS.
- Android menyimpan tenant aktif bersama session.
- Dispatcher menampilkan tenant aktif dengan jelas.
- Export audit dan laporan selalu tenant-scoped.

Acceptance criteria:

- Platform admin dapat membuat tenant baru tanpa akses database.
- Tenant admin dapat mengelola user dan grupnya sendiri.
- User multi-tenant dapat memilih tenant tanpa login password ulang selama
  refresh session masih valid.
- Menonaktifkan tenant menolak login baru dan koneksi realtime baru.

## Phase V2.5 - Scale Test dan Media Decision

Sebelum memilih SFU, jalankan uji beban dengan pola PTT nyata: satu speaker,
banyak listener, frame Opus 20 ms, koneksi seluler tidak stabil, dan reconnect.

WebRTC/SFU diprioritaskan jika salah satu kondisi ini konsisten:

- Bandwidth egress relay WebSocket menjadi bottleneck utama.
- Satu grup membutuhkan lebih dari sekitar 100 listener aktif.
- Deployment membutuhkan banyak region atau lebih dari dua instance media.
- Packet loss/jitter seluler tidak cukup tertangani oleh buffer aplikasi.
- Produk membutuhkan full-duplex conference, echo cancellation, simulcast, atau
  interoperabilitas browser yang lebih matang.

Jika kondisi tersebut belum terjadi, pertahankan Opus WebSocket relay karena
lebih sederhana untuk half-duplex PTT.

## Phase V2.6 - WebRTC/SFU Optional

Keputusan stack jika hasil Phase V2.5 menunjukkan SFU diperlukan:

- Custom audio-only SFU menggunakan `github.com/pion/webrtc/v4`.
- `coturn` sebagai STUN/TURN server production.
- `github.com/pion/turn/v4` hanya dipertimbangkan jika dibutuhkan TURN embedded
  yang dikontrol langsung dari service Go.
- Signaling, tenant authorization, membership, talk lock, dan queue tetap
  dikelola API Go serta WebSocket yang sudah ada.
- Audio Opus diteruskan sebagai RTP tanpa decode atau transcode.

`ion-sfu` tidak dipilih sebagai fondasi baru. Kebutuhan PTT hanya audio
half-duplex dan lebih kecil daripada conference audio/video umum, sehingga SFU
khusus memberi kontrol lebih baik dengan komponen yang lebih sedikit.

Arsitektur target:

```text
Android/Web
  | REST + control WebSocket
  v
Go API / Tenant Authorization / PTT Lock
  | signed media grant
  v
Custom Pion Audio SFU
  | WebRTC audio
  v
Authorized listeners

STUN/TURN path:
Android/Web <-> coturn <-> Custom Pion Audio SFU
```

Task:

- Tambahkan service baru `services/media-server`.
- Gunakan `pion/webrtc/v4` untuk PeerConnection, ICE, DTLS, SRTP, RTP/RTCP,
  NACK, dan statistik media.
- Pertahankan `ptt.start`, `ptt.granted`, queue, dan authorization di Go.
- Tambahkan signed short-lived media token per tenant/group/session.
- SFU hanya menerima publish dari pemegang talk lock.
- Satu room media memakai namespace `tenantId:groupId`.
- Satu room hanya memiliki satu publisher aktif, tetapi dapat memiliki banyak
  subscriber.
- Deploy `coturn` dengan UDP dan TLS untuk jaringan seluler/NAT sulit.
- Pantau alokasi TURN karena trafik relay menambah bandwidth server.
- Jalankan canary per tenant atau grup; jangan migrasikan semua tenant sekaligus.

Acceptance criteria:

- User tanpa media grant tidak dapat publish atau subscribe.
- Talk lock tetap authoritative di backend.
- Media tenant A tidak dapat masuk ke room tenant B.
- SFU tidak melakukan transcoding Opus untuk alur PTT normal.
- P95 time-to-first-audio memenuhi target hasil baseline.
- Fallback WebSocket tersedia selama masa transisi.
- Kegagalan SFU tidak membocorkan audio lintas tenant.

## Definition of Done V2

- Tenant isolation diuji pada REST, WebSocket, FCM, database, cache, dan audit.
- Dua instance API dapat melayani tenant yang sama.
- Backup/restore mendukung tenant default dan tenant baru.
- Monitoring dapat difilter per tenant tanpa menyimpan password/token.
- Runbook incident tenant leakage tersedia.
- WebRTC/SFU hanya dinyatakan selesai jika hasil load test dan canary memenuhi
  acceptance criteria.

## Risiko Teknis

### Tenant Data Leakage

- Wajib tenant context dari JWT.
- Repository tidak menerima tenant ID dari payload client.
- Test negatif lintas tenant untuk setiap resource.
- Pertimbangkan PostgreSQL Row Level Security sebagai lapisan tambahan setelah
  query tenant-aware stabil, bukan pengganti authorization aplikasi.

### Migration Blast Radius

- Gunakan migration expand/backfill/contract.
- Ambil backup sebelum constraint final.
- Jangan menambah `NOT NULL` sebelum seluruh data berhasil dibackfill.

### Redis Lock Split-Brain

- Gunakan TTL, ownership token, dan compare-and-delete.
- Session database tetap menjadi audit source, bukan lock realtime.

### SFU Operational Complexity

- TURN menambah bandwidth dan biaya.
- Observability media harus mencakup RTT, jitter, packet loss, bitrate, dan ICE.
- Mulai dari proof of concept terisolasi dan canary tenant.

### Cost Per Tenant

- Ukur storage GPS, egress audio, FCM, koneksi aktif, dan retention.
- Terapkan quota sebelum billing otomatis.
