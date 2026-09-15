# Balasan CS dari HP — Desain

**Tanggal:** 2026-09-15
**Status:** Disetujui untuk perencanaan
**Bagian dari:** pengukuran waktu balas CS, bagian 1 dari 2. Bagian 2 (pencatatan
waktu tunggu dan laporannya) didesain terpisah dan bergantung pada bagian ini.

## Kenapa

CS kadang membalas pelanggan langsung dari HP yang memegang nomor WhatsApp, bukan
dari TikMan. Pesan seperti itu dibuang oleh `internal/wa/inbound.go`, sehingga:

- thread tetap berada di tab "Belum dibalas" dan badge belum dibaca tidak hilang,
  padahal pelanggan sudah dijawab;
- CS yang membuka thread tidak melihat apa yang sudah dikatakan lewat HP;
- pengukuran waktu balas di bagian 2 akan salah: waktu tunggu pelanggan terus
  berjalan sampai ada balasan dari TikMan, lalu dibebankan ke CS yang membalas
  berikutnya.

Data production 4–15 September memuat 126 thread yang ditutup tanpa pernah dibalas
di TikMan. Dari data saja tidak bisa dibedakan mana yang dibalas lewat HP, mana
yang spam, dan mana yang memang tidak perlu dibalas.

## Yang sudah ada

- **whatsmeow meneruskan pesan dari perangkat lain akun yang sama.** Pesan yang
  diketik di HP datang sebagai `*events.Message` dengan `Info.IsFromMe = true`,
  `Info.Chat` berisi pelanggan (atribut `recipient` stanza), dan
  `Info.RecipientAlt` berisi bentuk lain alamat pelanggan: nomor telepon bila
  `Chat` berupa LID, LID bila `Chat` berupa nomor. `UnwrapRaw` sudah membuka
  `DeviceSentMessage`, jadi `evt.Message` berbentuk sama dengan pesan pelanggan.
- **Kiriman TikMan tidak pernah kembali ke TikMan.** `encryptMessageForDevices` di
  `send.go` whatsmeow melewati `ownJID` dan `ownLID`, sehingga WhatsApp tidak
  memegang salinan untuk perangkat yang mengirim. Satu-satunya `IsFromMe` yang
  sampai adalah pesan dari perangkat lain, yaitu HP.
- **Jalur pesan masuk:** `inboundHandler.handle` → `attachmentFor` (membuang grup,
  `IsFromMe`, channel) → `FindOrCreate` → `fetch` → `SaveInbound` → `announce`.
- **`CSConversationService.FindByPeer`** mencari thread tanpa membuat baru.
- **`CSMessageService.SaveInbound`** menyimpan dalam satu transaksi: lewati bila
  `wa_message_id` sudah ada (dicek di Go dan dijaga indeks unik parsial migrasi
  41), bentuk baris lewat `inboundRow`, lalu `bumpConversation`.
- **`ApplyReceipt`** memajukan status pesan berdasarkan `wa_message_id`.
- **`PushNotifierService.NotifyIncomingMessage`** hanya mengirim push untuk pesan
  berarah `in`.
- **`CSMessageService.Queue`** selalu mengisi `sender_user_id` dengan CS yang
  menulis balasan.

## Perilaku

Pesan yang diketik di HP untuk pelanggan yang sudah punya thread di nomor itu:

- muncul di thread tersebut secara langsung, seperti balasan biasa, dengan label
  **"dari HP"** karena tidak diketahui CS mana yang memegang HP;
- mengeluarkan thread dari "Belum dibalas" dan menghapus badge belum dibaca, bila
  pesan itu yang paling baru di thread;
- tidak mengklaim thread, tidak membuka ulang thread yang sudah ditutup, dan tidak
  memicu notifikasi push;
- ditempatkan sesuai waktu aslinya. Balasan HP yang baru diterima TikMan setelah
  koneksi pulih tidak menutupi pesan pelanggan yang lebih baru.

Pesan dari HP untuk orang yang belum punya thread tidak disimpan. Tanpa aturan ini,
chat pribadi atau urusan lain dari HP nomor CS ikut masuk ke inbox. Akibatnya,
pesan pembuka yang dikirim CS dari HP ke pelanggan baru tidak terlihat di TikMan;
thread muncul saat pelanggan membalas.

Perangkat lain yang tertaut ke nomor itu, seperti WhatsApp Web atau Desktop,
menghasilkan pesan `IsFromMe` yang sama, jadi balasan dari sana juga berlabel
"dari HP".

**Asumsi:** nomor CS tidak memakai pesan otomatis WhatsApp Business (sambutan,
sedang tidak di tempat, atau balasan AI). Ini dikonfirmasi user pada 2026-09-16.
Pesan otomatis dikirim oleh akun dan tiba sebagai `IsFromMe` tanpa penanda apa
pun dari whatsmeow. Kalau fitur itu diaktifkan, pesan otomatis akan dianggap
balasan dan mengeluarkan thread dari "Belum dibalas" sebelum ada orang yang
menjawab. Desain ini harus ditinjau ulang sebelum fitur itu diaktifkan.

## Komponen

### 1. Pemilahan di `internal/wa/inbound.go`

`attachmentFor` berhenti membuang `IsFromMe`. Grup (termasuk status, yang
whatsmeow tandai sebagai grup karena server broadcast) dan channel tetap dibuang.
Komentar di atasnya diperbarui: yang dibuang bukan lagi "gema sendiri", karena
kiriman TikMan tidak pernah kembali.

`handle` meneruskan event `IsFromMe` ke `handleFromPhone` di file baru
`internal/wa/inbound_phone.go`. Langkah unduh media, simpan, buang file yang tidak
terpakai, dan umumkan dipakai bersama oleh kedua jalur. Yang berbeda hanya cara
mencari thread dan fungsi penyimpannya.

### 2. Pencarian thread

`handleFromPhone` memanggil `FindByPeer(accountID, Chat.ToNonAD())`. Bila tidak
ada dan `RecipientAlt` terisi, dicoba lagi dengan `RecipientAlt.ToNonAD()`,
karena thread bisa tersimpan dengan alamat yang berbeda bentuk dari yang dipakai
HP. Bila tetap tidak ada, pesan dilewati dengan log tingkat debug.

`FindOrCreate` sengaja tidak dipakai: fungsi itu membuat thread baru dan membuka
ulang thread yang sudah ditutup.

### 3. Penyimpanan: `CSMessageService.SaveFromPhone`

File baru `internal/services/cs_message_phone.go`, karena
`cs_message_service.go` sudah 292 baris. Masukannya `InboundMessage` yang sama
dengan `SaveInbound` (pesan yang tiba dari WhatsApp), dan jawabannya juga sama:
baris tersimpan dan apakah baris itu baru.

Satu transaksi dengan kerangka yang sama dengan `SaveInbound`:

1. Bila `wa_message_id` sudah ada, kembalikan baris itu sebagai bukan baru.
   WhatsApp mengirim ulang event setiap koneksi tersambung kembali.
2. Bentuk baris dengan helper yang sama dengan `inboundRow`, hanya arah dan
   statusnya berbeda: `direction = out`, `status = sent`, `sender_user_id` kosong,
   `wa_timestamp` dari `evt.Info.Timestamp`. Media, kutipan, dan pratinjau link
   diisi dengan cara yang sama. Tanda terima yang WhatsApp kirim ke perangkat ini
   memajukan statusnya lewat `ApplyReceipt` yang sudah ada.
3. Perbarui thread (komponen 4).

### 4. Pembaruan thread

Satu UPDATE bersyarat di transaksi yang sama:

```sql
UPDATE cs_conversations
SET last_message_at = :at, last_message_direction = 'out', unread_count = 0
WHERE id = :conversation_id AND last_message_at <= :at
```

Tidak ada baris yang terubah berarti pelanggan sudah menulis lagi setelah balasan
HP itu, dan itu bukan kesalahan: thread tetap menunggu dan badge-nya tetap. Status
dan pemegang thread tidak disentuh.

### 5. Pemberitahuan

Setelah commit, `EventMessage` diumumkan persis seperti pesan masuk sehingga
browser yang membuka thread memuat ulang. Listener push menerima event itu dan
berhenti di pemeriksaan arah pesan.

### 6. Model

Komentar `CSMessage.SenderUserID` mencatat aturannya: baris `out` tanpa pengirim
diketik di HP, karena setiap balasan dari TikMan membawa CS penulisnya (lihat
`Queue`). Bagian 2 mengandalkan aturan ini. Tidak ada kolom atau migrasi baru.

### 7. Frontend

`MessageThread.tsx` menampilkan "dari HP" di baris jam untuk pesan `out` yang
tidak punya `senderUserId`. Pesan `out` lain tidak berubah.

## Penanganan kegagalan

Sama dengan pesan masuk, karena jalurnya dipakai bersama:

- Media gagal diunduh: pesan tetap disimpan dengan body berakhiran
  `[media gagal diunduh]`.
- `SaveFromPhone` gagal: file media yang sudah diunduh dihapus, error dikembalikan
  dan dicatat oleh `client.go`.
- Pesan duplikat: file media dihapus dan tidak ada pengumuman.
- `FindByPeer` gagal: error dikembalikan dan dicatat, tidak ada yang tersimpan.

## Pengujian

TDD: setiap tes ditulis dan gagal sebelum kodenya ada.

`internal/wa` (mengikuti `inbound_quote_test.go`):

- pesan dari HP tersimpan di thread pelanggan sebagai `out` tanpa pengirim;
- thread ditemukan lewat `RecipientAlt` bila `Chat` berupa LID dan thread
  tersimpan dengan nomor;
- tanpa thread, tidak ada baris tersimpan;
- pesan dari HP ke grup atau channel tetap diabaikan;
- event yang sama dua kali menghasilkan satu baris.

`internal/services`:

- balasan HP mengeluarkan thread dari daftar "Belum dibalas" dan mengosongkan
  `unread_count` (mengikuti `cs_awaiting_reply_test.go`);
- balasan HP yang lebih lama dari pesan pelanggan terakhir tidak mengubah arah
  pesan terakhir maupun badge;
- thread yang tertutup tetap tertutup dan `assigned_user_id` tidak berubah;
- varian Postgres untuk syarat `last_message_at <= :at`
  (`setupPostgresTestDB`, `TEST_POSTGRES_DSN`), karena SQLite membandingkan
  timestamp sebagai teks.

Frontend: label "dari HP" muncul untuk pesan `out` tanpa pengirim dan tidak muncul
untuk balasan dari TikMan.

## Deploy

Perilaku yang berubah ada di `wa` dan `frontend`. `api` tidak perlu dibuat ulang
dan tidak ada migrasi.

- `wa` dibuat ulang sekali, di jam sepi. Membuat ulang `wa` memutus sesi WhatsApp
  sebentar, dan sambung-putus berulang adalah yang membuat nomor tidak resmi
  diblokir.
- `frontend` dibuat ulang tersendiri (`--no-deps`).
- Verifikasi:
  - binary `wa` yang berjalan memuat string baru dari `inbound_phone.go`;
  - bundle frontend memuat "dari HP";
  - satu balasan dari HP ke thread uji muncul dengan label dan keluar dari
    "Belum dibalas".

## Di luar cakupan

- Membuat thread dari chat yang dimulai dari HP.
- Balasan dari HP sebelum fitur ini berjalan; WhatsApp tidak mengirimkannya ulang.
- Mengetahui CS mana yang memegang HP.
- Edit, hapus untuk semua, dan reaksi dari HP. Untuk pesan pelanggan pun hal ini
  sekarang diabaikan.
- Pencatatan waktu tunggu dan laporannya (bagian 2).
