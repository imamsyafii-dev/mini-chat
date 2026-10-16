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
openssl rand -base64 32
```

Ubah nilai `MINI_CHAT_POSTGRES_PASSWORD` di `.env`. Salin hasil perintah
`openssl` ke `MINI_CHAT_JWT_SECRET` dan simpan menggunakan tanda kutip agar
nilainya dibaca utuh:

```dotenv
MINI_CHAT_POSTGRES_PASSWORD='ganti-dengan-password-lokal'
MINI_CHAT_JWT_SECRET='tempel-hasil-openssl-di-sini'
MINI_CHAT_ACCESS_TOKEN_TTL=1h
```

Secret JWT wajib berukuran minimal 32 byte. Jangan mencatatnya ke log,
memasukkannya ke commit, atau membagikannya. File `docker/.env` hanya digunakan
secara lokal dan sudah diabaikan oleh Git.

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

Konfigurasi wajib server adalah:

- `MINI_CHAT_POSTGRES_URL`, yaitu URL koneksi PostgreSQL;
- `MINI_CHAT_JWT_SECRET`, yaitu secret minimal 32 byte untuk menandatangani
  access token.

`MINI_CHAT_ACCESS_TOKEN_TTL` bersifat opsional dan menggunakan nilai default
`1h`. Nilainya harus berupa durasi Go yang positif, misalnya `30m` atau `2h`.

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

## Login

Endpoint login:

```text
POST /auth/login
Content-Type: application/json
```

Contoh request:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username": " ALICE_STEP5 ",
    "password": "practice-password"
  }'
```

Username login di-trim dan diubah menjadi huruf kecil sehingga contoh di atas
mencari `alice_step5`. Password digunakan persis seperti input dan tidak
di-trim karena spasi dapat menjadi bagian dari password.

Login berhasil menghasilkan `HTTP/1.1 200 OK` dengan bentuk response:

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_at": "2026-10-16T15:00:00Z",
  "identity": {
    "id": 1,
    "username": "alice_step5",
    "display_name": "Alice Example",
    "created_at": "2026-10-16T13:00:00Z",
    "updated_at": "2026-10-16T13:00:00Z"
  }
}
```

Response login tidak pernah memuat password atau `password_hash`. Password
salah dan username yang tidak dikenal sama-sama menghasilkan respons yang
tidak membocorkan bagian credential mana yang salah:

```text
HTTP/1.1 401 Unauthorized
```

```json
{"error":"invalid credentials"}
```

Ketika username tidak ditemukan, server tetap menjalankan verifikasi Argon2id
terhadap dummy hash. Ini membantu mengurangi perbedaan waktu respons yang dapat
digunakan untuk menebak apakah sebuah username sudah terdaftar.

Error request login lainnya:

- JSON rusak, field tidak dikenal, atau lebih dari satu nilai JSON menghasilkan
  `400 Bad Request`;
- body lebih dari 64 KiB menghasilkan `413 Request Entity Too Large`;
- `Content-Type` selain `application/json` menghasilkan
  `415 Unsupported Media Type`.

## JWT access token

Access token ditandatangani dan diverifikasi hanya dengan algoritma `HS256`.
Claims yang digunakan adalah:

| Claim | Isi |
|---|---|
| `iss` | Selalu `mini-chat` |
| `sub` | ID identity berupa bilangan bulat positif |
| `iat` | Waktu token diterbitkan |
| `exp` | Waktu token kedaluwarsa |

Token dengan algoritma, issuer, subject, signature, atau expiry yang tidak
valid akan ditolak. Detail penyebab verifikasi gagal tidak dikirim ke client.

## Current user

Endpoint current user memerlukan access token pada header Bearer:

```text
GET /auth/me
Authorization: Bearer <access-token>
```

Alurnya:

```text
Authorization header
  → verifikasi JWT
  → identity ID dari claim sub
  → Principal di request context
  → current-user use case
  → lookup profil terbaru dari PostgreSQL
  → public identity response
```

Simpan sementara token dari response login tanpa memasukkannya ke source code
atau Git, lalu panggil endpoint:

```bash
export MINI_CHAT_ACCESS_TOKEN='tempel-access-token-di-sini'

curl -i \
  -H "Authorization: Bearer ${MINI_CHAT_ACCESS_TOKEN}" \
  http://127.0.0.1:28080/auth/me
```

Respons berhasil menggunakan `HTTP/1.1 200 OK` dan mengambil profil terbaru
dari PostgreSQL:

```json
{
  "id": 1,
  "username": "alice_step5",
  "display_name": "Alice Example",
  "created_at": "2026-10-16T13:00:00Z",
  "updated_at": "2026-10-16T13:00:00Z"
}
```

Respons tidak memuat password, password hash, atau access token. Header yang
hilang, token rusak, signature salah, atau token kedaluwarsa menghasilkan
respons generik yang sama tanpa detail JWT:

```text
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer
```

```json
{"error":"unauthorized"}
```

Hapus token dari shell setelah pengujian:

```bash
unset MINI_CHAT_ACCESS_TOKEN
```

## Dependency flow identity

Dependency server untuk identity disusun satu kali di `main.go`:

```text
PostgreSQL pool
└── IdentityStore
    ├── Register use case
    ├── Login use case
    └── Current-user use case

Argon2IDPasswordHasher
├── register: Hash
└── login: Verify dan VerifyDummy

JWTManager
├── login: Issue
└── authentication middleware: Verify
```

Satu instance Argon2id hasher dipakai bersama agar batas concurrency hashing
berlaku untuk register dan login. Satu JWT manager juga dipakai bersama untuk
menerbitkan dan memverifikasi token.

Step ini hanya menyediakan access token. Belum ada refresh token, logout atau
revocation, Redis-backed session, maupun sinkronisasi session multi-device yang
kompleks.

## Development rules

- Satu commit untuk satu milestone yang selesai.
- Jalankan test yang relevan sebelum commit.
- Gunakan UTC untuk waktu yang disimpan server.
- Jangan commit secret atau file `.env`.
- Jangan mencatat password, access token, atau credential ke log.
- Prioritaskan vertical slice yang dapat dijalankan end-to-end.
- `practice` dan repository BBGE hanya menjadi referensi dan tidak diubah.

Lihat [roadmap](docs/ROADMAP.md) untuk urutan implementasi.
