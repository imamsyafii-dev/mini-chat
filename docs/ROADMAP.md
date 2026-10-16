# Mini Chat Roadmap

Roadmap ini memprioritaskan aplikasi end-to-end yang dapat dijalankan dalam
tiga hari. Nomor milestone menjadi nomor step dan satu milestone menghasilkan
satu commit.

## Hari 1 — Server dan persistence

- [x] Step 1: menetapkan project contract dan roadmap
- [x] Step 2: membuat Go server, config, health endpoint, dan graceful shutdown
- [x] Step 3: menjalankan PostgreSQL dan Redis melalui Docker Compose
- [x] Step 4: membuat migration identity, conversation, member, dan message
- [x] Step 5: membuat register, Argon2id password hashing, dan persistence
- [ ] Step 6: membuat login, JWT authentication, dan current-user endpoint
- [ ] Step 7: membuat direct conversation serta HTTP send/history message

## Hari 2 — Realtime

- [ ] Step 8: membuat authenticated WebSocket hub
- [ ] Step 9: membuat Redis publisher dan subscriber
- [ ] Step 10: meneruskan message-created event ke penerima melalui WebSocket
- [ ] Step 11: membuat reconnect dan missed-message recovery
- [ ] Step 12: menguji realtime menggunakan Alice dan Bob

## Hari 3 — Desktop dan Android

- [ ] Step 13: membuat React dan Tauri desktop shell
- [ ] Step 14: membuat Rust commands untuk identity dan message HTTP
- [ ] Step 15: membuat Rust WebSocket client dan Tauri event bridge
- [ ] Step 16: membuat desktop login dan chat UI
- [ ] Step 17: membuat React Native bare Android application
- [ ] Step 18: membuat Android login, chat, dan WebSocket client
- [ ] Step 19: menguji chat realtime desktop dengan Android emulator
- [ ] Step 20: final verification dan dokumentasi penggunaan

## Definition of done

Sprint dianggap selesai ketika:

1. Dua user dapat register dan login.
2. Desktop dapat mengirim pesan ke Android.
3. Android menerima pesan tanpa refresh manual.
4. Android dapat membalas dan desktop menerima balasan.
5. Pesan tetap tersedia setelah server atau client dijalankan ulang.
6. Client mengambil pesan yang terlewat setelah reconnect.

## Setelah sprint

Arsitektur dapat dilanjutkan dengan:

- friend request;
- group conversation;
- read receipt;
- typing indicator;
- attachment;
- push notification;
- WuKongIM;
- multi-device session;
- message encryption.

Fitur lanjutan tidak menjadi bagian dari target tiga hari.
