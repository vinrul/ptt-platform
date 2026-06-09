# Dispatcher Web

Web dispatcher berbasis Bun, React, TypeScript, Vite, MapTalks, Zustand, dan
Tailwind CSS.

Fitur sampai Phase 9:

- Login melalui `POST /api/auth/login`.
- Session operator tersimpan di local storage.
- Fetch user dan group dari REST API.
- WebSocket reconnect dengan exponential backoff.
- Heartbeat setiap 25 detik.
- Presence user realtime.
- Group selector dan `group.join`.
- MapTalks lifecycle terpisah agar map tidak dibuat ulang saat state berubah.
- Marker unit dibuat dan digeser langsung saat menerima `gps.updated`.
- Peta otomatis fokus ke marker pertama dan menampilkan jumlah unit terlacak.
- Alarm SOS visual dan bunyi singkat saat emergency aktif.
- Marker SOS merah, auto-zoom ke lokasi, dan acknowledge dari dispatcher.

## Run Local

Jalankan backend di port `8080`, lalu:

```bash
bun --filter dispatcher-web dev
```

Buka `http://localhost:5173`. Vite meneruskan `/api` dan `/ws` ke backend local,
sehingga development tidak memerlukan konfigurasi CORS tambahan.

Optional production/build-time environment:

```env
VITE_API_URL=https://api.ptt.example.com
VITE_WS_URL=wss://api.ptt.example.com/ws
```

## Validation

```bash
bun --filter dispatcher-web test
bun --filter dispatcher-web lint
bun --filter dispatcher-web build
```

Browser dapat membatasi bunyi alarm sebelum ada interaksi user. Login dispatcher
menjadi interaksi awal yang biasanya mengizinkan alarm berikutnya.
