# Modul CS — Shared WhatsApp Inbox

**Tanggal**: 2026-09-02
**Proyek**: TikMan
**Versi**: 1.0

## 1. Masalah

Satu nomor WhatsApp CS dipegang bergantian oleh banyak orang. Karena aplikasi
WhatsApp hanya mengizinkan satu sesi utama, setiap kali seorang CS masuk di HP-nya
sendiri, CS lain terlempar keluar. Saat menangani gangguan, mereka harus berpindah
perangkat, dan aduan pelanggan tertinggal di HP yang sedang tidak dipegang siapa pun.

Modul ini memindahkan nomor CS itu ke TikMan sebagai *shared inbox*: satu nomor,
banyak petugas, tanpa ada yang perlu login WhatsApp di HP masing-masing.

## 2. Keputusan yang Melandasi Desain

| Keputusan | Pilihan | Alasan |
|---|---|---|
| Jalur WhatsApp | `whatsmeow` (self-host, unofficial) | Native Go, nomor tetap nomor WA biasa, tanpa jendela 24 jam dan tanpa biaya template |
| Pembagian kerja | Round-robin otomatis, dengan ambil alih manual | Adil tanpa perlu supervisor; ambil alih menutup kasus CS mendadak tidak di tempat |
| Sumber nomor pelanggan | Input manual per ONT | Tidak ada data nomor HP di database saat ini, dan tidak ada sistem lain untuk ditarik |
| Realtime | SSE | ~1-2 pesan/detik pada puncak dengan ~30 koneksi; Gin sudah mendukung SSE tanpa dependensi baru |
| Penyimpanan media | Volume Docker di server | Paling sederhana; ditebus dengan penghapusan otomatis media lama |
| Penempatan proses | Container `wa` tersendiri | `worker` mewarisi `wg0` milik `api` dan dirancang untuk di-scale; keduanya racun bagi koneksi WhatsApp yang harus tunggal |

## 3. Risiko yang Diterima

whatsmeow adalah jalur tidak resmi. Nomor bisa diblokir WhatsApp, dan tidak ada
desain yang bisa menghilangkan risiko itu — hanya menahan lajunya. Volume yang
direncanakan (ribuan pesan per hari melalui satu nomor) adalah profil risiko
tertinggi. Yang dilakukan desain ini:

- Tidak ada pengiriman massal. Sistem hanya membalas orang yang menghubungi lebih dulu.
- Pembatas laju per nomor, sehingga pengiriman tidak pernah meledak sekaligus.
- Sambung ulang memakai jeda yang membesar, bukan percobaan tanpa henti.
- Tabel `wa_accounts` menyediakan tempat bagi nomor kedua sejak awal, sehingga
  beban dapat dibagi dan satu nomor terblokir tidak mematikan seluruh CS.

Dua batasan yang melekat pada jalur ini dan tidak bisa didesain hilang:

- **Riwayat chat lama tidak ikut.** whatsmeow hanya menerima sebagian kecil
  riwayat saat pairing. Inbox dimulai dari kosong.
- **Sesi dapat diputus** oleh HP utama (menghapus perangkat tertaut) atau oleh
  WhatsApp sendiri. Karena itu status koneksi tampil mencolok di halaman inbox.

## 4. Arsitektur

```
                 ┌────────────┐
 Pelanggan ──WA──│ container  │──tulis──▶ Postgres ◀──baca── ┌─────┐
                 │    wa      │                              │ api │──SSE──▶ Browser CS
                 │ (whatsmeow)│──umumkan─▶ Redis ──langgan──▶ └─────┘
                 └────────────┘                                  │
                       ▲                                         │
                       └────────── ambil pesan queued ───────────┘
```

**Container `wa`** — proses tunggal (tidak boleh di-scale) di `tikman-network`,
tanpa `wg0`. Memegang koneksi whatsmeow, menulis pesan masuk ke Postgres,
mengunduh media ke disk, dan mengirim pesan keluar yang menunggu di antrean.

**Container `api`** — menyajikan REST dan SSE untuk browser CS. Tidak pernah
berbicara langsung ke WhatsApp.

**Postgres** adalah sumber kebenaran. **Redis** hanya mempercepat: kalau Redis
mati, penyapu berkala tetap membuat sistem benar, hanya terlambat.

## 5. Skema Database

Migrasi `41_add_cs_module.sql`, mengikuti penamaan migrasi yang sudah ada di repo.

### 5.1 `wa_accounts`
Nomor WhatsApp yang tersambung.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid PK | |
| `label` | varchar(100) | Nama tampilan, mis. "CS Utama" |
| `jid` | varchar(64) | JID WhatsApp, kosong sebelum pairing |
| `status` | varchar(20) | `disconnected` / `pairing` / `connected` / `banned` |
| `last_connected_at` | timestamptz null | |
| `created_at`, `updated_at` | timestamptz | |

Sesi whatsmeow sendiri disimpan pada tabel milik library (`sqlstore` Postgres
bawaan whatsmeow), terpisah dari tabel ini.

### 5.2 `cs_conversations`
Satu baris per pasangan (nomor CS, nomor pelanggan).

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid PK | |
| `wa_account_id` | uuid FK | |
| `customer_jid` | varchar(64) | Unik bersama `wa_account_id` |
| `customer_phone` | varchar(20) | Ternormalisasi `628xxx` |
| `customer_name` | varchar(255) | Nama profil WhatsApp |
| `assigned_user_id` | uuid FK null | Pemegang saat ini |
| `status` | varchar(20) | `unassigned` / `open` / `closed` |
| `ont_id` | uuid FK null | ONT yang tertaut |
| `last_message_at` | timestamptz | Indeks untuk pengurutan inbox |
| `unread_count` | int | |
| `created_at`, `updated_at` | timestamptz | |

### 5.3 `cs_messages`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid PK | |
| `conversation_id` | uuid FK | |
| `wa_message_id` | varchar(128) | **Indeks unik** — kunci idempotensi |
| `direction` | varchar(3) | `in` / `out` |
| `sender_user_id` | uuid FK null | Kosong untuk pesan masuk |
| `kind` | varchar(20) | `text` / `image` / `document` / `audio` / `video` |
| `body` | text | Isi teks atau caption |
| `media_path` | text null | Jalur relatif di volume media |
| `media_mime` | varchar(100) null | |
| `media_size` | bigint null | |
| `media_filename` | varchar(255) null | |
| `status` | varchar(20) | `queued` / `sent` / `delivered` / `read` / `failed` |
| `fail_reason` | text null | |
| `wa_timestamp` | timestamptz | |
| `tsv` | tsvector | Kolom terkalkulasi dari `body`, indeks GIN |
| `created_at` | timestamptz | |

Baris berstatus `queued` **adalah** outbox. Tidak ada tabel outbox terpisah.

### 5.4 `cs_quick_replies`

| Kolom | Tipe |
|---|---|
| `id` | uuid PK |
| `title` | varchar(100) |
| `body` | text |
| `created_by` | uuid FK |
| `created_at`, `updated_at` | timestamptz |

### 5.5 Perubahan tabel yang sudah ada

- `onts` mendapat kolom `phone varchar(20)` dengan indeks unik parsial
  (`WHERE phone IS NOT NULL`), agar satu nomor tidak menempel di dua ONT.
- `users.role` menerima nilai baru `cs`, di samping `admin` / `technician` / `viewer`.

### 5.6 Bukan di database

Status online CS disimpan di Redis (`cs:online:<user_id>`, TTL 60 detik),
diperbarui oleh denyut koneksi SSE. Kalau browser tertutup, statusnya luruh
sendiri tanpa perlu pembersihan.

## 6. Alur Pesan

### 6.1 Masuk
1. `wa` menerima event pesan dari whatsmeow.
2. Nomor pengirim dinormalisasi; percakapan dicari atau dibuat.
3. Untuk percakapan baru, nomor dicocokkan ke `onts.phone`; `ont_id` diisi bila cocok.
4. Bila ada media: diunduh dan ditulis ke `/data/cs-media/<tahun>/<bulan>/<uuid>`.
5. Baris `cs_messages` disimpan. Konflik pada `wa_message_id` diabaikan —
   pengiriman ulang dari WhatsApp tidak menghasilkan pesan kembar.
6. Bila percakapan belum ada pemegangnya dan bukan `closed`, jalankan penugasan (§7).
7. Umumkan ke Redis channel `cs:events`.

### 6.2 Keluar
1. CS mengirim `POST /api/cs/conversations/:id/messages`.
2. API memvalidasi bahwa pengirim adalah pemegang percakapan.
3. Baris `cs_messages` ditulis berstatus `queued`; diumumkan ke `cs:outbox`.
4. `wa` mengambilnya, mengirim lewat whatsmeow, lalu memperbarui status menjadi
   `sent` berikut `wa_message_id`, atau `failed` berikut sebabnya.
5. Tanda terima WhatsApp memperbarui status menjadi `delivered` lalu `read`.

### 6.3 Pengaman
- **Penyapu tiap 30 detik** memungut pesan `queued` yang lebih tua dari satu menit.
  Pengumuman Redis yang hilang atau proses `wa` yang sempat mati tidak membuat
  balasan CS menguap.
- **Pembatas laju** (token bucket) per akun WhatsApp.

## 7. Penugasan

Daftar CS yang dianggap online adalah pemilik kunci `cs:online:<user_id>` yang
masih hidup — yaitu yang koneksi SSE-nya terbuka. Konsekuensinya, teknisi yang
tidak membuka inbox tidak pernah kebagian percakapan, tanpa perlu tombol
kesiapan terpisah.

Percakapan baru dibagikan bergilir ke daftar tersebut, diurutkan berdasarkan
`user_id`, dengan penunjuk giliran di Redis (`cs:rr:pointer`).

Bila tidak ada yang online, percakapan tetap `unassigned` dan menunggu. Saat CS
pertama membuka inbox, seluruh percakapan `unassigned` dibagikan bergilir — chat
malam hari tidak hilang, hanya menunggu pagi.

Percakapan `closed` yang menerima pesan baru dibuka kembali dan dibagikan ulang.

**Ambil alih dan oper.** Setiap CS boleh mengambil alih percakapan orang lain
atau mengopernya, dan setiap perpindahan dicatat lewat `AuditService` yang sudah
ada. Otomatis untuk keadaan normal, manual sebagai jalan keluar.

## 8. Realtime

`GET /api/cs/stream` (SSE, satu koneksi per CS). API berlangganan channel Redis
dan meneruskan peristiwa: pesan baru, perubahan penugasan, perubahan status
percakapan, dan perubahan status koneksi WhatsApp. Denyut tiap 15 detik menjaga
koneksi tetap hidup melewati proxy sekaligus memperpanjang penanda online.

Seluruh CS melihat seluruh inbox — mereka satu tim, dan saling melihat justru
mencegah tabrakan. Yang dibatasi hanya hak membalas.

SSE bukan sumber kebenaran, hanya pemicu. Saat koneksi putus dan tersambung
kembali, frontend menarik ulang data untuk menutup celah pesan yang terlewat.

## 9. Antarmuka

### 9.1 Halaman inbox `/cs`
Tiga kolom:

**Kiri** — daftar percakapan dengan saringan *Milik saya / Belum dipegang /
Semua / Selesai*, pencarian, dan penanda pemegang.

**Tengah** — ruang percakapan: gelembung chat, unggah foto, balasan cepat, serta
tombol Ambil alih, Oper ke CS lain, dan Tandai selesai. Tombol kirim mati bila
percakapan dipegang orang lain, dengan keterangan "Dipegang <nama> — ambil alih
dulu", bukan tombol abu-abu tanpa penjelasan.

**Kanan** — panel pelanggan: nama, nomor, dan ONT tertaut berikut status
hidup/LOS, redaman RX, ODP dan portnya, dengan tautan ke halaman ONT. Bila
nomornya belum cocok dengan ONT mana pun, panel berisi pencarian untuk menautkan
manual — sehingga data nomor HP terisi sambil jalan, bukan lewat proyek
pengisian data tersendiri.

Lencana status koneksi WhatsApp tampil di halaman ini, bukan tersembunyi di
pengaturan.

### 9.2 Pengaturan → WhatsApp
Status koneksi, tombol Sambungkan (menampilkan QR untuk dipindai dari HP pemegang
nomor), dan Putuskan. Server masuk sebagai perangkat tertaut seperti WhatsApp
Web, jadi HP utama tetap memegang nomornya.

### 9.3 Form ONT
Bertambah satu isian Nomor HP pelanggan.

## 10. Hak Akses

| Role | Akses |
|---|---|
| `admin` | Seluruh inbox, membalas, ambil alih, sambung/putus WhatsApp, kelola balasan cepat |
| `cs` | Seluruh inbox, membalas, ambil alih, ikut round-robin |
| `technician` | Sama seperti `cs` |
| `viewer` | Tidak ada akses |

## 11. Endpoint

| Metode | Jalur | Role |
|---|---|---|
| GET | `/api/cs/stream` | cs, technician, admin |
| GET | `/api/cs/conversations` | cs, technician, admin |
| GET | `/api/cs/conversations/:id/messages` | cs, technician, admin |
| POST | `/api/cs/conversations/:id/messages` | cs, technician, admin |
| POST | `/api/cs/conversations/:id/media` | cs, technician, admin |
| PUT | `/api/cs/conversations/:id/assign` | cs, technician, admin |
| PUT | `/api/cs/conversations/:id/status` | cs, technician, admin |
| PUT | `/api/cs/conversations/:id/ont` | cs, technician, admin |
| GET | `/api/cs/media/:message_id` | cs, technician, admin |
| GET/POST/PUT/DELETE | `/api/cs/quick-replies` | baca: semua di atas; ubah: admin |
| GET | `/api/cs/wa-accounts` | admin |
| POST | `/api/cs/wa-accounts/:id/connect` | admin |
| POST | `/api/cs/wa-accounts/:id/disconnect` | admin |

## 12. Penanganan Galat

Prinsipnya: tidak ada kegagalan yang diam.

| Kejadian | Perilaku |
|---|---|
| Koneksi WhatsApp putus | Status tersimpan, lencana merah di inbox, sambung ulang dengan jeda membesar |
| Pengiriman gagal | Gelembung menampilkan sebabnya dengan tombol kirim ulang |
| Media gagal diunduh | Pesan tetap tersimpan dan ditandai, agar CS tahu ada kiriman |
| SSE putus | Browser menyambung ulang lalu menarik ulang data |
| Redis mati | Penyapu database tetap bekerja; inbox benar, terlambat sampai 30 detik |
| Nomor cocok ke dua ONT | Dicegah indeks unik parsial; penautan kedua ditolak dengan pesan jelas |

## 13. Retensi

Media lebih tua dari 90 hari dihapus beserta jalurnya di database oleh tugas
harian di `wa`. Baris pesan tetap disimpan; yang hilang hanya berkasnya.

## 14. Struktur Berkas

**Backend**
```
cmd/wa/main.go
internal/wa/client.go        koneksi, pairing, reconnect
internal/wa/inbound.go       penanganan pesan masuk
internal/wa/outbound.go      pengambilan antrean, kirim, pembatas laju
internal/wa/media.go         unduh dan simpan berkas
internal/wa/receipts.go      status delivered/read
internal/services/cs_conversation_service.go
internal/services/cs_message_service.go
internal/services/cs_assignment.go
internal/services/cs_presence.go
internal/services/cs_quick_reply_service.go
internal/services/cs_media_retention.go
internal/api/cs_handler_conversations.go
internal/api/cs_handler_messages.go
internal/api/cs_handler_stream.go
internal/api/cs_handler_wa.go
internal/api/cs_dto.go
internal/models/cs_conversation.go
internal/models/cs_message.go
migrations/41_add_cs_module.sql
```

**Frontend**
```
domain/entities/CsConversation.ts, CsMessage.ts
domain/repositories/ICsRepository.ts
infrastructure/repositories/CsRepository.ts
application/hooks/useCsInbox.ts, useCsStream.ts, useCsQuickReplies.ts
presentation/pages/CsInboxPage.tsx
presentation/components/cs/ConversationList.tsx
presentation/components/cs/MessageThread.tsx
presentation/components/cs/MessageComposer.tsx
presentation/components/cs/CustomerPanel.tsx
presentation/components/cs/QuickReplyPicker.tsx
presentation/components/cs/WaConnectionBadge.tsx
presentation/components/cs/WaPairingModal.tsx
```

**Compose** — service `wa` baru pada `tikman-network`, tanpa `NET_ADMIN`, tanpa
`network_mode: service:api`, dengan volume `cs_media`.

## 15. Pengujian

whatsmeow dibungkus antarmuka kecil (`SendText` / `SendMedia`) agar `outbound.go`
dapat diuji dengan pengganti palsu. `client.go` yang memegang koneksi sungguhan
tidak diuji unit, sejalan dengan pengecualian kode jaringan pada CLAUDE.md.

Yang diuji:

- Round-robin membagi bergilir, melewati CS yang offline, dan menahan percakapan
  sebagai `unassigned` ketika tidak ada yang online.
- Pesan masuk dengan `wa_message_id` yang sama tidak tersimpan dua kali.
- Pengiriman oleh yang bukan pemegang percakapan ditolak.
- Penyapu mengirim ulang pesan `queued` yang tertinggal.
- Percakapan `closed` terbuka kembali saat ada pesan baru.
- RBAC tiap role pada seluruh endpoint.
- Frontend: daftar percakapan, komposer (tombol mati saat bukan pemegang), dan
  panel pelanggan.

## 16. Dependensi Baru

`go.mau.fi/whatsmeow`, dipatok pada versi rilis terbaru saat implementasi,
beserta turunannya `go.mau.fi/libsignal` dan `google.golang.org/protobuf`.

Tidak ada dependensi lain: SSE memakai Gin yang sudah ada, penyimpanan sesi
memakai Postgres yang sudah ada, dan media ditulis dengan pustaka standar.

## 17. Di Luar Cakupan

- Balasan otomatis atau bot penjawab.
- Impor massal nomor pelanggan dari Excel/CSV.
- Panggilan suara dan video WhatsApp.
- Grup WhatsApp — modul ini hanya menangani percakapan pribadi.
- Laporan dan statistik kinerja CS.
