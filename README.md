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

Server memerlukan PostgreSQL yang aktif dan seluruh migration sudah diterapkan.
Jalankan terlebih dahulu langkah pada bagian "Menjalankan PostgreSQL dan Redis"
dan "Menjalankan database migration" di atas.

Masuk ke folder server, muat konfigurasi Docker, lalu buat URL koneksi
PostgreSQL:

```bash
cd server

set -a
source ../docker/.env
set +a

export MINI_CHAT_POSTGRES_URL="postgres://mini_chat:${MINI_CHAT_POSTGRES_PASSWORD}@127.0.0.1:25432/mini_chat?sslmode=disable"
```

Jalankan server:

```bash
go run .
```

Sebelum mulai menerima request, server membuat connection pool dan melakukan
`Ping` ke PostgreSQL. Server tidak akan mulai listening jika koneksi database
gagal.

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

## Register identity

Endpoint register:

```text
POST /auth/register
Content-Type: application/json
```

Request hanya menerima tiga field berikut:

| Field | Aturan |
|---|---|
| `username` | Wajib, 3–40 karakter, hanya `a-z`, `0-9`, dan underscore (`_`) |
| `password` | Wajib, 8–128 karakter |
| `display_name` | Wajib, maksimal 100 karakter |

Register tidak menggunakan atau menerima field `email`. Username dirapikan
dengan menghapus spasi di awal dan akhir serta mengubahnya menjadi huruf kecil.

Alur register di server:

```text
HTTP request
  → domain validation dan normalization
  → Argon2id password hashing
  → PostgreSQL identity store
  → HTTP response
```

Password asli hanya digunakan selama proses register. Database menyimpan hash
Argon2id, sedangkan response tidak pernah mengembalikan password maupun
`password_hash`.

Dengan server dan PostgreSQL masih berjalan, buat identity baru:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "alice_step5",
    "password": "practice-password",
    "display_name": "Alice Example"
  }'
```

Request pertama menghasilkan `HTTP/1.1 201 Created`. Response berisi `id`,
`username`, `display_name`, `created_at`, dan `updated_at`, tanpa password atau
hash.

Mengirim ulang request dengan username yang sama menghasilkan:

```text
HTTP/1.1 409 Conflict
```

Request dengan data yang tidak valid atau field yang tidak dikenal, termasuk
`email`, menghasilkan:

```text
HTTP/1.1 400 Bad Request
```

Contoh pemeriksaan data langsung dari folder `docker`:

```bash
docker compose exec postgres \
  psql -U mini_chat -d mini_chat \
  -c "
    SELECT
      id,
      username,
      display_name,
      LEFT(password_hash, 40) AS password_hash_prefix,
      created_at
    FROM identity
    ORDER BY created_at;
  "
```

Nilai `password_hash_prefix` dimulai dengan:

```text
$argon2id$v=19$m=19456,t=2,p=1$
```

Password asli tidak boleh terlihat pada tabel maupun response HTTP.

## Development rules

- Satu commit untuk satu milestone yang selesai.
- Jalankan test yang relevan sebelum commit.
- Gunakan UTC untuk waktu yang disimpan server.
- Jangan commit secret atau file `.env`.
- Jangan mencatat password, access token, atau credential ke log.
- Prioritaskan vertical slice yang dapat dijalankan end-to-end.
- `practice` dan repository BBGE hanya menjadi referensi dan tidak diubah.

Lihat [roadmap](docs/ROADMAP.md) untuk urutan implementasi.
