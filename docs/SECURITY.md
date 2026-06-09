# Security

Security baseline untuk production awal.

## Transport

- Production API menolak request non-HTTPS dengan HTTP `426`.
- Caddy meneruskan `X-Forwarded-Proto: https` ke API.
- Endpoint `/healthz` dan `/readyz` tetap dapat diakses dari network internal.
- Web dispatcher production wajib memakai HTTPS dan WebSocket `wss://`.

## Authentication

- Access token memakai JWT dengan secret minimal 32 karakter.
- Refresh token disimpan dalam bentuk hash dan dirotasi saat dipakai.
- Password disimpan menggunakan bcrypt.
- Protected endpoint menolak request tanpa Bearer access token.
- WebSocket memvalidasi JWT dan status aktif user sebelum upgrade.
- Access log hanya mencatat path tanpa query string agar token WebSocket tidak
  masuk ke log.

## Login Rate Limit

Login dibatasi berdasarkan kombinasi IP client dan username. Default:

```env
LOGIN_RATE_LIMIT=10
LOGIN_RATE_WINDOW_SECONDS=60
```

Saat limit terlampaui, API mengembalikan HTTP `429` dan header `Retry-After`.
Counter disimpan di Redis agar konsisten jika API dijalankan lebih dari satu
instance. Jika Redis tidak tersedia, limiter fail-open dan readiness check akan
tetap melaporkan dependency tidak siap.

## Origin Policy

REST browser dan WebSocket browser hanya menerima origin yang terdaftar:

```env
CORS_ALLOWED_ORIGINS=https://ptt.example.com
```

Beberapa origin dipisahkan dengan koma. Origin production wajib memakai HTTPS.
Client Android tidak mengirim header `Origin`, sehingga tetap dapat terhubung.

## Trusted Proxy

`TRUSTED_PROXIES` menentukan proxy yang boleh memasok alamat IP client melalui
forwarded headers. Untuk network Docker private:

```env
TRUSTED_PROXIES=172.16.0.0/12
```

Jangan memakai `0.0.0.0/0`, karena client dapat memalsukan alamat IP dan
melewati rate limit.

## Production Checklist

- Buat `JWT_SECRET` random minimal 32 byte.
- Gunakan password PostgreSQL yang kuat.
- Batasi port PostgreSQL, Redis, dan API agar tidak dipublish langsung.
- Pastikan hanya Caddy yang mengekspos port `80` dan `443`.
- Isi allowlist origin dan trusted proxy secara eksplisit.
- Verifikasi request HTTP API ditolak dan HTTPS berhasil.
- Verifikasi origin asing ditolak untuk REST dan WebSocket.
- Verifikasi login ke-11 dalam satu menit menerima HTTP `429`.
