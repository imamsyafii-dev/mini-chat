# Mini Chat

Mini Chat adalah project pembelajaran chat realtime yang dibangun dari nol.
Project ini mengambil konsep umum dari arsitektur BBGE, tetapi memiliki scope,
struktur, dan implementasi sendiri.

## Target

Dalam tiga hari, project ini ditargetkan memiliki:

- server Go;
- PostgreSQL sebagai penyimpanan permanen;
- Redis Pub/Sub untuk distribusi event realtime;
- WebSocket untuk pengiriman event ke client;
- desktop macOS menggunakan React, Tauri, dan Rust;
- mobile Android menggunakan React Native bare;
- chat realtime antara desktop dan Android emulator.

## Fitur utama

Identity:

- register dengan username dan password;
- Argon2id password hashing;
- login;
- JWT access token;
- current-user profile.

Chat:

- direct conversation;
- mengirim pesan;
- mengambil riwayat pesan;
- menerima pesan realtime;
- reconnect dan mengambil pesan yang terlewat.

## Arsitektur

```text
Desktop Tauri/Rust ─┐
                    ├── HTTP + WebSocket ── Go server
Android React Native┘                         ├── PostgreSQL
                                             └── Redis Pub/Sub
```

PostgreSQL adalah sumber data permanen. Redis Pub/Sub hanya mengirim event
realtime. Jika event realtime terlewat, client mengambil kembali pesan dari
PostgreSQL.

## Struktur target

```text
mini-chat/
├── server/
├── desktop/
├── mobile/
├── docker/
└── docs/
```

Folder dibuat bertahap ketika mulai digunakan.

## Local resources

| Resource | Value |
|---|---|
| HTTP dan WebSocket server | `localhost:28080` |
| PostgreSQL host port | `25432` |
| Redis host port | `26380` |
| Database | `mini_chat` |
| Environment prefix | `MINI_CHAT_` |
| Docker Compose project | `mini-chat` |
| Android application ID | `dev.imamsyafii.minichat` |
| Tauri identifier | `dev.imamsyafii.minichat` |

Android emulator mengakses server Mac melalui `10.0.2.2:28080`.

## Menjalankan PostgreSQL dan Redis

Pertama kali, masuk ke folder Docker dan buat konfigurasi lokal:

```bash
cd docker
cp .env.example .env
```

Ubah nilai `MINI_CHAT_POSTGRES_PASSWORD` di `.env`. File `.env` hanya digunakan
secara lokal dan tidak boleh masuk Git.

Validasi dan jalankan service:

```bash
docker compose config --quiet
docker compose up -d
docker compose ps
```

Periksa PostgreSQL:

```bash
docker compose exec postgres \
  psql -U mini_chat -d mini_chat -c "SELECT 1;"
```

Periksa Redis:

```bash
docker compose exec redis redis-cli ping
```

Aplikasi server yang berjalan langsung di Mac menggunakan:

```text
PostgreSQL: localhost:25432
Redis:      localhost:26380
```

Service yang berjalan di network Docker menggunakan:

```text
PostgreSQL: postgres:5432
Redis:      redis:6379
```

Untuk menghentikan container tanpa menghapus data PostgreSQL:

```bash
docker compose down
```

Untuk menghapus container sekaligus data PostgreSQL lokal:

```bash
docker compose down -v
```

Perintah `down -v` bersifat destruktif dan hanya digunakan ketika ingin
mereset database lokal.

## Menjalankan database migration

File migration berada di `server/db/migrations` dan dijalankan berurutan oleh
Goose.

Pastikan PostgreSQL sudah berjalan, lalu siapkan koneksi dari folder server:

```bash
cd server

set -a
source ../docker/.env
set +a

export MINI_CHAT_POSTGRES_URL="postgres://mini_chat:${MINI_CHAT_POSTGRES_PASSWORD}@localhost:25432/mini_chat?sslmode=disable"
```

Terapkan seluruh migration:

```bash
go tool goose \
  -dir db/migrations \
  postgres "$MINI_CHAT_POSTGRES_URL" \
  up
```

Periksa status migration:

```bash
go tool goose \
  -dir db/migrations \
  postgres "$MINI_CHAT_POSTGRES_URL" \
  status
```

Rollback satu migration terakhir:

```bash
go tool goose \
  -dir db/migrations \
  postgres "$MINI_CHAT_POSTGRES_URL" \
  down
```

Perintah `down` dapat menghapus tabel atau data. Gunakan hanya ketika memahami
isi bagian `-- +goose Down` pada migration yang akan dibatalkan.

Bersihkan variable koneksi setelah selesai:

```bash
unset MINI_CHAT_POSTGRES_URL
unset MINI_CHAT_POSTGRES_PASSWORD
```

Migration yang sudah masuk repository bersifat append-only. Untuk mengubah
schema, tambahkan file migration baru dan jangan mengedit migration lama.

## Menjalankan server

Masuk ke folder server:

```bash
cd server
```

Jalankan server:

```bash
go run .
```

Secara default, server berjalan di:

```text
http://localhost:28080
```

Periksa health endpoint:

```bash
curl -i http://localhost:28080/health
```

Respons yang diharapkan:

```json
{"status":"ok"}
```

Alamat server dan batas waktu graceful shutdown dapat diubah menggunakan environment variable:

```bash
MINI_CHAT_HTTP_ADDR=:28081 \
MINI_CHAT_SHUTDOWN_TIMEOUT=15s \
go run .
```

Untuk menghentikan server secara aman, tekan `Ctrl+C`.

## Development rules

- Satu commit untuk satu milestone yang selesai.
- Jalankan test yang relevan sebelum commit.
- Gunakan UTC untuk waktu yang disimpan server.
- Jangan commit secret atau file `.env`.
- Jangan mencatat password, access token, atau credential ke log.
- Prioritaskan vertical slice yang dapat dijalankan end-to-end.
- `practice` dan repository BBGE hanya menjadi referensi dan tidak diubah.

Lihat [roadmap](docs/ROADMAP.md) untuk urutan implementasi.
