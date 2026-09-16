# Pemetaan Jaringan — Desain

**Tanggal:** 2026-09-16
**Status:** Menunggu review user

## Kenapa

TikMan sudah punya peta plant: model `ODC`, `ODCFeed`, `ODP` dengan koordinat,
jalur kabel, dan pengikatan ke card serta port PON di OLT. Peta itu bisa
menggambar jalur, mengukur panjangnya, dan mencari alamat dari koordinat.

Setelah sekian lama, isinya **satu ODC dan satu ODP**, sementara ada 1.179 ONT
di sistem. Petanya tidak dipakai.

Sebabnya bukan kemampuan, melainkan beratnya pengisian. Formulir ODP TikMan
menuntut Site, ODC induk, Card, dan PON port sebelum satu titik bisa disimpan.
Untuk memetakan ratusan titik di lapangan, itu terlalu lambat, jadi tidak ada
yang memulainya.

Desain ini menggantinya dengan alur yang dipakai panel GenieACS pihak ketiga
yang user coba: tekan tombol jenis node, klik di peta, isi seperlunya, selesai.
Strukturnya longgar dan hubungannya dibuat dengan menarik kabel antar node,
bukan lewat medan wajib di formulir.

**Konsekuensi yang diterima sadar:** node tidak lagi terikat ke card dan port
PON, jadi peta tidak bisa menjawab "ODP ini di port berapa". Kaitan itu ada di
model lama tapi tidak pernah terisi. Kalau kelak dibutuhkan, ia ditambahkan
sebagai medan opsional pada node, bukan sebagai syarat menyimpan.

## Model data

Dua tabel baru, menggantikan peran `odcs`, `odc_feeds`, dan `odps`.

### `mapping_nodes`

| Kolom | Isi |
|---|---|
| `id` | UUID |
| `node_id` | pengenal unik yang dipakai edge (teks) |
| `type` | `server`, `odc`, `odp`, atau `ont` |
| `name` | nama tampil, wajib |
| `latitude`, `longitude` | wajib; terisi dari klik di peta |
| `capacity` | jumlah slot keluar; dipakai memvalidasi penyambungan |
| `splitter` | rasio splitter, mis. `1:8` |
| `pppoe` | username PPPoE pelanggan (untuk node `ont`) |
| `serial_number` | serial ONT |
| `notes` | catatan bebas |
| `created_at`, `updated_at` | |

`node_id` unik. Menyimpan node dengan `node_id` yang sudah ada ditolak.

### `mapping_edges`

| Kolom | Isi |
|---|---|
| `id` | UUID |
| `edge_id` | pengenal unik |
| `source`, `target` | `node_id` kedua ujung |
| `fiber_type` | salah satu dari tujuh nilai di bawah |
| `distance` | panjang kabel (meter) |
| `waypoints` | JSON larik titik `{lat, lng}` — jalur sebenarnya |
| `notes` | |
| `created_at`, `updated_at` | |

Tujuh jenis kabel:

| `fiber_type` | Dari → ke |
|---|---|
| `feeder` | server/OLT → ODC |
| `distribution` | ODC → ODP |
| `drop` | ODP → ONT |
| `odp_to_odp` | ODP → ODP (kaskade) |
| `odp_to_odp_ratio` | ODP → ODP, melewati splitter |
| `odc_to_odc` | ODC → ODC (kaskade) |
| `odc_to_odc_ratio` | ODC → ODC, melewati splitter |

Kaskade ODP dan ODC adalah kemampuan yang model lama tidak punya sama sekali.

### `map_settings`

Satu baris: `center_lat`, `center_lng`, `default_zoom`, `max_zoom_in`,
`max_zoom_out`.

## Aturan kapasitas

Penyambungan ditolak bila slot node sumber sudah penuh. Dihitung per jenis
tujuan, bukan total:

- **ODC** → jumlah edge ke ODP, dan jumlah edge `odc_to_odc` ke ODC, masing-masing
  dibandingkan dengan `capacity` ODC itu.
- **ODP** → jumlah edge ke ONT, dan jumlah edge `odp_to_odp` ke ODP.

`capacity` bernilai 0 berarti tanpa batas. Penolakan menyebut angkanya:
`ODC "X" slots are full (8/8)`.

## Tampilan dan cara pakai

Satu halaman peta dengan basemap satelit, sama seperti sekarang.

**Toolbar atas:** `Cari`, `+ Server`, `+ ODC`, `+ ODP`, `+ ONT`, `Tarik kabel`.
Kanan atas: pilihan tampilan `Peta` / `Daftar` / `Pengaturan`.

**Menambah node:** tekan tombol jenisnya → toolbar berganti jadi `Batal` dan
`Simpan` → klik di peta untuk menaruh penanda → tekan `Simpan` → formulir muncul
berisi nama (wajib), rasio splitter, kotak centang "isi koordinat manual",
latitude dan longitude yang sudah terisi dari klik tadi, dan catatan. Untuk node
`ont` formulirnya juga meminta PPPoE dan serial.

**Menarik kabel:** tekan `Tarik kabel` → klik node awal → klik titik-titik
belokan di peta → klik node tujuan → pilih jenis kabel → simpan. Panjangnya
dihitung dari jalur yang digambar, bukan garis lurus. Satu klik yang salah
dibatalkan satu titik, bukan seluruh jalur.

**Kartu jumlah** di bawah peta: jumlah Server/OLT, ODC, ODP, dan ONT.

**Daftar** menampilkan node dan kabel dalam tabel, bisa disunting dan dihapus
dari sana.

## Yang tidak ikut

Panel menyimpan seluruh peta dengan satu operasi "sync" yang menghapus semua
baris lalu menulis ulang. Itu tidak diikuti: dua orang yang menyunting
bersamaan akan saling menimpa, dan satu kegagalan di tengah menghilangkan
seluruh peta. Di TikMan setiap node dan kabel disimpan sendiri-sendiri, seperti
data lain di sistem ini.

## Nasib model lama

`odcs`, `odc_feeds`, dan `odps` berisi satu ODC, satu ODP, dan nol feed. Tapi
ODP bukan hanya entitas peta: `models.ONT` punya `ODPID` dan `ODPPort` dengan
indeks unik `uq_onts_odp_port`, registrasi GPON ZTE menanyakan keduanya
(`zte_register_odp.go`), dan ada rute `PUT`/`DELETE /onts/:id/odp` untuk
menugaskan ONT ke sebuah port ODP.

Di produksi, **nol dari 1.179 ONT punya ODP**. Jadi jalur itu ada di kode tapi
tidak pernah dipakai, sama seperti petanya.

Keputusannya: ODP lama diganti, bukan didampingi. Dua konsep "ODP" yang hidup
berdampingan akan membingungkan siapa pun yang membaca sistem ini nanti.

- `ONT.ODPID` menunjuk ke baris `mapping_nodes` bertipe `odp`; `ONT.ODPPort`
  dan indeks uniknya tetap, sehingga dua ONT tetap tidak bisa berbagi port.
- Registrasi GPON ZTE tetap bekerja — validasinya hanya berganti tabel rujukan.
- `odcs`, `odc_feeds`, dan `odps` dihapus setelah satu ODC dan satu ODP-nya
  dipindahkan ke `mapping_nodes`.
- Halaman peta lama beserta komponennya dihapus, digantikan yang baru.

Tidak ada data produksi yang hilang.

## Pengujian

- Migrasi: node lama pindah, kolom dan indeks baru ada di Postgres.
- Model: `node_id` dan `edge_id` unik; `type` dan `fiber_type` hanya menerima
  nilai yang sah.
- Kapasitas: ODC penuh menolak ODP kesembilan; ODP penuh menolak ONT; kaskade
  dihitung terpisah dari tujuan biasa; `capacity` 0 tidak pernah menolak.
- API: membuat, mengubah, menghapus node dan kabel; menolak `node_id` ganda.
- Peta: menaruh node lewat klik, menggambar kabel berbelok, membatalkan satu
  titik, panjang terhitung dari jalur bukan garis lurus.
