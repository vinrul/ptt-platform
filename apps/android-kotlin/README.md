# Android Kotlin App

Native Android client untuk PTT Fleet Platform.

Fitur sampai Phase 9:

- Kotlin native, min SDK 26.
- Login REST dengan input server URL, username, dan password.
- Access/refresh token dienkripsi memakai AES-GCM dengan key di Android Keystore.
- WebSocket OkHttp dengan JWT query token.
- `connection.ready`, heartbeat 25 detik, dan reconnect exponential backoff.
- Status koneksi dan logout.
- GPS realtime melalui Fused Location Provider dengan interval target 15 detik.
- Permission lokasi baru diminta ketika user menekan `Start GPS tracking`.
- Update lokasi dikirim sebagai event `gps.update` selama WebSocket terhubung.
- Permission audio, foreground service, notification, dan wake lock tetap belum
  diminta/digunakan sebelum phase terkait.

## Server URL Local

Android emulator memakai:

```text
http://10.0.2.2:8080
```

Perangkat fisik harus memakai IP LAN komputer development, misalnya:

```text
http://192.168.1.10:8080
```

HTTP cleartext hanya diizinkan pada build debug. Build release mengharuskan
HTTPS/WSS.

## Build

Project membutuhkan JDK 17 dan Android SDK 35.

```bash
export JAVA_HOME=/path/to/jdk-17
export ANDROID_HOME=$HOME/Library/Android/sdk
./gradlew testDebugUnitTest
./gradlew assembleDebug
./gradlew lintDebug
```

APK debug:

```text
app/build/outputs/apk/debug/app-debug.apk
```

## Emulator via Bun

AVD default adalah `Medium_Phone_API_36.1`.

```bash
bun run android:emulator
bun run android:install
bun run android:open
```

Atau jalankan emulator, tunggu boot, install APK, dan buka aplikasi sekaligus:

```bash
bun run android:run
```

AVD atau APK dapat dioverride:

```bash
ANDROID_AVD=My_AVD bun run android:run
ANDROID_APK=/path/to/app.apk bun run android:install
```

## Manual Test Notes

1. Jalankan PostgreSQL, Redis, migration, dan backend.
2. Pastikan user aktif tersedia.
3. Install APK debug pada emulator/perangkat.
4. Login dan pastikan status berubah menjadi `Connected`.
5. Matikan backend sementara dan pastikan status menjadi `Reconnecting`.
6. Nyalakan backend dan pastikan koneksi pulih otomatis.
7. Logout dan pastikan session terenkripsi dihapus.
8. Tekan `Start GPS tracking`, izinkan lokasi, lalu set lokasi emulator.
9. Pastikan koordinat terbaru tampil di aplikasi dan diterima dispatcher.

Tracking Phase 9 berjalan saat activity aktif. Foreground patrol service untuk
tracking background disiapkan pada phase Android patrol/PTT berikutnya.
