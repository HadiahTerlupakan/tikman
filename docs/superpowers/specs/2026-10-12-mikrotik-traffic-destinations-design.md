# MikroTik: tujuan trafik per POP dan upstream (NetFlow/IPFIX)

Tanggal: 2026-10-12
Status: disetujui di percakapan, menunggu review spec tertulis
Melanjutkan: `2026-10-12-mikrotik-monitoring-design.md` (sub-proyek 2)

## Tujuan

Operator ingin tahu **ke mana trafik pelanggan mengarah** — layanan/organisasi
tujuan (Google, Meta, CDN, ...) — dari POP mana, lewat upstream mana, dan
bagaimana trennya dari menit ke tahun. Dipakai untuk dua hal:

1. **Perencanaan:** memutuskan peering/IX/cache dan kapasitas upstream dari
   tren harian, mingguan, bulanan dan tahunan, dengan rentang tanggal bebas.
2. **Troubleshooting:** saat satu POP atau upstream penuh, melihat beberapa
   menit terakhir trafiknya ke mana, sampai ke IP tujuan.

## Batas yang ditentukan topologi

Router border tidak melakukan NAT. Setiap POP menerima satu prefix publik
kecil (/29) dari blok milik operator dan me-NAT pelanggannya sendiri. Jadi di
border, alamat sumber sebuah flow adalah salah satu IP publik POP, bukan IP
pelanggan. Konsekuensinya:

- **Bisa:** per POP × tujuan (ASN/organisasi, dan IP tujuan untuk 7 hari
  terakhir) × upstream × arah.
- **Tidak bisa dari border:** per pelanggan. Itu membutuhkan NetFlow dari
  router tiap POP (sebelum NAT) — di luar cakupan.

## Keputusan

| Hal | Keputusan |
|---|---|
| Ekspor | IPFIX dari RouterOS v7 `traffic-flow`, `interfaces=all`, dengan sampling paket |
| Penghitungan | setiap paket dicatat di interface masuknya: masuk dari upstream = download, masuk dari VLAN POP = upload |
| Collector | service baru `flowd` (Go, stdlib), container sendiri, UDP 2055 |
| Tujuan | IP sisi luar → ASN + nama organisasi dari dataset publik iptoasn.com (`ip2asn-v4.tsv.gz`), diunduh harian |
| Penyimpanan | ringkasan, bukan flow mentah |
| Protokol IP | IPv4 saja di tahap ini |
| Akses | semua peran melihat; pengaturan hanya Admin |

Flow mentah tidak disimpan: dengan ratusan ribu koneksi aktif di router, itu
berarti jutaan baris per hari — terlalu besar untuk VPS ini. Collector siap
pakai + ClickHouse ditolak karena menambah dua komponen berat di VPS yang
juga menjalankan k3s dan Jenkins, sementara pertanyaan operator terjawab
oleh ringkasan.

## Arsitektur

```
router border ──IPFIX/UDP 2055──▶ flowd ──▶ TimescaleDB ◀── api ◀── UI
                                    ▲
                     iptoasn.com (harian)  +  router_interfaces (POP, upstream)
```

### `flowd`

`cmd/flowd`, dibangun di image worker seperti `trapd`, tetapi **tidak** di
network namespace `api`: ia container sendiri yang mempublikasikan
`2055/udp` (juga di `docker-compose.vps.yml`). Mati atau restart-nya tidak
menyentuh polling OLT, trap, maupun WhatsApp.

Alur per paket UDP:

1. **Sumber:** IP pengirim harus `host` salah satu `mikrotik_routers` yang
   aktif dengan flow dinyalakan; selain itu dibuang tanpa diproses.
2. **Parse IPFIX** (RFC 7011): header, set template, set data. Template
   disimpan per (router, observation domain, template id). Field yang dibaca:
   `sourceIPv4Address`, `destinationIPv4Address`, `ingressInterface`,
   `egressInterface`, `octetDeltaCount`, `packetDeltaCount`,
   `protocolIdentifier`, `destinationTransportPort`. Record tanpa template
   yang dikenal dibuang sampai templatenya tiba.
3. **Perkaya** (lihat Pemetaan).
4. **Jumlahkan** di memori ke ember menit berjalan.

Setiap menit ember ditutup dan ditulis (lihat Penyimpanan). Penulisan yang
gagal dicoba sekali lagi di menit berikutnya lalu dibuang, dan dicatat di
log; collector tidak boleh menumpuk memori karena database lambat.

### Pemetaan

- **Rasio sampling:** dari kolom router `flow_sampling` (1:N) yang diisi Admin
  dan harus sama dengan konfigurasi router. Byte dan paket dikalikan N.
- **POP:** IP sisi lokal (sumber untuk upload, tujuan untuk download)
  dicocokkan ke subnet di `router_interfaces.addresses` milik interface yang
  **bukan** upstream. Hasilnya `pop_interface_id`; tidak cocok → `NULL`
  ("lainnya"). Tabel subnet dimuat ulang dari database tiap 5 menit.
- **Upstream:** interface upstream = interface yang dicocokkan ke sesi BGP
  (`router_bgp_sessions.interface_id`, sudah ada dari sub-proyek 1). Flow
  download: upstream = `ingressInterface`; upload: upstream =
  `egressInterface`. ifIndex dipetakan lewat `router_interfaces.if_index`.
  Flow yang tidak menyentuh upstream (antar-POP, ke router sendiri) dibuang.
- **Arah:** `ingressInterface` adalah upstream → `download`; selain itu, bila
  `egressInterface` upstream → `upload`.
- **Tujuan:** IP sisi luar → ASN dan nama organisasi. Dataset iptoasn dimuat
  ke memori sebagai array rentang terurut; pencarian biner. Tidak ditemukan →
  ASN 0 ("tidak dikenal").

### Batas sumber daya

Mengikuti pelajaran klien RouterOS API (sumber yang tidak dipercaya tidak
boleh menentukan pemakaian memori):

- ukuran paket UDP dibaca maksimal 64 KiB;
- maksimal 256 template per router; panjang field template divalidasi;
  record yang melewati batas set dibuang;
- maksimal 50.000 kunci agregasi per router per menit; kelebihannya masuk ke
  kunci "lainnya" (ASN 0) dan dihitung di metrik `dropped_keys`;
- maksimal 100.000 IP tujuan berbeda per POP per 5 menit di penghitung IP
  teratas; kelebihannya hanya dijumlahkan ke total.

### Dataset ASN

`flowd` mengunduh `https://iptoasn.com/data/ip2asn-v4.tsv.gz` saat start
(bila salinan lokal lebih tua dari 24 jam) dan tiap 24 jam, menyimpannya di
volume, dan menukar tabel di memori secara atomik. Unduhan gagal atau isi
tidak valid → tabel lama tetap dipakai. Nama organisasi per ASN disimpan juga
di tabel `asn_names` agar API bisa menampilkannya tanpa bergantung pada
`flowd`.

## Penyimpanan

Tabel baru (migrasi 58):

**`router_flow_minute`** — hypertable, satu baris per menit per
(router, POP, upstream, arah, ASN):

| Kolom | Tipe |
|---|---|
| `time` | timestamptz (awal menit) |
| `router_id` | uuid |
| `pop_interface_id` | uuid, nullable ("lainnya") |
| `upstream_interface_id` | uuid |
| `direction` | `download` / `upload` |
| `asn` | integer (0 = tidak dikenal / lainnya) |
| `bytes`, `packets` | bigint (sudah dikalikan sampling) |

Per (router, POP, arah, menit) hanya **20 ASN teratas** yang disimpan
sebagai baris sendiri; sisanya dijumlahkan ke baris `asn = 0`. Total per POP
tetap tepat.

**Rollup** (continuous aggregate, `materialized_only=false` seperti
`router_interface_metrics`):

| Resolusi | Sumber | Retensi |
|---|---|---|
| 1 menit | `router_flow_minute` | 7 hari |
| 5 menit | `router_flow_5min` | 90 hari |
| 1 jam | `router_flow_hourly` | 2 tahun |
| 1 hari | `router_flow_daily` | tanpa batas |

**`router_flow_top_ips`** — hypertable, per (router, POP, arah, 5 menit):
100 IP tujuan teratas dengan `ip inet`, `asn`, `bytes`. Retensi 7 hari.

**`asn_names`** — `asn` (PK), `name`, `country`, `updated_at`.

**`mikrotik_routers`** mendapat `flow_enabled bool` dan `flow_sampling int`
(default 100), serta `flow_last_seen_at timestamptz`.

## API

Di bawah `/api/v1/routers/:id`, semua peran (Admin+Technician+Viewer):

- `GET flows/destinations?from&to&pop&upstream&direction&limit=10` — deret
  waktu bertumpuk untuk N ASN teratas + "Lainnya". Resolusi dipilih otomatis
  dari tabel paling rinci yang retensinya mencakup `from` (1 menit ≤ 7 hari,
  5 menit ≤ 90 hari, 1 jam ≤ 2 tahun, selainnya harian), dibatasi agar satu
  respons ≤ 1.500 titik per seri.
- `GET flows/summary?from&to&pop&upstream&direction` — per ASN: nama, total
  byte, persen, rata-rata dan puncak bit/s, pembagian per upstream.
- `GET flows/pops?from&to&direction` — per POP: total dan 3 ASN teratas.
- `GET flows/top-ips?from&to&pop&direction` — hanya dalam 7 hari terakhir;
  rentang di luarnya → 400 dengan pesan yang jelas.

Admin: `PUT /routers/:id` menerima `flow_enabled` dan `flow_sampling`
(1–10000).

## UI

Tab baru **Tujuan Trafik** di halaman router:

- `TrafficRangePicker` yang sudah ada (preset 15m/1j/24j/7h/30h/90h/1t +
  rentang bebas); preset bergerak tiap menit seperti grafik interface.
- Filter POP, upstream, arah.
- Grafik bertumpuk 10 tujuan teratas + "Lainnya" (bit/s).
- Tabel tujuan: organisasi, ASN, total, %, rata-rata, puncak, batang
  pembagian per upstream.
- Tabel POP: total + 3 tujuan teratas; klik baris = filter POP.
- Panel **IP teratas** (dinonaktifkan dengan keterangan bila rentang
  melewati 7 hari).

Tab **Router**: status flow — aktif/nonaktif, rasio sampling, flow terakhir
diterima, versi dataset ASN. Peringatan bila flow aktif tetapi tidak ada
data lebih dari 5 menit. Form router (Admin): toggle "Kumpulkan NetFlow" dan
rasio sampling.

## Operasional

`docs/operator_guide.md` mendapat bagian NetFlow:

- di router (dijalankan operator):
  `/ip traffic-flow set enabled=yes interfaces=all packet-sampling=yes sampling-interval=... sampling-space=...`
  dan `/ip traffic-flow target add dst-address=IP_VPS port=2055 version=ipfix`,
  dengan saran mulai 1:100 dan memantau CPU;
- membuka `2055/udp` di firewall provider VPS, dibatasi ke IP router bila
  memungkinkan;
- cara memastikan data masuk (status di tab Router).

Deploy: `flowd` ditambahkan ke urutan deploy; ia tidak bergantung pada
namespace `api`, jadi cukup `up -d --no-deps flowd`.

## Pengujian

- Parser IPFIX: paket contoh (template + data, rentang dokumentasi), template
  datang setelah data, panjang variabel, paket terpotong, template melebihi
  batas, sumber tidak dikenal.
- Pemetaan: IP → POP lewat subnet, ifIndex → upstream, arah, IP → ASN
  (batas rentang, tidak ditemukan), perkalian sampling.
- Agregasi: 20 ASN + "lainnya" menjaga total; batas 50.000 kunci; 100 IP
  teratas.
- Postgres: rollup dan retensi; API memilih resolusi yang benar per rentang;
  top-ips menolak rentang > 7 hari.
- Frontend: filter, rentang, peringatan data berhenti, panel IP teratas
  nonaktif di luar 7 hari.

## Di luar cakupan

Per pelanggan (butuh NetFlow dari router POP), IPv6, notifikasi lonjakan,
NetFlow dari router selain border, penyimpanan flow mentah.
