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

## Direct conversation

Step 7 menyediakan pembuatan dan daftar direct conversation. Direct
conversation adalah wadah yang menghubungkan tepat dua identity; isi chat di
dalam wadah tersebut berupa message yang ditambahkan pada Step 8. Redis,
WebSocket, dan pengiriman realtime belum digunakan pada kedua step ini.

### Model data dan aturan domain

Satu direct conversation mempunyai dua participant. Domain selalu menyusun ID
keduanya menjadi pasangan canonical:

```text
identity_low_id  = ID yang lebih kecil
identity_high_id = ID yang lebih besar
```

Karena pasangan tersebut memiliki unique constraint, `Alice → Bob` dan
`Bob → Alice` selalu menunjuk conversation yang sama. Membuat conversation
dengan diri sendiri ditolak.

Pembuatan data dilakukan dalam satu transaction yang mencakup:

1. satu row `conversation`;
2. satu row `direct_conversation` dengan pasangan canonical;
3. dua row `conversation_member`.

Identity yang pertama kali membuat conversation mendapat role `owner`,
sedangkan identity lainnya mendapat role `member`. Request kebalikan yang
dilakukan kemudian hanya mengambil conversation yang sudah ada sehingga tidak
mengubah role tersebut. Jika dua request bersamaan mencoba membuat pasangan
yang sama, unique constraint menentukan satu pemenang. Transaction yang kalah
di-rollback lalu mengambil conversation pemenang, sehingga tidak meninggalkan
conversation atau member yatim.

### Dependency flow

```text
HTTP request + Bearer token
  → authentication middleware
  → Principal berisi identity ID
  → create-direct atau list-conversations use case
  → IdentityStore dan/atau ConversationStore
  → PostgreSQL
```

Create-direct memakai `IdentityStore` untuk mencari profil publik penerima,
kemudian memakai `ConversationStore` untuk membuat atau mengambil conversation.
List-conversations langsung memakai `ConversationStore` karena alurnya hanya
validasi actor ID dan pembacaan sederhana; tidak ada domain service tambahan
yang diperlukan. Satu instance `ConversationStore` dibuat di `main.go` dan
dipakai bersama oleh kedua use case.

### Membuat direct conversation

Endpoint berikut memerlukan access token:

```text
POST /conversations/direct
Authorization: Bearer <access-token>
Content-Type: application/json
```

Request:

```json
{
  "recipient_username": "bob"
}
```

Username penerima di-trim dan diubah menjadi huruf kecil menggunakan aturan
domain identity yang sama dengan register dan login. Conversation baru
menghasilkan `201 Created`. Pasangan yang sudah ada menghasilkan `200 OK`
dengan ID conversation yang sama.

Contoh response publik:

```json
{
  "id": 10,
  "kind": "direct",
  "created_by": 1,
  "participant": {
    "id": 2,
    "username": "bob",
    "display_name": "Bob",
    "created_at": "2026-10-17T08:00:00Z",
    "updated_at": "2026-10-17T08:00:00Z"
  },
  "created_at": "2026-10-17T08:05:00Z",
  "updated_at": "2026-10-17T08:05:00Z"
}
```

Status error:

| Status | Kondisi |
|---|---|
| `400 Bad Request` | JSON atau username tidak valid, field tidak dikenal, atau participant ID tidak valid |
| `401 Unauthorized` | Bearer token hilang atau tidak valid |
| `404 Not Found` | Username penerima tidak ditemukan |
| `409 Conflict` | Mencoba membuat conversation dengan diri sendiri |
| `413 Request Entity Too Large` | Body lebih dari 64 KiB |
| `415 Unsupported Media Type` | `Content-Type` bukan `application/json` |
| `500 Internal Server Error` | Kegagalan internal yang tidak aman untuk dijelaskan ke client |

Response tidak pernah memuat password atau `password_hash`.

### Melihat daftar conversation

```text
GET /conversations
Authorization: Bearer <access-token>
```

Request berhasil menghasilkan `200 OK`. Bearer token yang hilang atau tidak
valid menghasilkan `401 Unauthorized`, sedangkan kegagalan internal yang tidak
aman dijelaskan ke client menghasilkan `500 Internal Server Error`.

Response menggunakan array langsung. Field `participant` selalu berisi lawan
bicara dari pengguna yang sedang login:

```json
[
  {
    "id": 10,
    "kind": "direct",
    "created_by": 1,
    "participant": {
      "id": 2,
      "username": "bob",
      "display_name": "Bob",
      "created_at": "2026-10-17T08:00:00Z",
      "updated_at": "2026-10-17T08:00:00Z"
    },
    "created_at": "2026-10-17T08:05:00Z",
    "updated_at": "2026-10-17T08:05:00Z"
  }
]
```

Jika belum ada conversation, response adalah `[]`, bukan `null`. Server
mengambil maksimal 50 conversation dengan urutan `updated_at DESC, id DESC`.
Sejak Step 8, setiap message yang berhasil disimpan juga memperbarui
`conversation.updated_at`, sehingga conversation dengan aktivitas pesan terbaru
berada di bagian awal daftar. Endpoint ini belum memiliki pagination.

### Pengujian manual Step 7

Pastikan PostgreSQL, migration, dan server sudah berjalan. Gunakan dua username
unik, lalu masukkan password tanpa menampilkannya di terminal:

```bash
MINI_CHAT_RUN_ID=$(date +%Y%m%d%H%M%S)
export ALICE_USERNAME="alice_${MINI_CHAT_RUN_ID}"
export BOB_USERNAME="bob_${MINI_CHAT_RUN_ID}"
read -s "MINI_CHAT_E2E_PASSWORD?Password E2E: "; echo
export MINI_CHAT_E2E_PASSWORD
```

Register Alice dan Bob:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/auth/register \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${ALICE_USERNAME}\",\"password\":\"${MINI_CHAT_E2E_PASSWORD}\",\"display_name\":\"Alice E2E\"}"

curl -i \
  -X POST http://127.0.0.1:28080/auth/register \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${BOB_USERNAME}\",\"password\":\"${MINI_CHAT_E2E_PASSWORD}\",\"display_name\":\"Bob E2E\"}"
```

Login keduanya untuk mendapatkan access token:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${ALICE_USERNAME}\",\"password\":\"${MINI_CHAT_E2E_PASSWORD}\"}"

curl -i \
  -X POST http://127.0.0.1:28080/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${BOB_USERNAME}\",\"password\":\"${MINI_CHAT_E2E_PASSWORD}\"}"
```

Salin masing-masing `access_token` dari response dan simpan tanpa
menampilkannya saat ditempel:

```bash
read -s "ALICE_TOKEN?Token Alice: "; echo
read -s "BOB_TOKEN?Token Bob: "; echo
export ALICE_TOKEN BOB_TOKEN
```

Buat conversation sebagai Alice:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/conversations/direct \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"recipient_username\":\"${BOB_USERNAME}\"}"
```

Request pertama harus menghasilkan `201`. Jalankan request yang sama sekali
lagi dan pastikan hasilnya `200` dengan ID yang sama. Kemudian jalankan arah
kebalikannya dari sisi Bob:

```bash
curl -i \
  -X POST http://127.0.0.1:28080/conversations/direct \
  -H "Authorization: Bearer ${BOB_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"recipient_username\":\"${ALICE_USERNAME}\"}"
```

Request Bob juga harus menghasilkan `200` dengan ID yang sama. Simpan ID
tersebut untuk pemeriksaan data:

```bash
read "CONVERSATION_ID?Conversation ID: "
```

Periksa daftar dari kedua sisi:

```bash
curl -i \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  http://127.0.0.1:28080/conversations

curl -i \
  -H "Authorization: Bearer ${BOB_TOKEN}" \
  http://127.0.0.1:28080/conversations
```

Periksa tiga error utama:

```bash
# Chat dengan diri sendiri: 409 Conflict
curl -i \
  -X POST http://127.0.0.1:28080/conversations/direct \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"recipient_username\":\"${ALICE_USERNAME}\"}"

# Username valid tetapi tidak terdaftar: 404 Not Found
curl -i \
  -X POST http://127.0.0.1:28080/conversations/direct \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"recipient_username\":\"missing_${MINI_CHAT_RUN_ID}\"}"

# Daftar conversation tanpa token: 401 Unauthorized
curl -i http://127.0.0.1:28080/conversations
```

Checklist hasil:

- kedua daftar memakai ID conversation yang sama;
- daftar Alice menampilkan Bob sebagai `participant` dan sebaliknya;
- response tidak memuat password atau hash;
- create dengan username sendiri menghasilkan `409`;
- recipient yang tidak ada menghasilkan `404`;
- request tanpa token menghasilkan `401`.

Dari folder `docker`, pastikan conversation mempunyai tepat dua membership:

```bash
docker compose exec -T postgres \
  psql -U mini_chat -d mini_chat -c "
    SELECT i.username, cm.role
    FROM conversation_member AS cm
    JOIN identity AS i ON i.id = cm.identity_id
    WHERE cm.conversation_id = ${CONVERSATION_ID}
    ORDER BY cm.role, i.username;
  "
```

Hasilnya harus memuat dua row: pembuat pertama sebagai `owner` dan pengguna
lainnya sebagai `member`. Pertahankan variable pengujian karena akan langsung
digunakan kembali pada Step 8.

## HTTP message dan history

Step 8 menambahkan isi chat ke direct conversation yang dibuat pada Step 7.
Perbedaannya adalah:

```text
direct conversation = wadah chat dan daftar anggotanya
message             = satu isi chat di dalam wadah tersebut
```

Message yang sudah tersimpan mempunyai field berikut:

| Field | Isi |
|---|---|
| `id` | ID message dari PostgreSQL |
| `conversation_id` | ID wadah conversation |
| `sender_id` | ID identity pengirim dari JWT Principal |
| `body` | Isi pesan yang sudah divalidasi domain |
| `created_at` | Waktu message disimpan |

Client tidak dapat menentukan `sender_id` sendiri. Authentication middleware
memverifikasi Bearer token, lalu menaruh identity ID pada `Principal` di request
context. Alur dependency lengkapnya adalah:

```text
Bearer token
  → authentication middleware
  → Principal berisi identity ID
  → send-message atau list-messages use case
  → satu MessageStore yang dipakai bersama
  → PostgreSQL
```

### Aturan domain message

Body dirapikan dengan `TrimSpace`, sehingga spasi dan baris kosong di awal atau
akhir dibuang. Isi di tengah message tetap dipertahankan. Setelah dirapikan,
body wajib berukuran 1–4.000 karakter Unicode (rune), bukan sekadar 4.000 byte.
Conversation ID dan sender ID juga wajib berupa bilangan bulat positif.

### Mengirim message

Endpoint berikut memerlukan access token dan membership pada conversation:

```text
POST /conversations/{conversationID}/messages
Authorization: Bearer <access-token>
Content-Type: application/json
```

Request hanya menerima field `body`:

```json
{
  "body": "Hello Bob"
}
```

Message yang berhasil disimpan menghasilkan `201 Created`:

```json
{
  "id": 101,
  "conversation_id": 10,
  "sender_id": 1,
  "body": "Hello Bob",
  "created_at": "2026-10-17T09:00:00Z"
}
```

Pengiriman message menggunakan satu transaction PostgreSQL:

```text
BEGIN
  → pastikan sender adalah member dan lock conversation
  → INSERT message
  → UPDATE conversation.updated_at dengan waktu message
COMMIT
```

Jika salah satu operasi sebelum commit gagal, seluruh transaction di-rollback.
Dengan demikian tidak ada keadaan message tersimpan tanpa memperbarui waktu
conversation, atau waktu conversation berubah tanpa message. Update memakai
`GREATEST`, sehingga waktu aktivitas tidak bergerak mundur ketika beberapa
pengiriman terjadi bersamaan.

Status response endpoint send:

| Status | Kondisi |
|---|---|
| `201 Created` | Message berhasil disimpan |
| `400 Bad Request` | Conversation ID, JSON, field request, atau body tidak valid |
| `401 Unauthorized` | Bearer token hilang atau tidak valid |
| `404 Not Found` | Conversation tidak ada atau pengirim bukan member |
| `413 Request Entity Too Large` | Body request lebih dari 64 KiB |
| `415 Unsupported Media Type` | `Content-Type` bukan `application/json` |
| `500 Internal Server Error` | Kegagalan internal yang tidak aman dijelaskan ke client |

### Mengambil message history

Halaman pertama:

```text
GET /conversations/{conversationID}/messages
Authorization: Bearer <access-token>
```

Contoh response `200 OK` untuk halaman pertama berikut dipersingkat agar mudah
dibaca. Message dengan ID `52`–`99` tidak ditampilkan pada potongan JSON;
response sebenarnya berisi 50 message ketika `next_before_id` tersedia:

```json
{
  "messages": [
    {
      "id": 51,
      "conversation_id": 10,
      "sender_id": 1,
      "body": "Pesan lama",
      "created_at": "2026-10-17T09:00:00Z"
    },
    {
      "id": 100,
      "conversation_id": 10,
      "sender_id": 2,
      "body": "Pesan baru",
      "created_at": "2026-10-17T09:10:00Z"
    }
  ],
  "next_before_id": 51
}
```

Karena response tersebut mempunyai `next_before_id` bernilai `51`, halaman
yang lebih lama diminta menggunakan cursor positif itu:

```text
GET /conversations/{conversationID}/messages?before_id=51
Authorization: Bearer <access-token>
```

Request lanjutan hanya mengambil message dengan ID lebih kecil dari `51`.

Store meminta 51 row dengan urutan ID terbaru terlebih dahulu. Use case
mengembalikan maksimal 50 message, membalik hasil menjadi urutan lama → baru
agar mudah ditampilkan client, dan memakai ID message tertua yang masih
dikembalikan sebagai `next_before_id`. Request berikutnya memakai cursor itu dan
mengambil message dengan `id < before_id`, sehingga tidak ada message yang
terlewat atau dikirim dua kali. Jika tidak ada halaman berikutnya,
`next_before_id` bernilai `null`.

History kosong selalu berbentuk array kosong, bukan `null`:

```json
{
  "messages": [],
  "next_before_id": null
}
```

Status response endpoint history:

| Status | Kondisi |
|---|---|
| `200 OK` | History berhasil diambil, termasuk ketika kosong |
| `400 Bad Request` | Conversation ID atau `before_id` tidak valid |
| `401 Unauthorized` | Bearer token hilang atau tidak valid |
| `404 Not Found` | Conversation tidak ada atau pembaca bukan member |
| `500 Internal Server Error` | Kegagalan internal yang tidak aman dijelaskan ke client |

Baik send maupun history selalu memeriksa `conversation_member`. Conversation
yang tidak ada dan conversation yang ada tetapi tidak boleh diakses sengaja
menghasilkan response yang sama:

```text
HTTP/1.1 404 Not Found
```

```json
{"error":"conversation is unavailable"}
```

Penyamaan response ini mencegah pengguna menebak keberadaan conversation yang
bukan miliknya.

### Pengujian manual Step 8

Gunakan `ALICE_TOKEN`, `BOB_TOKEN`, dan `CONVERSATION_ID` yang masih tersimpan
dari pengujian Step 7. Kirim message sebagai Alice:

```bash
curl -i \
  -X POST "http://127.0.0.1:28080/conversations/${CONVERSATION_ID}/messages" \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"body":"Hello Bob from Step 8"}'
```

Ambil history sebagai Bob, lalu kirim balasan:

```bash
curl -i \
  -H "Authorization: Bearer ${BOB_TOKEN}" \
  "http://127.0.0.1:28080/conversations/${CONVERSATION_ID}/messages"

curl -i \
  -X POST "http://127.0.0.1:28080/conversations/${CONVERSATION_ID}/messages" \
  -H "Authorization: Bearer ${BOB_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"body":"Hello Alice from Step 8"}'
```

Ambil kembali history sebagai Alice dan pastikan urutannya lama → baru:

```bash
curl -i \
  -H "Authorization: Bearer ${ALICE_TOKEN}" \
  "http://127.0.0.1:28080/conversations/${CONVERSATION_ID}/messages"
```

Checklist E2E:

- send Alice dan Bob masing-masing menghasilkan `201 Created`;
- `sender_id` selalu berasal dari identity pada token;
- Alice dan Bob melihat message yang sama dalam urutan lama → baru;
- body dengan spasi luar disimpan dalam bentuk yang sudah di-trim;
- body kosong atau lebih dari 4.000 rune menghasilkan `400 Bad Request`;
- `before_id` nol, negatif, teks, kosong, atau dikirim lebih dari sekali
  menghasilkan `400 Bad Request`;
- request tanpa token menghasilkan `401 Unauthorized`;
- identity ketiga yang bukan member mendapat `404 Not Found` untuk send dan
  history, sama seperti conversation yang tidak ada;
- setelah server dihentikan dan dijalankan kembali, history yang sama masih
  tersedia karena PostgreSQL adalah sumber data permanen;
- daftar `GET /conversations` menempatkan conversation yang baru menerima
  message lebih awal berdasarkan `updated_at`.

Setelah seluruh pengujian Step 8 selesai, bersihkan variable sementara:

```bash
unset ALICE_TOKEN BOB_TOKEN ALICE_USERNAME BOB_USERNAME
unset MINI_CHAT_E2E_PASSWORD MINI_CHAT_RUN_ID CONVERSATION_ID
```

Step 8 hanya menyediakan send dan history melalui HTTP serta penyimpanan
permanen PostgreSQL. Message belum didorong otomatis ke client lain. Redis,
authenticated WebSocket, event realtime, reconnect, dan missed-message
recovery mulai dikerjakan pada Step 9 dan milestone setelahnya.

## Development rules

- Satu commit untuk satu milestone yang selesai.
- Jalankan test yang relevan sebelum commit.
- Gunakan UTC untuk waktu yang disimpan server.
- Jangan commit secret atau file `.env`.
- Jangan mencatat password, access token, atau credential ke log.
- Prioritaskan vertical slice yang dapat dijalankan end-to-end.
- `practice` dan repository BBGE hanya menjadi referensi dan tidak diubah.

Lihat [roadmap](docs/ROADMAP.md) untuk urutan implementasi.
