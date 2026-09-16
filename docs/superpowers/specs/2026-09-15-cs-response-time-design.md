# Waktu Balas CS — Desain

**Tanggal:** 2026-09-15
**Status:** Disetujui untuk perencanaan
**Bagian dari:** pengukuran waktu balas CS, bagian 2 dari 2.
**Bergantung pada:** `2026-09-15-cs-phone-replies-design.md` (balasan dari HP
tersimpan sebagai pesan `out` tanpa pengirim).

## Kenapa

Admin ingin tolok ukur kecepatan CS membalas pelanggan. Angkanya dipakai untuk
empat hal sekaligus: evaluasi berkala per CS, pemantauan harian tim, dasar
penilaian kinerja, dan dilihat sendiri oleh tiap CS. Karena menyangkut penilaian
orang, angkanya harus bisa dipertanggungjawabkan: tiap angka bisa dibuka sampai ke
thread-nya, dan tidak bisa diubah dengan menghapus pesan.

Spec inbox CS (`2026-09-02-cs-whatsapp-shared-inbox-design.md`, bagian 17)
menaruh laporan dan statistik kinerja CS di luar cakupan. Ini kelanjutannya.

## Temuan dari data production

Dasar aturan di bawah adalah analisis sekali pakai atas data production
4–15 September (2.977 pesan, 1.043 giliran menunggu). Hanya waktu dan metadata
yang dibaca, tanpa isi pesan.

- **Rata-rata menyesatkan.** Median waktu balas 4,2 menit, sedangkan rata-ratanya
  414 menit. Laporan memakai median dan persentil, bukan rata-rata.
- **Gangguan sistem.** Pesan pelanggan 9–12 September baru tiba di TikMan sampai
  70 jam setelah dikirim, sebagian besar sekaligus pada 13 September. Selama
  itu CS tidak bisa membalas pesan yang belum sampai. Selisih
  `created_at − wa_timestamp` pada pesan masuk memperlihatkannya.
- **Sesi yang tergabung.** Pelanggan yang menulis "ok makasih", lalu bertanya hal
  baru beberapa hari kemudian, terhitung menunggu sejak "ok makasih". Ini
  menyumbang 22 dari 70 balasan yang lebih dari 12 jam.
- **Tumpukan chat malam.** Sekitar 1 dari 10 chat dimulai antara 23.00 dan
  08.00, lalu dibalas CS yang mulai bekerja pagi.
- **Jeda antar balasan satu CS.** Dari 924 jeda, hampir semuanya di bawah 2 jam
  (jeda saat bekerja) atau di atas 6 jam (pergantian shift). Hanya 3 yang jatuh
  di antaranya.
- **Penghapusan pesan** masih jarang (4 pesan satuan dan 6 pesan dari 2 thread
  selama periode itu), tapi CS bisa menghapus pesan dan mengosongkan thread
  secara permanen.

## Keputusan

- **Tanpa data shift.** Jadwal shift CS tidak tertulis. CS dinilai dari balasan
  yang dia kirim, bukan dari siapa yang seharusnya bertugas.
- **Layanan 24 jam.** Angka tim dihitung terus, termasuk malam hari.
- **Angka per CS dihitung sejak CS itu mulai aktif**, supaya CS pagi tidak
  menanggung tumpukan chat malam.
- **Target 15 menit**, sebagai konstanta di kode dan dikirim dalam respons
  laporan. `SettingService` menyimpan kredensial (nilai disamarkan, tanpa
  validasi angka), jadi tidak dipakai untuk ini.
- **Fakta dicatat saat kejadian, tafsiran saat laporan dibuka.** Kapan giliran
  mulai dan selesai, bagaimana selesainya, dan siapa pelakunya tidak pernah
  berubah. Target, ambang 3 jam, dan ambang tertunda sistem diterapkan saat
  laporan dibuka. Aturan 24 jam adalah pengecualian: aturan itu menentukan di
  mana satu giliran berakhir dan giliran berikutnya dimulai, jadi diterapkan
  saat pencatatan.
- **Dimulai dari nol.** Riwayat sebelum deploy tidak diisi ulang.
- **Tanpa pesan otomatis WhatsApp Business**, dikonfirmasi user pada
  2026-09-16. Balasan `phone` dianggap balasan manusia. Kalau pesan sambutan,
  sedang tidak di tempat, atau balasan AI diaktifkan, pesan itu akan tercatat
  sebagai giliran `phone` berdurasi hampir 0 menit dan menaikkan angka tim, jadi
  desain ini harus ditinjau ulang lebih dulu.

## Aturan hitung

### Giliran menunggu

- **Mulai:** saat pesan pelanggan yang belum dibalas tiba di TikMan
  (`cs_messages.created_at`). Pesan pelanggan berikutnya sebelum ada balasan masuk
  ke giliran yang sama.
- **Selesai**, dengan salah satu dari:
  - `replied`: dibalas dari TikMan. Waktunya saat CS menekan Kirim, pelakunya CS
    pengirim.
  - `phone`: dibalas dari HP, tanpa pelaku.
  - `closed`: thread ditutup tanpa balasan. Pelakunya CS yang menutup.
  - `abandoned`: pelanggan diam lebih dari 24 jam (dihitung dari waktu kirim
    WhatsApp, bukan waktu tiba), lalu menulis lagi. Giliran lama berakhir saat
    pesan baru itu tiba, dan giliran baru dimulai dari pesan tersebut.
- **Tertunda sistem:** pesan pertama giliran tiba di TikMan lebih dari 30 menit
  setelah dikirim pelanggan. Giliran seperti ini tampil di daftar, tapi tidak
  masuk angka waktu tim maupun CS.

### Angka tim

Untuk giliran `replied` dan `phone` yang tidak tertunda sistem, waktu tunggu tim
adalah `ended_at − started_at`, 24 jam. Laporan menampilkan jumlah, median,
persentil 90, dan persentase ≤ 15 menit. Ditambah jumlah giliran `closed`,
`abandoned`, dan tertunda sistem, serta jumlah pelanggan yang sedang menunggu
beserta waktu tunggu terlamanya.

### Angka per CS

Untuk giliran `replied` milik CS itu yang tidak tertunda sistem:

```
dihitung = ended_at − max(started_at, awal sesi CS)
awal sesi CS = balasan pertama CS itu setelah ≥ 3 jam tanpa membalas
```

- Balasan pertama setelah jeda terhitung 0 menit.
- "Membalas" berarti giliran `replied` oleh CS tersebut. Pesan susulan di thread
  tanpa giliran terbuka tidak dihitung.
- Sesi yang sudah berjalan sebelum awal periode tetap dikenali: catatan balasan
  CS itu dibaca mundur dari awal periode sampai ditemukan jeda ≥ 3 jam.

Laporan menampilkan jumlah balasan, median, persentil 90, persentase ≤ 15 menit,
dan jumlah thread yang dia tutup tanpa balasan. Kolom terakhir menutup celah
"menutup chat yang sulit alih-alih membalasnya", yang tidak tertangkap bila CS
hanya dinilai dari balasan.

### Periode

- `from` dan `to` berupa tanggal (`YYYY-MM-DD`) dalam WIB, inklusif, menjadi
  rentang `[from 00.00 WIB, to+1 00.00 WIB)`. Rentang paling panjang 366 hari.
- Giliran masuk ke periode, dan ke hari di grafik, menurut `ended_at` dalam WIB.
- Persentil memakai interpolasi linear, sama dengan `percentile_cont` di
  Postgres, supaya angkanya bisa dicocokkan dengan query langsung.

## Pencatatan

### Tabel `cs_waits`

Model `CSWait` (`internal/models/cs_wait.go`) masuk ke daftar AutoMigrate, tanpa
field relasi. Constraint dan indeks ditambahkan di migrasi
`52_cs_waits.sql`, mengikuti pola migrasi 41 dan 49.

| Kolom | Isi |
|---|---|
| `id` | UUID |
| `conversation_id`, `wa_account_id` | Thread dan nomor |
| `started_at` | Pesan pertama giliran tiba di TikMan |
| `customer_sent_at` | Waktu kirim WhatsApp pesan tersebut |
| `last_customer_sent_at` | Waktu kirim pesan pelanggan terbaru dalam giliran |
| `ended_at`, `end_reason` | Kapan dan bagaimana giliran selesai; kosong selama masih terbuka |
| `ended_by` | CS untuk `replied` dan `closed`; kosong untuk `phone` dan `abandoned` |
| `reply_message_id` | Pesan balasan untuk `replied` dan `phone` |
| `created_at` | |

Aturan database:

- satu giliran terbuka per thread: indeks unik parsial pada `conversation_id`
  `WHERE ended_at IS NULL`;
- `end_reason` salah satu dari empat nilai di atas, dan terisi tepat ketika
  `ended_at` terisi;
- pasangan `ended_by` dan `reply_message_id` mengikuti `end_reason`:

  | `end_reason` | `ended_by` | `reply_message_id` |
  |---|---|---|
  | `replied` | wajib | wajib |
  | `phone` | kosong | wajib |
  | `closed` | wajib | kosong |
  | `abandoned` atau masih terbuka | kosong | kosong |

- indeks untuk laporan: `(ended_at)` dan `(ended_by, ended_at)`.

**Tanpa foreign key**, sengaja. Menghapus pesan, mengosongkan thread, dan menghapus
nomor (`CSPurgeService.DeleteAccount` menghapus thread-nya) tidak menyentuh
`cs_waits`. Angka tidak bisa diubah dengan menghapus, dan penghapusan nomor tidak
diblokir seperti yang pernah terjadi pada riwayat broadcast. Menghapus pesan
pelanggan juga tidak mengakhiri giliran, karena pelanggan tetap belum dijawab.

### Titik pencatatan

Semuanya berjalan di dalam transaksi yang sudah ada, di bawah savepoint:

1. **`CSMessageService.SaveInbound` (wa).** Setelah pemeriksaan duplikat, sehingga
   event yang dikirim ulang tidak menyentuh giliran.
   - Ada giliran terbuka dan pelanggan belum diam lebih dari 24 jam: perbarui
     `last_customer_sent_at`, dengan nilai terbesar dipertahankan bila pesan tiba
     tidak berurutan.
   - Ada giliran terbuka tapi pelanggan sudah diam lebih dari 24 jam: tutup
     sebagai `abandoned`, lalu buka giliran baru.
   - Tidak ada giliran terbuka: buka giliran baru.
2. **`CSMessageService.Queue` (api).** Dipakai balasan teks dan media. Tutup giliran
   terbuka sebagai `replied`, dengan `ended_at` waktu balasan dan `ended_by` CS
   pengirim. Tanpa giliran terbuka, tidak ada yang berubah.
3. **`CSMessageService.SaveFromPhone` (wa, bagian 1).** Hanya bila balasan HP
   dikirim tidak lebih awal dari `customer_sent_at` giliran terbuka:
   - tutup sebagai `phone`, dengan `ended_at = max(waktu kirim balasan, started_at)`;
   - bila thread punya pesan pelanggan yang dikirim setelah balasan HP itu, buka
     giliran baru mulai dari pesan pertama tersebut.

   Balasan HP yang lebih tua dari giliran terbuka menjawab giliran sebelumnya, dan
   tidak mengubah apa pun.
4. **`CSConversationService.Close` (api).** Tutup sebagai `closed`, dengan
   `ended_by` CS yang menutup. `Close` menerima ID CS tersebut dan menjalankan
   perubahan status serta pencatatan giliran dalam satu transaksi.

**Aman saat bersamaan.** Setiap langkah berupa UPDATE bersyarat
(`WHERE conversation_id = ? AND ended_at IS NULL`) dan INSERT yang dijaga indeks
unik parsial, bukan baca-lalu-tulis. Pada READ COMMITTED, Postgres mengevaluasi
ulang syarat UPDATE setelah transaksi yang bersaing commit. Pesan pelanggan yang
tersimpan setelah balasan pun membuka giliran baru, bukan hilang ke dalam giliran
yang sudah ditutup.

**Pesan tetap tersimpan bila pencatatan gagal.** Kalau pencatatan giliran gagal,
savepoint-nya dibatalkan dan kegagalannya dicatat di log oleh pemanggil, sedangkan
pesan pelanggan, balasan CS, atau penutupan thread tetap tersimpan. Menyimpan pesan
adalah tugas utama inbox; angka tidak boleh menjadi alasan pesan hilang atau
balasan gagal terkirim.

**Batas yang disengaja.** Pelanggan yang hanya mengirim stiker, lokasi, kontak,
atau polling tidak membuka giliran, karena inbox tidak menyimpan pesan itu.

## Laporan

### API

Dua rute baru di grup `/api/v1/cs` yang sudah membatasi peran ke admin, CS, dan
teknisi:

- **`GET /cs/performance/summary?from=&to=`** menjawab:
  - `target_minutes`;
  - angka tim;
  - baris per CS;
  - angka per hari untuk grafik (jumlah, median, persentase ≤ target);
  - `waiting`: jumlah giliran terbuka yang thread-nya masih ada, dan waktu tunggu
    terlama.
- **`GET /cs/performance/waits?from=&to=&user_id=&limit=&offset=`** menjawab
  daftar giliran yang selesai dalam periode, per halaman dengan pola
  `limit`/`offset` yang sudah dipakai rute lain (`paginationParams` yang sudah
  ada: default 20, maksimum 100). Tiap giliran memuat:
  - thread, nama dan nomor pelanggan, atau tanda bahwa thread sudah dihapus;
  - `started_at`, `customer_sent_at`, `ended_at`, `end_reason`, dan pelakunya;
  - menit untuk tim, menit yang dihitung untuk CS, dan tanda tertunda sistem.

  Tanpa `user_id`, admin mendapat semua giliran.

**Siapa melihat apa**, berdasarkan `middleware.GetUserRole`:

- **Admin** melihat angka tim, semua baris CS, dan daftar giliran siapa pun.
- **CS dan teknisi** melihat angka tim dan baris miliknya sendiri. Di `waits`,
  `user_id` selalu dipaksa menjadi dirinya sendiri, dan hanya giliran yang dia
  balas atau tutup yang tampil.
- **Viewer** ditolak oleh grup rute dengan 403, seperti rute CS lainnya.

Tanggal yang tidak valid, `from` setelah `to`, atau rentang lebih dari 366 hari
dijawab 400.

Perhitungan angka (median, persentil, persentase, sesi CS, batas hari WIB) berupa
fungsi murni di service, terpisah dari query, supaya bisa diuji tanpa database.

### Tampilan

Halaman **"Kinerja CS"** di `/cs/performance`. Menunya tampil untuk semua peran
yang boleh membuka CS Inbox (`buildNavigationRoutes`).

```
Kinerja CS                    [Hari ini | 7 hari | Bulan ini | Bulan lalu | Pilih tanggal]
┌─────────┬─────────┬────────────┬──────────────┐
│ Giliran │ Median  │ ≤ 15 menit │ 90% tercepat │   ← angka tim, 24 jam
└─────────┴─────────┴────────────┴──────────────┘
Ditutup tanpa balasan · Ditinggal · Tertunda sistem
Sedang menunggu: N pelanggan, terlama X menit  → tab "Belum dibalas"

Per CS   Balasan   Median   ≤ 15 menit   90% tercepat   Ditutup tanpa balasan
CS A        ...      ...        ...          ...                ...          ›
[grafik harian: % dibalas ≤ 15 menit]
```

- Pemilih periode memakai `RangePicker` seperti `GraphsPage`, dan grafiknya
  memakai `recharts` yang sudah terpasang. Tidak ada dependensi baru.
- Klik baris CS membuka daftar gilirannya. Klik giliran membuka thread di inbox
  (`/cs?conversation=<id>`). Giliran yang thread-nya sudah dihapus tetap tampil
  dengan keterangan itu.
- "Sedang menunggu" menautkan ke `/cs?view=belum-dibalas`, bukan membuat daftar
  kedua. Jumlahnya bisa berbeda dari tab itu bila ada pesan pelanggan yang
  dihapus sebelum dibalas, karena gilirannya tetap terbuka.
- CS dan teknisi melihat halaman yang sama, dengan tabel per CS yang hanya berisi
  barisnya sendiri.

## Pengujian

TDD, dengan pola tes yang sudah ada.

**Pencatatan** (`internal/services`):

- pesan pertama membuka giliran dengan waktu tiba dan waktu kirim; pesan
  berikutnya hanya memperpanjang;
- pelanggan diam lebih dari 24 jam: giliran lama `abandoned`, giliran baru terbuka;
- `Queue` menutup sebagai `replied` atas nama CS; balasan tanpa giliran terbuka
  tidak mengubah apa pun;
- `SaveFromPhone` menutup sebagai `phone`; balasan HP yang telat tiba membuka
  giliran baru dari pesan pelanggan sesudahnya; balasan HP yang lebih tua dari
  giliran terbuka tidak mengubahnya;
- `Close` menutup sebagai `closed` dengan `ended_by`;
- event duplikat tidak menyentuh giliran;
- kegagalan pencatatan giliran tidak menggagalkan penyimpanan pesan;
- menghapus pesan, mengosongkan thread, dan menghapus nomor tidak menghapus
  giliran, dan penghapusan nomor tetap berhasil.

**Postgres** (`TEST_POSTGRES_DSN`):

- pesan pelanggan dan balasan yang bersamaan tidak menghilangkan giliran (pola
  `TestConcurrentFirstRepliesLeaveOneHolderOnPostgres`);
- database menolak giliran terbuka kedua dan kombinasi kolom yang salah (pola tes
  constraint di `internal/database/migrations_postgres_test.go`);
- migrasi 52 ikut tercakup `TestEveryMigrationAppliesToAFreshSchema`.

**Perhitungan** (table-driven, fungsi murni):

- median, persentil 90, persentase ≤ 15 menit, dengan interpolasi linear;
- tertunda sistem tidak dihitung; `phone` masuk angka tim tapi tidak masuk angka
  CS; `closed` dan `abandoned` hanya dihitung jumlahnya;
- balasan pertama setelah jeda ≥ 3 jam terhitung 0 menit; sesi yang dimulai
  sebelum awal periode tetap dikenali;
- batas hari WIB: giliran yang selesai 23.59 dan 00.01 WIB masuk hari berbeda.

**Handler:**

- CS dan teknisi hanya mendapat barisnya sendiri dan tidak bisa membuka giliran CS
  lain lewat `user_id`;
- admin mendapat semuanya;
- viewer 403;
- tanggal tidak valid, urutan terbalik, dan rentang lebih dari 366 hari dijawab
  400.

**Frontend:**

- menu "Kinerja CS" tampil untuk admin, CS, dan teknisi, tidak untuk viewer (pola
  `navigationRoutes.test.tsx`);
- preset periode mengirim tanggal WIB yang benar;
- klik baris CS membuka daftar gilirannya;
- tautan "Sedang menunggu" menuju tab "Belum dibalas";
- giliran dengan thread terhapus tetap tampil.

## Deploy

1. **`api`:** AutoMigrate membuat `cs_waits`, migrasi 52 menambahkan constraint dan
   indeks, rute laporan aktif, dan `Queue`/`Close` mulai mencatat.
2. **`wa`:** `SaveInbound` dan `SaveFromPhone` mulai mencatat. Harus setelah `api`
   karena menulis ke tabel yang dibuat `api`. Pesan yang masuk di antara dua
   langkah ini tidak membuka giliran.
3. **`frontend`:** dibuat ulang tersendiri (`--no-deps`).

Bagian 1 juga mengubah `wa`. Bila bagian 1 belum di-deploy, keduanya sebaiknya
di-deploy bersama, supaya sesi WhatsApp hanya terputus sekali.

Verifikasi:

- rute `/api/v1/cs/performance/summary` menjawab 401 tanpa login dari dalam
  container `api`;
- indeks unik parsial `cs_waits` ada di `pg_indexes`;
- binary `wa` yang berjalan memuat string baru dari kode pencatatan giliran;
- bundle frontend memuat "Kinerja CS";
- satu pesan uji dari pelanggan membuka giliran, dan balasan dari TikMan menutupnya
  sebagai `replied`.

## Di luar cakupan

- Data shift, jadwal, atau status bertugas.
- Mengubah target dari UI (perlu jenis pengaturan non-rahasia di Settings).
- Mengisi giliran dari riwayat sebelum deploy.
- Ekspor CSV atau Excel, perbandingan antarperiode, dan notifikasi saat target
  tidak tercapai.
- Pengecualian periode gangguan secara manual. Pesan yang telat tiba sudah
  dikecualikan otomatis lewat tanda tertunda sistem, tapi gangguan yang hanya
  membuat halaman TikMan tidak bisa dibuka tidak terdeteksi.
