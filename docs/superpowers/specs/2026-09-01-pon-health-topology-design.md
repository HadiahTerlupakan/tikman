# Topologi Kesehatan PON — Desain

**Tanggal:** 2026-09-01
**Status:** Disetujui untuk implementasi.
**Cakupan:** Satu tab baru pada halaman Pelanggan Bermasalah. Penyaring status pada
tab pelanggan dikerjakan bersamaan karena menyentuh kueri yang sama.

## Tujuan

Halaman Pelanggan Bermasalah menjawab *siapa* yang gagal. Ia tidak menjawab
*di mana* kerusakannya.

Empat puluh satu pelanggan pada satu PON yang semuanya bergejolak bukan empat
puluh satu gangguan rumah tangga — itu satu gangguan di port, splitter, atau
fiber pengumpannya. Selama daftar hanya berisi pelanggan, teknisi mendatangi
rumah satu per satu untuk masalah yang tidak ada di rumah mana pun.

Tampilan ini menaikkan pertanyaannya satu tingkat: OLT → kartu → PON →
pelanggan, dan **hanya menggambar cabang yang bermasalah**.

## Keadaan Sekarang, Terukur

Diukur pada 2026-09-01 terhadap produksi, jendela 24 jam, 39 PON yang memiliki
sekurangnya 5 ONT.

| Temuan | Bukti |
|---|---|
| Pangsa ONT bermasalah tidak membedakan apa pun | Hampir setiap PON menunjukkan 100%, karena hampir setiap ONT mengirim trap |
| Trap per ONT membedakan dengan tajam | median 19, p75 241, p90 610, maksimum 851 |
| Kerusakan punya dua tanda yang berbeda | Cariu 9/8: 851 trap/ONT tapi hanya 16 menit mati. Cariu 8/12: 686 trap/ONT **dan** 171 menit mati |
| Ada kerusakan yang sunyi | Depok 3/2: **1 trap/ONT** tapi pelanggannya kehilangan **10,7%** hari |
| Pola tingkat kartu nyata | Lima dari enam PON terburuk Cariu ada di kartu 8 dan 9 |

Baris terakhir adalah alasan tampilan ini ada. Baris keempat adalah alasan satu
kriteria tidak cukup.

## Keputusan yang Diambil

**1. Dua kriteria, saling melengkapi, bukan saling mencakup.**

Sebuah PON ditampilkan bila salah satu benar:

- **Kehilangan layanan rata-rata > 5% rentang.** Mutlak dan bermakna langsung:
  pelanggan di port ini benar-benar kehilangan layanan. Inilah yang menangkap
  Depok 3/2, yang nyaris tak mengirim trap dan tak terlihat oleh kriteria kedua.
- **Trap/ONT > 5× median OLT itu sendiri, dan > 100.** Gejolak diukur terhadap
  jaringannya sendiri, karena Cariu yang ramai dan Bekasi yang sepi tidak bisa
  dinilai dengan angka yang sama. Inilah yang menangkap Cariu 9/8: bergejolak
  hebat, hampir tanpa kehilangan layanan.

**Angka 100 adalah penilaian, bukan turunan data.** Tanpa batas bawah itu,
aturan relatif menyorot PON Bekasi pada 13 trap/ONT semata karena median Bekasi
sangat rendah — pencilan di jaringan sehat, bukan kerusakan. Ambang yang berlaku
ditampilkan di layar agar keputusannya terlihat, bukan tersembunyi.

**2. Hanya cabang bermasalah yang digambar.** Kartu tanpa PON bermasalah tidak
muncul sama sekali. Ini yang membuat diagram tetap terbaca saat sistem tumbuh:
yang menentukan ukurannya adalah jumlah kerusakan, bukan jumlah perangkat.

**3. Satu OLT sekali lihat.** Dipilih dari penyaring OLT yang sudah ada. Diagram
berisi puluhan chassis akan menjadi tidak terpakai persis ketika sistem mencapai
skala yang dituju.

**4. SVG sendiri, tanpa pustaka grafik.** Topologi GPON adalah pohon berkedalaman
tetap; tata letaknya matematika kolom, bukan graph layout umum. Repo ini punya 13
dependensi runtime, dan satu pustaka diagram untuk satu halaman tidak sepadan.

**5. Penyaringan terjadi di SQL.** Mengirim 39 PON lalu membuang 31 di browser
akan bekerja hari ini dan gagal pada skala yang jadi alasan sistem ini ada.

## Bentuk

Empat tingkat, kiri ke kanan, dihubungkan garis melengkung:

```
                ┌ Kartu 8 ┐   ┌ PON 12 ─ 686 trap/ONT · 12% mati ┐   ┌ pelanggan terparah
  ┌────────┐   ╱│ 2 PON   │──┤                                   │──┤ ONU-8:12 · 1.204 trap
  │ Cariu  │──┤ └─────────┘   └ PON 13 ─ 278 trap/ONT · 12% mati ┘   │ ONU-8:12 ·   987 trap
  │ 651 ONT│  ╲┌ Kartu 9 ┐   ┌ PON 8 ── 851 trap/ONT ·  1% mati ─┐   └ …
  └────────┘   │ 3 PON   │──┤ PON 6 ── 783 trap/ONT ·  0% mati   │
               └─────────┘   └ PON 1 ── 482 trap/ONT ·  7% mati ─┘
```

Pelanggan di bawah tiap PON dibatasi lima terparah: satu PON dapat memuat tujuh
puluh, dan menggambar semuanya mengembalikan masalah yang justru ingin
dihilangkan tampilan ini.

Warna mengikuti bahasa visual tabel: relatif terhadap simpul terburuk pada OLT
itu, memakai token tema sehingga ikut gelap dan terang. Setiap simpul membawa
angkanya sebagai teks, jadi tidak ada informasi yang hanya disampaikan warna.

## Model Data

`GET /api/v1/olts/:id/pon-health?hours=24`

```json
{
  "olt":       { "id": "…", "name": "Cariu", "ont_count": 651 },
  "median_trap_per_ont": 19,
  "thresholds": { "trap_per_ont": 423, "trap_floor": 100, "outage_share": 0.05 },
  "cards": [
    { "slot": 8, "pon_count": 2,
      "pons": [
        { "port": 12, "ont_count": 41, "trap_per_ont": 686, "outage_share": 0.12,
          "worst": [ { "ont_id": "…", "label": "ONU-8:12", "name": "…",
                       "trap_count": 1204, "down_minutes": 340 } ] }
      ] }
  ]
}
```

`thresholds` dikirim agar layar dapat menyatakan ambang yang berlaku, bukan
menyembunyikan keputusannya.

## Komponen

| Berkas | Tanggung jawab |
|---|---|
| `internal/services/pon_health.go` | Kueri agregat dan aturan penyaringan |
| `internal/api/pon_health_handler.go` | Parameter, validasi, bentuk balasan |
| `presentation/components/onts/PonTopology.tsx` | Menggambar pohon. Murni presentasi: menerima pohon, tidak mengambil data |
| `presentation/components/onts/ponLayout.ts` | Menghitung posisi simpul dan jalur garis. Fungsi murni, diuji tanpa DOM |

Tata letak dipisahkan dari penggambaran supaya matematikanya dapat diuji
langsung: beri pohon, periksa koordinat. Menguji tata letak lewat DOM hanya
menguji React.

## Interaksi

Klik PON menyaring tab pelanggan ke port itu — inilah yang menutup alur
pencarian akar masalah: lihat port merah, klik, lihat pelanggannya. Klik
pelanggan membuka detail ONT yang sudah ada.

## Pengujian

Aturan penyaringan diuji di harness Postgres yang sudah dipakai peringkat
pelanggan, dengan kasus yang diambil dari produksi:

- PON yang trap-nya rendah tetapi kehilangan layanannya tinggi **harus** muncul
  (kasus Depok 3/2, yang membuktikan satu kriteria tidak cukup)
- PON yang bergejolak tanpa kehilangan layanan **harus** muncul (kasus Cariu 9/8)
- PON pencilan di jaringan sepi **tidak boleh** muncul (kasus Bekasi, alasan
  batas bawah ada)
- Kartu tanpa PON bermasalah tidak muncul sama sekali
- PON dengan kurang dari lima ONT dikecualikan

Tata letak diuji sebagai fungsi murni. Komponen diuji dengan Vitest: beri pohon,
periksa simpul dan labelnya tergambar.

## Batas yang Diketahui

**Jumlah trap menghitung seluruh trap, bukan hanya alarm.** Tingkat keparahan
tersimpan sejak 2026-08-31; sebelum itu tidak ada. Menyaring hanya alarm akan
mengosongkan sebagian besar rentang tujuh hari. Setelah `community` terkumpul
sepekan, metrik ini dapat dipertajam menjadi hitungan alarm.

**Batas bawah 100 trap/ONT adalah penilaian.** Ia ditampilkan di layar dan
sebaiknya ditinjau setelah dipakai di lapangan.

**Rentang tujuh hari mahal.** Agregat membaca seluruh tabel trap dalam jendela,
dan chunk lama terkompresi. Batas retensi tujuh hari tetap berlaku.

## Urutan Pengerjaan

1. **Penyaring status pada tab pelanggan.** Sudah disetujui terpisah, menyentuh
   kueri yang sama, dan menggeser tanda tangan ke `TroubledFilter`.
2. **Kueri dan endpoint kesehatan PON**, dengan tesnya.
3. **Tata letak dan komponen**, dengan tesnya.
4. **Tab dan penelusuran** yang menyambungkan keduanya.
