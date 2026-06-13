# Android Kotlin App

Native Android client untuk PTT Fleet Platform.

Fitur sampai Phase 9:

- Kotlin native, min SDK 26.
- Login REST dengan input server URL, username, dan password.
- Access/refresh token dienkripsi memakai AES-GCM dengan key di Android Keystore.
- Access token diperbarui otomatis sekitar 30 detik sebelum kedaluwarsa.
- Refresh token dirotasi dan pasangan token baru langsung disimpan terenkripsi.
- WebSocket OkHttp dengan JWT query token.
- `connection.ready`, heartbeat 25 detik, dan reconnect exponential backoff.
- Timeout handshake WebSocket 15 detik; socket yang belum ready akan dicoba ulang.
- Playback memakai jitter buffer 3 frame (sekitar 60 ms) dan queue terbatas.
- Status koneksi dan logout.
- GPS realtime melalui Fused Location Provider dengan interval adaptif.
- Permission lokasi baru diminta ketika user menekan `Start GPS tracking`.
- Update lokasi dikirim sebagai event `gps.update` selama WebSocket terhubung.
- Tab Map memakai MapLibre Native dengan tile OpenStreetMap. Snapshot posisi
  terakhir anggota grup dimuat dari REST API dan marker digeser langsung saat
  menerima `gps.updated`. Detail marker menyediakan aksi private PTT.
- Jika switch GPS mati, awal transmisi PTT mencoba mengirim satu snapshot lokasi
  tanpa mengaktifkan tracking periodik atau meminta permission baru.
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

Default server aplikasi memakai:

```text
https://ptt.vinrul.my.id

Setelah login, aplikasi menjalankan foreground service `PTT Fleet patrol active`.
Service ini mempertahankan WebSocket, join channel terakhir, dan playback suara
ketika layar mati, aplikasi diminimalkan, atau Activity ditutup. Gunakan aksi
`Stop` pada notifikasi atau tombol logout untuk menghentikannya.

Pilihan switch GPS disimpan lokal dengan `SharedPreferences`. Restart aplikasi
atau FCM wakeup tidak mengaktifkan tracking periodik jika sebelumnya dimatikan.
FCM wakeup tetap dapat meminta satu lokasi terbaru tanpa mengubah pilihan switch.

Tracking periodik dimulai dalam mode bergerak dengan high accuracy dan interval
target 10 detik. Setelah enam sampel berturut-turut berada di bawah 0,5 m/s,
tracking berpindah ke balanced power dengan interval target 60 detik. Kecepatan
minimal 1 m/s mengembalikan mode bergerak pada sampel berikutnya. Hysteresis ini
mencegah noise GPS membuat mode sering berpindah saat perangkat diam.

Untuk jaringan seluler buruk, WebSocket melakukan reconnect exponential mulai
2 detik. Lima percobaan awal tetap cepat hingga maksimum 30 detik, lalu koneksi
yang gagal lama diperlambat ke 60 detik, 120 detik, dan maksimum 5 menit agar
mode idle tidak terus membangunkan radio jaringan. FCM tetap menjadi jalur
wake-up utama saat server perlu membangunkan perangkat untuk PTT atau permintaan
lokasi. Status baru dianggap connected setelah server mengirim
`connection.ready`. REST memakai connect timeout 10 detik, write timeout 15
detik, read timeout 20 detik, dan call timeout total 30 detik. OkHttp hanya
melakukan retry koneksi transparan; operasi perubahan data seperti ganti password
tidak diulang manual agar tidak terkirim dua kali.

Talk lock berlaku per grup. Jika Android meminta PTT ketika grup sedang dipakai,
request masuk antrean FIFO dan aplikasi menampilkan nomor antrean. Mikrofon baru
aktif setelah `ptt.granted`. Melepas tombol sebelum mendapat grant mengirim
`ptt.cancel`, sehingga user tidak tiba-tiba mulai bicara setelah niat bicara
sudah dibatalkan.

Saat server memberi `ptt.granted`, aplikasi mencoba mengambil satu lokasi jika
switch GPS sedang mati. Pengambilan berlangsung asynchronous sehingga audio
langsung berjalan tanpa menunggu GPS. Jika izin lokasi belum diberikan, layanan
lokasi perangkat mati, atau lokasi tidak tersedia, snapshot dilewati secara
silent dan user tetap dapat berbicara. Jika switch GPS aktif, tracking periodik
sudah menangani lokasi sehingga snapshot PTT tidak dikirim.

Setelah login, navigasi utama dibagi menjadi tiga tab:

- `Home`: pilihan grup, GPS, PTT broadcast ke seluruh grup, dan SOS.
- `Talk Target`: daftar anggota grup beserta status online/offline dan PTT privat.
- `Profile`: identitas user, ganti password, dan logout.

Pada beberapa perangkat, nonaktifkan battery optimization untuk PTT Fleet agar
vendor Android tidak membunuh foreground service ketika layar mati. Menekan
`Force stop` dari Settings tetap menghentikan seluruh service sampai aplikasi
dibuka kembali.

FCM `ptt_wakeup` akan menghidupkan service, menyambungkan ulang WebSocket,
bergabung ke `groupId` dari payload, meminta satu lokasi terbaru, lalu mengirim
`gps.update`. Layar dibangunkan sekitar 8 detik. Android dapat menolak membuka
Activity otomatis dari background, tetapi wake lock dan notifikasi tetap aktif.
Wake-up broadcast membuka tab `Home`. Wake-up direct PTT membuka tab
`Talk Target` dan otomatis memilih user yang mengirim private PTT berdasarkan
`speakerUserId`, dengan username sebagai fallback. Navigasi wake-up disimpan
sementara selama maksimal lima menit agar tetap diterapkan ketika Activity baru
berhasil dibuka setelah service hidup.
Untuk mengurangi baterai, service tidak lagi menahan partial wake lock atau
high-performance Wi-Fi lock permanen. Partial wake lock hanya diambil singkat
saat FCM wake-up, reconnect, permintaan lokasi, atau audio PTT masuk. Durasi
wake lock singkat ini memberi waktu hingga 60 detik agar token refresh,
WebSocket ready, join grup, dan snapshot GPS bisa selesai tanpa kembali ke lock
permanen. WebSocket tetap hidup selama foreground patrol aktif; jika koneksi
mati lama, reconnect diperlambat dan FCM dipakai sebagai pemicu wake-up
berikutnya.
FCM `gps_location_request` memakai jalur wake service yang sama dengan
`ptt_wakeup`, tetapi membawa `requestId` agar dispatcher bisa menunggu jawaban
posisi dan menonaktifkan overlay sehingga aplikasi tidak dibuka di atas layar.
Sebelum reconnect, service memakai access token yang masih valid atau menukar
refresh token terlebih dahulu. Jika handshake WebSocket ditolak dengan
`401/403`, service melakukan satu refresh terkoordinasi lalu reconnect dengan
access token baru. Jika refresh token sudah kedaluwarsa atau dicabut, sesi lokal
dihapus dan user harus login kembali.

Untuk direct PTT, target tidak harus sedang online. Server mengirim
`ptt_wakeup` hanya ke perangkat user yang dipilih; audio mulai diterima setelah
service tersambung dan berhasil join grup.
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
bun run android:build:playstore
```

Build debug menjalankan unit test dan menghasilkan APK debug. Build release
menjalankan lint dan menghasilkan APK yang ditandatangani release. Build
Play Store menghasilkan Android App Bundle (`.aab`) untuk upload ke Google Play
Console:

```text
apps/android-kotlin/app/build/outputs/apk/debug/app-debug.apk
apps/android-kotlin/app/build/outputs/apk/release/app-release.apk
apps/android-kotlin/app/build/outputs/bundle/release/app-release.aab
```

Install release lokal ke emulator atau perangkat yang terhubung:

```bash
bun run android:install:release
bun run android:run:release
```

`android:run:release` menyalakan emulator jika belum ada device aktif, lalu
memasang dan membuka aplikasi. `android:install:release` digunakan ketika
emulator atau perangkat fisik sudah terhubung ke ADB.

Konfigurasi signing dibaca dari `local.properties` yang tidak di-commit, atau
dari environment variable berikut untuk CI:

```text
ANDROID_RELEASE_STORE_FILE
ANDROID_RELEASE_STORE_PASSWORD
ANDROID_RELEASE_KEY_ALIAS
ANDROID_RELEASE_KEY_PASSWORD
```

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
