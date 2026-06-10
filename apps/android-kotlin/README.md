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
- Tombol SOS dengan dialog konfirmasi mengirim `sos.create`.
- SOS menyertakan koordinat terakhir jika tersedia dan tetap dapat dikirim tanpa
  GPS fix.
- Daftar channel hanya berisi grup yang ditugaskan kepada field user.
- PTT hold-to-talk dengan talk lock server.
- Capture `AudioRecord`, Opus mono 16 kHz/20 ms, dan playback `AudioTrack`.
- Playback PTT memakai loudspeaker/media volume dengan gain suara terbatas.
- Microphone baru aktif setelah server mengirim `ptt.granted`.
- Permission foreground service, notification, dan wake lock tetap belum
  digunakan sebelum patrol background phase.

## Server URL Local

Android emulator memakai:

```text
http://100.82.105.93:8080

Setelah login, aplikasi menjalankan foreground service `PTT Fleet patrol active`.
Service ini mempertahankan WebSocket, join channel terakhir, dan playback suara
ketika layar mati, aplikasi diminimalkan, atau Activity ditutup. Gunakan aksi
`Stop` pada notifikasi atau tombol logout untuk menghentikannya.

Pada beberapa perangkat, nonaktifkan battery optimization untuk PTT Fleet agar
vendor Android tidak membunuh foreground service ketika layar mati. Menekan
`Force stop` dari Settings tetap menghentikan seluruh service sampai aplikasi
dibuka kembali.

FCM `ptt_wakeup` akan menghidupkan service, menyambungkan ulang WebSocket,
bergabung ke `groupId` dari payload, meminta satu lokasi terbaru, lalu mengirim
`gps.update`. Layar dibangunkan sekitar 8 detik. Android dapat menolak membuka
Activity otomatis dari background, tetapi wake lock dan notifikasi tetap aktif.
```

Perangkat fisik harus memakai IP LAN komputer development, misalnya:

```text
http://192.168.1.10:8080
```

HTTP cleartext hanya diizinkan pada build debug. Build release mengharuskan
HTTPS/WSS.

## Build

Project membutuhkan JDK 17 dan Android SDK 35.

Build dari root repository di Windows atau macOS:

```bash
bun run android:build:debug
bun run android:build:release
```

Build debug menjalankan unit test dan menghasilkan APK debug. Build release
menjalankan lint, membuat APK unsigned, lalu menandatanganinya memakai Android
debug keystore lokal agar dapat dipasang untuk pengujian:

```text
apps/android-kotlin/app/build/outputs/apk/debug/app-debug.apk
apps/android-kotlin/app/build/outputs/apk/release/app-release-unsigned.apk
apps/android-kotlin/app/build/outputs/apk/release/app-release-local.apk
```

Install release lokal ke emulator atau perangkat yang terhubung:

```bash
bun run android:install:release
bun run android:run:release
```

`android:run:release` menyalakan emulator jika belum ada device aktif, lalu
memasang dan membuka aplikasi. `android:install:release` digunakan ketika
emulator atau perangkat fisik sudah terhubung ke ADB.

`app-release-local.apk` memakai debug keystore lokal dan bukan untuk distribusi
production. Release production harus ditandatangani dengan keystore production.
Keystore dan password production tidak boleh disimpan di repository.

Perintah Gradle manual untuk macOS:

```bash
export JAVA_HOME=/path/to/jdk-17
export ANDROID_HOME=$HOME/Library/Android/sdk
./gradlew testDebugUnitTest
./gradlew assembleDebug
./gradlew lintDebug
```

Windows PowerShell:

```powershell
$env:JAVA_HOME = "C:\Program Files\Java\jdk-17"
$env:ANDROID_HOME = "$env:LOCALAPPDATA\Android\Sdk"
Set-Location apps/android-kotlin
.\gradlew.bat testDebugUnitTest
.\gradlew.bat assembleDebug
.\gradlew.bat lintDebug
```

`local.properties` bersifat lokal dan diabaikan Git. Jika Android SDK tidak
terdeteksi otomatis, buat file tersebut tanpa mengubah konfigurasi macOS:

```properties
sdk.dir=C\:\\Users\\YOUR_USER\\AppData\\Local\\Android\\Sdk
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

Windows PowerShell memakai sintaks environment variable berikut:

```powershell
$env:ANDROID_AVD = "Pixel_6a"
bun run android:run
```

Script memilih AVD pertama yang tersedia jika AVD default macOS tidak ditemukan.
Konfigurasi AVD tersimpan lokal di masing-masing komputer dan tidak di-commit,
sehingga AVD Windows dan macOS dapat memakai nama berbeda.

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
10. Tekan `Send SOS`, konfirmasi, dan pastikan status pengiriman muncul.
11. Pastikan dispatcher menerima alarm serta lokasi terbaru jika tersedia.
12. Login dua perangkat fisik dengan user berbeda dalam grup yang sama.
13. Pilih channel yang sama lalu tahan PTT pada perangkat A.
14. Pastikan perangkat B mendengar suara dan menerima status busy jika mencoba
    bicara bersamaan.
15. Lepaskan PTT A dan pastikan B dapat mengambil channel.

Tracking Phase 9 berjalan saat activity aktif. Foreground patrol service untuk
tracking background disiapkan pada phase Android patrol/PTT berikutnya.

Android Emulator tidak cocok untuk acceptance test microphone end-to-end.
Gunakan minimal dua perangkat fisik untuk pengujian suara.
