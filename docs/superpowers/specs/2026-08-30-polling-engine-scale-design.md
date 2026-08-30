# Mesin Polling Berskala — Desain

**Tanggal:** 2026-08-30
**Status:** A1 dan A2 selesai dan terpasang. A3 menunggu prasyarat trap.
**Cakupan:** Proyek A dari tiga proyek. Proyek B (paginasi daftar ONT di server) dan
Proyek C (indeks skala besar) punya spec sendiri nanti.

## Tujuan

TikMan harus memantau **ratusan ribu ONT** di **puluhan chassis OLT**, dengan
status yang tidak lebih basi dari **satu menit**.

Hari ini instalasi berisi 930 ONT di 3 OLT, dan satu siklus polling makan 84 detik
sambil melewati dua dari tiga OLT. Bukan soal penyetelan: ada empat plafon
struktural yang harus dicabut.

## Keadaan Sekarang, Terukur

| Temuan | Bukti |
|---|---|
| SNMP memakai GETNEXT, satu round-trip per nilai | Semua pemanggilan `client.Walk()`; gosnmp v1.44.0 mendokumentasikan `Walk` sebagai GETNEXT dan `BulkWalk` sebagai GETBULK |
| Worker hanya mem-poll 1000 ONT pertama | `cmd/worker/main.go`: `ontService.List(nil, nil, 1000, 0)` |
| Satu INSERT dan satu baris log per ONT | `MetricsService.StoreMetrics` |
| Discovery jalan tiap siklus untuk semua OLT | `go oltService.AutoDiscoverONTMetrics(&olts[i])` di tiap `collectMetrics` |
| RTT ke OLT lewat tunnel | 7,3 ms rata-rata (ping 5 paket ke Cariu) |
| Penyimpanan | TimescaleDB, retensi dan continuous aggregate 5 menit/jam sudah aktif |

Klien SNMP sudah `Version2c`, jadi GETBULK tersedia dan hanya tidak dipakai.
`MaxRepetitions` tidak pernah diset di mana pun.

## Keputusan yang Diambil

1. **Status disegarkan ~1 menit; metrik optik dan trafik ~10 menit.** Daya optik
   tidak berubah tiap menit — yang berubah tiap menit adalah status. Pemisahan ini
   yang menurunkan beban tulis dari ~5.000 menjadi ~500 baris/detik.
2. **SNMP trap dipakai sebagai pemberitahuan cepat**, dengan poll status tetap
   jalan sebagai jaring pengaman. Trap lewat UDP dan bisa hilang tanpa jejak, jadi
   trap tidak pernah jadi satu-satunya sumber kebenaran.
3. **Antrean pekerjaan memakai Postgres `FOR UPDATE SKIP LOCKED`, bukan Redis.**
   Daftar OLT sudah hidup di Postgres; antrean terpisah menciptakan sumber
   kebenaran kedua yang, bila melenceng, membuat sebuah OLT tidak di-poll siapa pun
   tanpa ada yang tahu. Daftar Redis juga tidak punya ack: worker yang mati setelah
   `BRPOP` menghilangkan pekerjaannya diam-diam. Redis baru berbayar bila worker
   tersebar di beberapa mesin; `SKIP LOCKED` tetap bekerja bila itu terjadi.
4. **Deploy: satu VPS, beberapa proses worker.** Tidak ada proses penjadwal
   terpisah dan tidak ada pemilihan pemimpin — klaim pekerjaan itu sendiri yang
   jadi koordinasinya.

## Arsitektur

| Proses | Jumlah | Tugas |
|---|---|---|
| `cmd/api` | 1 | Tidak berubah |
| `cmd/worker` | N (mulai 2–4) | Klaim satu pekerjaan OLT yang jatuh tempo, baca SNMP, tulis batch, jadwalkan berikutnya |
| `cmd/trapd` | 1 | Tahap A3. Dengar UDP 162, terjemahkan trap jadi perubahan status |

Klaim bersifat per-OLT, sehingga satu OLT hanya disentuh satu worker pada satu
waktu. Ini bukan efek samping melainkan syarat: agen SNMP ZTE tidak tahan dilayani
beberapa pembaca sekaligus.

## Model Pekerjaan

Tabel `olt_poll_jobs`, primary key `(olt_id, kind)`:

| Kolom | Keterangan |
|---|---|
| `olt_id` | FK ke `olts` |
| `kind` | `status` \| `metrics` \| `discovery` |
| `due_at` | Kapan pekerjaan ini boleh diambil |
| `locked_by` | Identitas worker pemegang, null bila bebas |
| `locked_at` | Waktu klaim; lease kedaluwarsa dihitung dari sini |
| `last_run_at`, `last_error`, `consecutive_failures` | Untuk backoff dan diagnosis |

Interval bawaan: `status` 60 detik, `metrics` 600 detik, `discovery` 3600 detik.
Lease: `status` 2 menit, `metrics` 10 menit, `discovery` 30 menit.

Pengambilan:

```sql
UPDATE olt_poll_jobs SET locked_by = $1, locked_at = now()
WHERE (olt_id, kind) IN (
  SELECT olt_id, kind FROM olt_poll_jobs
  WHERE due_at <= now()
    AND (locked_at IS NULL OR locked_at < now() - $2::interval)
  ORDER BY due_at
  FOR UPDATE SKIP LOCKED
  LIMIT 1)
RETURNING olt_id, kind;
```

Selesai: `due_at = now() + interval`, `locked_by = NULL`, `consecutive_failures = 0`.
Gagal: `consecutive_failures + 1` dan `due_at = now() + interval * 2^min(failures, 5)`,
sehingga OLT yang mati tidak dicoba tiap menit selamanya.

Baris dibuat saat OLT dibuat dan dihapus bersama OLT-nya. Migrasi awal mengisi
baris untuk setiap OLT yang sudah ada.

OLT di balik tunnel yang turun mendorong `due_at` maju alih-alih berputar gagal;
pengecekan `oltsBehindDownTunnel` yang sudah ada tetap dipakai.

## Jalur Baca SNMP

`newSNMPClientWithContext` mendapat `MaxRepetitions`, dapat diatur lewat
`SNMP_MAX_REPETITIONS` dengan default **10**, ditetapkan dari pengukuran (lihat
"Hasil Pengukuran" di bawah). Semua pemanggilan tabel pindah dari `client.Walk`
ke `client.BulkWalk`.

Karena sebagian agen menolak GETBULK besar, pemanggilan dibungkus satu helper:
coba GETBULK, dan bila agen menjawab `tooBig` atau sejenisnya, turun ke GETNEXT
sekali untuk OLT itu **dan catat di log**. Penurunan senyap adalah pola kegagalan
yang berulang kali menyembunyikan masalah di sistem ini; kejadiannya harus
terdengar.

Tiap tingkatan hanya membaca yang diperlukannya:

- `status` — tabel phase-state saja
- `metrics` — rx power, tx power, jarak, dan tabel laju trafik
- `discovery` — tabel inventaris

## Jalur Tulis

- Metrik ditulis sebagai multi-row `INSERT` per ~1000 baris. Batas 65535 parameter
  Postgres dibagi 13 kolom memberi plafon ~5000; 1000 dipilih agar aman.
- Perubahan status ditulis sebagai satu `UPDATE` batch, bukan satu per ONT.
- Tingkatan `status` tidak menulis baris `ont_metrics` sama sekali. Ia hanya
  menulis bila status berubah, ke `onts` dan `ont_events`.
- Log per-ONT dihapus dan diganti ringkasan per OLT. Pada 300 ribu ONT, log lama
  menghasilkan 300 ribu baris tiap siklus.

Anggaran penyimpanan dengan pemisahan tingkatan: metrik tiap 10 menit ≈ 500
baris/detik ≈ 43 juta baris/hari ≈ 10,8 GB mentah, turun ke sekitar 1 GB/hari
dengan kompresi TimescaleDB. Kompresi diaktifkan untuk chunk berusia di atas satu
jam; riwayat panjang mengandalkan agregat 5 menit dan per jam yang sudah ada.

## Penerima Trap (A3)

`cmd/trapd` memakai `gosnmp.TrapListener` pada UDP 162. Trap perubahan status ONU
diterjemahkan menjadi (OLT, slot, port, ONU) dan status baru, lalu ditulis lewat
jalur yang sama dengan poller sehingga tidak ada dua cara menulis status.

Prasyarat yang harus diverifikasi sebelum tahap ini dimulai, bukan diasumsikan:
OLT dikonfigurasi mengirim trap ke alamat WireGuard server, dan UDP 162 lolos
melewati tunnel.

Trap tidak pernah menghapus baris atau menyimpulkan ketiadaan; ia hanya melaporkan
perubahan. Poll status satu menit tetap jadi kebenaran dan mendamaikan selisihnya.

## Kegagalan

Aturan berikut sudah pernah jadi bug di repo ini dan harus tetap berlaku:

- Poll yang gagal atau dilewati **tidak** menandai ONT offline.
- Pembacaan tabel yang tidak tuntas **tidak** memicu prune ONT.
- Kegagalan registrasi dicatat, tidak dibuang.

Worker yang mati di tengah pekerjaan melepaskan lease-nya lewat kedaluwarsa, dan
worker lain memungut pekerjaan itu. Tidak ada proses yang kematiannya
menghentikan seluruh pemantauan.

## Pengujian

- Logika klaim `SKIP LOCKED` tidak dapat diuji di SQLite. Job tes backend di CI
  mendapat service `postgres:15`, dan tes antrean berjalan terhadap Postgres
  sungguhan lewat `TEST_POSTGRES_DSN`.
- Bila DSN tidak ada, tes **gagal keras saat variabel `CI` terpasang** dan hanya
  melewatkan diri di mesin lokal dengan pesan jelas. Tes yang diam-diam tidak
  pernah berjalan adalah kegagalan yang sedang kita hindari, bukan yang kita buat.
- Tes bersamaan: dua worker mengklaim serentak harus menghasilkan dua pekerjaan
  berbeda, tidak pernah pekerjaan yang sama.
- Tes lease: pekerjaan yang pemegangnya menghilang harus dapat dipungut kembali
  setelah lease kedaluwarsa, dan tidak sebelum itu.
- Encoder dan pemetaan OID diuji tanpa perangkat, seperti pola tes SNMP yang sudah
  ada di repo.

## Urutan Pengerjaan

**A1 — Kebenaran dan kecepatan.** Ukur GETBULK terhadap OLT sungguhan lebih dulu,
lalu cabut plafon 1000 ONT, pindah ke `BulkWalk`, tulis batch, dan buang log
per-ONT. Tanpa infrastruktur baru. Setelah tahap ini kita punya angka nyata:
berapa ONT per menit yang sanggup dibaca satu worker lewat tunnel ini.

**A2 — Antrean dan banyak worker.** Tabel pekerjaan, klaim `SKIP LOCKED`, tingkatan
jadwal, dan worker yang dapat ditambah jumlahnya. Angka dari A1 menentukan berapa
worker yang benar-benar diperlukan.

**A3 — Penerima trap.** Setelah prasyarat trap terverifikasi.

## Hasil Pengukuran

Diukur dengan `cmd/snmpbench` terhadap Cariu (655 ONU, tabel phase-state), dua
kali jalan pada beban berbeda:

| Mode | Nilai | Jalan 1 | Jalan 2 |
|---|---|---|---|
| GETNEXT | 655 | 11,19 s | 20,30 s |
| GETBULK@5 | 655 | — | 5,56 s |
| **GETBULK@10** | 655 | **3,88 s** | **4,70 s** |
| GETBULK@15 | 655 | — | 6,96 s |
| GETBULK@25 | 655 | 4,94 s | — |
| GETBULK@50 | 655 | 5,36 s | — |
| GETBULK@100 | 655 | 5,15 s | — |

Tiga kesimpulan, dan yang kedua membatalkan asumsi awal desain ini:

1. **Agen ZTE melayani GETBULK dengan benar.** Setiap mode mengembalikan 655 nilai
   yang sama persis — tidak ada pemotongan senyap. Risiko utama spec ini gugur.
2. **`MaxRepetitions` optimal adalah 10, bukan 25.** Di atas ~10 justru melambat.
   Aritmetikanya menjelaskan kenapa: pada @10 ada ~66 permintaan × 7,3 ms RTT =
   0,5 detik di jaringan, dari total 3,9 detik. Sisanya waktu agen menyusun
   jawaban. **Lehernya adalah CPU agen SNMP di OLT, bukan jaringan dan bukan kode
   kita.** Permintaan yang lebih besar membuat agen bekerja lebih lama per
   permintaan daripada penghematan round-trip yang didapat.
3. **Percepatannya 3–4×, bukan 25×.** Anggaran waktu di desain ini harus dihitung
   dari ~140 nilai/detik per agen, bukan dari RTT.

### Konsekuensi untuk target 1 menit

Pada ~140 nilai/detik, satu chassis berisi 10 ribu ONU memerlukan sekitar **72
detik** hanya untuk satu tabel status. Artinya **polling penuh tiap menit tidak
tercapai untuk chassis sebesar itu**, berapa pun jumlah worker yang ditambahkan,
karena batasnya ada di agen OLT-nya sendiri.

Ini menggeser peran trap dari pelengkap menjadi **inti**: trap adalah satu-satunya
jalan mengetahui perubahan status di bawah satu menit pada skala ini, dengan poll
status berkala sebagai pendamai. Interval poll status di A2 ditetapkan dari
kapasitas agen tiap chassis, bukan dari satu angka global.

Belum diukur, dan diperlukan sebelum A2: apakah agen melayani dua walk bersamaan
lebih cepat secara total, yang menentukan apakah membagi walk per kartu ke
beberapa worker ada gunanya, atau justru memperlambat.

## Hasil A1

Setelah GETBULK, plafon 1000 dicabut, indeks posisi, dan tulis batch, satu
siklus diukur per fase:

| OLT | ONT | SNMP | sync | lintasan ONT | total |
|---|---|---|---|---|---|
| Cariu | 651 | 41,3 s | 28,9 s | 2,7 s | 72,9 s |
| Depok | 199 | 38–44 s | 36–41 s | 0,8 s | 75–85 s |
| Bekasi | 80 | 3,1 s | 3,4 s | 0,4 s | 6,8 s |

Tiga hal yang hanya terlihat karena diukur per fase:

1. **Kerja per-ONT sudah bukan biaya.** Seluruh lintasan ONT — pencocokan,
   perubahan status, event, dan tulis metrik batch — memakan 0,4 sampai 2,7
   detik. Batching dan indeks posisi tetap diperlukan pada skala ratusan ribu,
   tetapi tidak ada lagi yang bisa diperas dari sisi ini sekarang.
2. **Chassis yang sama di-walk dua kali tiap siklus.** `readOLT` membaca status,
   metrik, dan laju; lalu `syncOntsWithDiscovery` menjalankan walk discovery
   penuh terhadap chassis yang sama. Itulah sebabnya `snmp` dan `sync` hampir
   sama besar. Menjadwalkan discovery per jam alih-alih tiap siklus menghapus
   sekitar separuh waktu siklus, dan itu satu-satunya penghematan terbesar yang
   tersisa.
3. **Waktu tidak sebanding dengan jumlah ONT.** Depok dengan 199 ONT memakan
   waktu sama dengan Cariu yang 651. Yang menentukan adalah berapa tabel yang
   di-walk dan secepat apa agen chassis itu. Penjadwalan di A2 karena itu harus
   berbasis kapasitas terukur tiap chassis, bukan jumlah pelanggannya.

Konsekuensi untuk A2: pemisahan tingkatan bukan sekadar penghematan tulis
seperti yang diduga saat desain ditulis, melainkan penghematan **walk**. Tingkat
`status` membaca satu tabel; `metrics` dan `discovery` berjalan jauh lebih
jarang.

### Masih terbuka

`updateOntFields` menulis satu UPDATE per ONT ke tabel `onts`. Pada 930 ONT
biayanya tenggelam di dalam 2,7 detik lintasan ONT dan belum layak diubah; pada
ratusan ribu ia menjadi ratusan ribu UPDATE per siklus dan harus ikut dibatch.

## Hasil A2

Durasi tiap pekerjaan utuh setelah antrean bertingkat dipasang:

| OLT | ONT | status | metrics | discovery |
|---|---|---|---|---|
| Bekasi | 80 | 0,42 s | 3,4 s | 16,3 s |
| Cariu | 651 | 3,4 s | 38,1 s | — |
| Depok | 199 | 0,53 s | 16,8 s | 95,9 s |

Sebelum A1 dan A2, setiap OLT membayar 73–85 detik **tiap menit**, karena satu
siklus membaca semuanya dan men-discovery ulang semuanya. Yang dibayar tiap
menit sekarang hanya tingkat status: 0,4 sampai 3,4 detik.

Cariu membaca 651 ONT dalam 3,4 detik, atau sekitar **190 ONT per detik**.
Diekstrapolasi, satu chassis berisi 10 ribu ONU memerlukan sekitar **52 detik**
untuk satu putaran status — muat di dalam satu menit, dengan sedikit sisa. Untuk
300 ribu ONT tersebar di 30 chassis, seluruh putaran tetap sekitar 52 detik
asalkan chassis-chassis itu dibaca bersamaan, karena batasnya ada di agen tiap
chassis dan bukan pada worker-nya. Itulah jumlah worker yang diperlukan: cukup
untuk menjalankan chassis secara paralel, bukan untuk mempercepat satu chassis.

Terlihat langsung pada satu worker: pekerjaan discovery Depok yang 96 detik
menahan antrean, dan discovery Cariu terlambat tujuh menit. Menambah worker
adalah jawabannya, dan tidak memerlukan konfigurasi apa pun selain menjalankan
prosesnya.

## Risiko

**Pengukuran RTT 7,3 ms dan ~140 nilai/detik berasal dari satu site.** Site lain
bisa berbeda jauh, dan anggaran per OLT harus dihitung dari pengukuran
masing-masing. `cmd/snmpbench` ada untuk itu.

**Kecepatan agen berubah menurut bebannya.** GETNEXT terukur 11,2 detik dan 20,3
detik pada dua jalan berbeda untuk tabel yang sama. Penjadwalan harus menganggap
kapasitas agen sebagai perkiraan, bukan konstanta.

## Proyek B dan C — Selesai

**B (paginasi dan pencarian di server).** Daftar ONT dan halaman grafik mengirim
filternya dan menerima satu halaman. Kartu, port, dan pencarian serial atau nama
dikerjakan database; jumlah yang menggerakkan pager berasal dari server, bukan
dari panjang array yang datang. `matchesOntFilters`, `filterOntsByQuery`, dan
`describeOntCoverage` ikut terhapus — yang terakhir hanya ada untuk meminta maaf
atas plafon yang sudah tidak ada.

**C (indeks).** Diambil dari `EXPLAIN` terhadap tabel sungguhan, bukan tebakan:

| Query | Sebelum | Sesudah |
|---|---|---|
| Paging worker per OLT | scan seluruh primary key | `idx_onts_olt_id_id` |
| Urutan posisi di daftar | sort seluruh tabel tiap halaman | `idx_onts_position_order`, tanpa sort |
| Sinyal terlemah | seq scan + top-N sort | indeks parsial, membaca tepat 5 baris |
| Pencarian substring | tak terlayani btree | indeks trigram GIN |

## Di Luar Cakupan

Paginasi daftar ONT di server, indeks untuk query skala besar, dan sharding worker
lintas mesin. Dua yang pertama adalah Proyek B dan C. Yang ketiga tidak diperlukan
selama deploy masih satu VPS, dan `SKIP LOCKED` tidak menghalanginya bila nanti
diperlukan.
