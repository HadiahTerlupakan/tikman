# Monitoring Router MikroTik (Border) — Desain

Tanggal: 2026-10-12
Status: disetujui per bagian dalam percakapan, menunggu review spec tertulis

## 1. Konteks dan tujuan

Operator punya router border MikroTik (CCR2116, RouterOS v7) di datacenter
APJII. Router itu memegang sesi eBGP ke transit, ke beberapa IX/CDN, dan
membawa VLAN ke setiap POP. Saat ini satu-satunya cara melihat trafik per
upstream atau per POP adalah Winbox.

Tujuan akhirnya dua, dan dipecah menjadi dua sub-proyek berurutan:

1. **Sub-proyek 1 (spec ini):** trafik per interface dan per VLAN, status
   sesi BGP, dan trafik per upstream — "berapa Mbps lewat transit, berapa
   lewat IX, dan sesi mana yang turun".
2. **Sub-proyek 2 (spec terpisah, menyusul):** trafik per ASN tujuan lewat
   NetFlow/IPFIX — "trafik terbanyak ke mana, dan lewat upstream yang mana".
   Sub-proyek 2 memakai daftar router dan interface (termasuk `ifIndex` dan
   subnet) yang dibangun di sini, karena tanpa itu "lewat mana" tidak bisa
   dijawab.

### Fakta router yang membentuk desain ini

Diambil dari output CLI router pada 2026-10-12 (alamat dan nama disamarkan):

- RouterOS 7.x, sehingga REST API tersedia; tidak perlu library API biner.
- ±50 interface, sekitar 25 di antaranya VLAN. Comment interface adalah label
  yang informatif ("Cariu Via Fibernet", "CDN ILIX"); nama interface tidak.
- Satu interface `pppoe-in` dinamis yang namanya berganti tiap koneksi.
- 7 koneksi BGP di 4 VLAN upstream. Satu VLAN IX membawa empat sesi ke route
  server, sehingga trafik per sesi tidak dapat dipisahkan — hanya per VLAN.
- Dua koneksi BGP dikonfigurasi tetapi tidak pernah naik; keduanya ada di
  `/routing/bgp/connection`, tetapi entri `/routing/bgp/session`-nya tidak
  membawa state yang berguna.
- Full table (±1 juta prefix dari transit). Menarik tabel route lewat REST
  tidak layak.
- `l3-hw-offloading=no`: semua trafik diproses CPU (fastpath). Penjumlahan
  rx VLAN upstream cocok dengan rx port fisiknya, jadi counter VLAN dapat
  dipercaya.
- Port 443 router dipakai `reverse-proxy`; `www-ssl` nonaktif dan belum
  bersertifikat.
- Router dijangkau dari VPS lewat IP publik, bukan lewat tunnel WireGuard.

## 2. Ruang lingkup

Masuk:

- CRUD router MikroTik dengan uji koneksi SNMP + REST sebelum disimpan.
- Discovery interface dan VLAN (nama, comment, VLAN ID, parent, subnet).
- Polling trafik interface per menit lewat SNMP v2c, dengan rollup 5 menit
  dan per jam.
- Polling status BGP dan ringkasan sumber daya router per menit lewat REST.
- Riwayat perubahan state sesi BGP.
- Pencocokan otomatis sesi BGP ke interface berdasarkan subnet.
- Halaman MikroTik: daftar router dan detail router dengan tiga tab.

Tidak masuk:

- NetFlow, trafik per ASN, top talker (sub-proyek 2).
- Notifikasi push/WhatsApp saat sesi BGP turun. Event-nya sudah tercatat
  sehingga notifikasi dapat ditambahkan tanpa mengubah model.
- Mengaitkan VLAN POP ke Site TikMan.
- RouterOS v6 dan RouterOS API biner (port 8728/8729).
- SNMPv3. Router operator memakai v2c; risikonya ditekan dengan community
  khusus yang dibatasi ke IP VPS.
- Menulis konfigurasi ke router. Semua akses read-only.

## 3. Pendekatan

Polling berjalan di dalam `worker` yang sudah ada, dengan antrian sendiri
`router_poll_jobs` yang memakai pola claim `FOR UPDATE SKIP LOCKED` yang sama
dengan `olt_poll_jobs`. Antrian OLT tidak disentuh, deploy tidak berubah, dan
worker yang di-scale ikut mengerjakan job router tanpa container baru.

Service terpisah ditolak untuk sub-proyek ini: ritme dan beban polling-nya
sama dengan job yang sudah ada. Service sendiri baru masuk akal di sub-proyek 2,
karena collector NetFlow adalah listener UDP.

Worker berbagi network namespace dengan `api`; trafik ke IP publik router
keluar lewat IP publik VPS, dan itulah alamat yang diizinkan di router.

## 4. Model data

Migrasi baru `56_mikrotik_monitoring.sql`.

### `mikrotik_routers`

| Kolom | Keterangan |
|---|---|
| `id` | UUID |
| `name` | nama tampilan |
| `host` | IP atau hostname yang dijangkau VPS |
| `site_id` | opsional, FK ke `sites` |
| `snmp_community` | terenkripsi AES-256-GCM, seperti kredensial OLT |
| `snmp_port` | default 161 |
| `api_username` | |
| `api_password` | terenkripsi AES-256-GCM |
| `api_port` | default 8443 |
| `tls_fingerprint` | SHA-256 sertifikat REST, dipin saat router ditambahkan |
| `enabled` | |
| `status` | `up` / `down` / `unknown`, dari SNMP |
| `api_status` | `ok` / `error` / `certificate_changed`, dari REST |
| `api_error` | pesan error REST terakhir |
| `cpu_load`, `memory_total`, `memory_free`, `uptime_seconds`, `ros_version`, `board_name` | dari `/system/resource` |
| `last_seen_at` | terakhir SNMP berhasil |
| `created_at`, `updated_at` | |

### `router_interfaces`

| Kolom | Keterangan |
|---|---|
| `id` | UUID |
| `router_id` | FK, `ON DELETE CASCADE` |
| `name` | identitas interface di dalam router; unik bersama `router_id` |
| `comment` | label utama di UI |
| `type` | `ether`, `vlan`, `bridge`, `loopback`, … |
| `vlan_id`, `parent_name` | hanya untuk VLAN, dari `/interface/vlan` |
| `if_index` | dari SNMP `ifName`; boleh berubah, tidak dipakai sebagai kunci |
| `speed_mbps` | `ifHighSpeed` |
| `disabled` | interface disabled tetap dicatat tetapi tidak di-poll |
| `oper_status` | `up` / `down` |
| `addresses` | `cidr[]` dari `/ip/address` |
| `last_in_bps`, `last_out_bps`, `last_polled_at` | nilai terakhir untuk tabel |
| `prev_in_octets`, `prev_out_octets`, `prev_sample_at` | counter terakhir untuk menghitung delta |

Identitas interface adalah **nama**, bukan `ifIndex`. Di RouterOS `ifIndex`
dapat bergeser saat interface ditambah atau dihapus; menjadikannya kunci akan
menukar grafik VLAN tanpa terlihat. Discovery mencocokkan `ifName` SNMP ke
nama REST untuk mengisi `if_index`.

Interface dinamis (flag `dynamic=true` di REST, misalnya `<pppoe-…>`) tidak
dicatat. Interface yang hilang dari router saat discovery dihapus beserta
metriknya.

### `router_interface_metrics` (hypertable)

`time`, `interface_id`, `in_bps`, `out_bps`. Rollup 5 menit dan per jam,
kompresi dan retensi mengikuti metrics ONT yang sudah ada (migrasi 02, 22,
23, 36 sebagai acuan).

### `router_bgp_sessions`

Satu baris per koneksi BGP yang dikonfigurasi:

| Kolom | Keterangan |
|---|---|
| `router_id` | FK, cascade |
| `name` | nama koneksi (`/routing/bgp/connection`) |
| `remote_address`, `remote_as`, `local_address` | |
| `state` | `established` atau `down` |
| `uptime_seconds`, `prefix_count` | dari sesi; 0 bila down |
| `last_started_at`, `last_stopped_at` | |
| `interface_id` | hasil pencocokan subnet; NULL bila tidak cocok |

Sesi dari `/routing/bgp/session` digabung ke koneksi lewat nama: RouterOS
menamai sesi `<nama koneksi>-<n>`. Koneksi tanpa sesi Established berstatus
`down`.

`interface_id` diisi dengan mencari interface yang salah satu `addresses`-nya
memuat `remote_address`. Sesi multihop yang tidak cocok dengan subnet mana
pun dibiarkan NULL dan ditampilkan "interface tidak diketahui"; tidak ditebak.

### `router_bgp_events`

`id`, `router_id`, `session_name`, `from_state`, `to_state`, `occurred_at`.
`occurred_at` diambil dari `last-started`/`last-stopped` router bila ada,
supaya waktu event adalah waktu kejadian, bukan waktu poll.

## 5. Polling

Antrian `router_poll_jobs` (`router_id`, `kind`, `next_run_at`, kolom lease
seperti `olt_poll_jobs`). Loop `runNextJob` di worker mencoba antrian OLT,
lalu antrian router.

| Job | Interval | Sumber |
|---|---|---|
| `interfaces` | 1 menit | SNMP GETBULK `ifXTable`: `ifHCInOctets`, `ifHCOutOctets`, `ifHighSpeed`; `ifOperStatus` |
| `bgp` | 1 menit | REST `/routing/bgp/connection`, `/routing/bgp/session`, `/system/resource` |
| `discovery` | 6 jam, saat router dibuat, dan lewat tombol refresh | REST `/interface`, `/interface/vlan`, `/ip/address`; SNMP `ifName` |

Discovery ulang juga mencocokkan ulang sesi BGP ke interface.

Aturan:

- **bps** = delta octet × 8 / delta detik antar sampel. Bila counter lebih
  kecil dari sebelumnya (reboot, reset), sampel dibuang dan counter disimpan
  sebagai titik awal baru.
- **Event BGP** ditulis hanya bila state berbeda dari baris tersimpan.
- **SNMP timeout** (2 detik) → `status=down`. **REST gagal** (5 detik) →
  `api_status=error` dengan pesannya. Keduanya independen: SNMP dapat jalan
  sementara REST mati, atau sebaliknya.
- **Fingerprint sertifikat berbeda** dari yang dipin → `api_status=certificate_changed`,
  job `bgp` dan `discovery` berhenti sampai Admin menerima fingerprint baru.
  Verifikasi TLS dilakukan terhadap fingerprint, bukan CA, karena router
  memakai sertifikat self-signed.
- Nilai REST RouterOS seluruhnya string (`"prefix-count":"9175"`,
  `"uptime":"1d14h52m22s720ms"`); parser angka dan durasi RouterOS
  diperlakukan sebagai input jaringan dan divalidasi.

Volume: ±50 interface × 1.440 sampel/hari ≈ 72 ribu baris/hari sebelum
rollup. Nilai per menit adalah rata-rata; lonjakan beberapa detik tidak
terlihat.

## 6. API

Grup `/api/v1/routers`, RBAC mengikuti pola OLT.

| Endpoint | Peran |
|---|---|
| `GET /routers` | semua yang login |
| `POST /routers/test-connection` — uji SNMP + REST, kembalikan fingerprint | Admin |
| `POST /routers` — termasuk fingerprint yang sudah dikonfirmasi | Admin |
| `PUT /routers/:id` | Admin, Teknisi |
| `DELETE /routers/:id` | Admin |
| `POST /routers/:id/refresh` — discovery sekarang | Admin, Teknisi |
| `POST /routers/:id/trust-certificate` | Admin |
| `GET /routers/:id/interfaces` | semua |
| `GET /routers/:id/interfaces/:ifId/traffic?from&to` | semua |
| `GET /routers/:id/bgp/sessions` | semua |
| `GET /routers/:id/bgp/events?from&to` | semua |

Resolusi `traffic` dipilih dari panjang rentang: data per menit untuk
≤ 6 jam, rollup 5 menit untuk ≤ 7 hari, rollup per jam di atasnya.

Kredensial tidak pernah dikembalikan oleh endpoint mana pun. Setiap create,
update, delete, dan trust-certificate dicatat di audit log.

## 7. Tampilan

Menu baru **MikroTik**: `/routers` (daftar) dan `/routers/:id` (detail).

**Tab Upstream & BGP** (default)

- Grafik area bertumpuk download per interface upstream (interface yang
  memiliki sesi BGP), dan grafik upload di bawahnya. Pemilih rentang waktu
  mengikuti pola halaman Graphs; grafik memakai recharts.
- Tabel sesi BGP dikelompokkan per interface: AS, state, uptime, prefix,
  `last-stopped`. Sesi down ditandai merah.
- Daftar event BGP terbaru.

**Tab Interface & VLAN**

- Tabel dikelompokkan per port fisik (parent). Kolom: comment, nama, VLAN ID,
  status, kecepatan, trafik terakhir. Pencarian berdasarkan comment/nama.
- Klik baris membuka grafik trafik interface itu.

**Tab Router**

- CPU, RAM, uptime, versi RouterOS, status SNMP dan REST beserta error
  terakhir, waktu baca terakhir.
- Form edit, tombol refresh discovery, tombol terima sertifikat baru.

**Label arah**

- Interface upstream: rx router = **Download**, tx = **Upload**.
- Interface lain (VLAN POP): tx router = **Ke pelanggan**, rx =
  **Dari pelanggan**, supaya grafik POP tidak terbaca terbalik.

## 8. Pengujian

Backend, logika murni (fixture disamarkan dari data router asli):

- bps dari delta counter 64-bit, termasuk counter mundur.
- Pencocokan peer ke interface: /31, /30, /23 (alamat route server kedua
  berada di setengah atas /23), dan multihop yang tidak cocok.
- Penggabungan connection + session: koneksi tanpa sesi Established → `down`.
- Diff state → event: tiga sesi yang turun di detik yang sama menghasilkan
  tiga event dengan `occurred_at` dari router.
- Parser angka dan durasi RouterOS, termasuk input rusak.
- Interface dinamis tidak ikut discovery.

Backend, klien dan infrastruktur:

- Klien REST dengan `httptest.NewTLSServer`: fingerprint cocok diterima,
  beda ditolak dengan `certificate_changed`, 401 dilaporkan.
- Test Postgres untuk claim `router_poll_jobs` dan rollup.
- Test handler untuk RBAC setiap endpoint.
- Walk SNMP dikecualikan dari unit test sesuai aturan kode jaringan di
  CLAUDE.md; diverifikasi langsung ke router setelah deploy.

Frontend (Vitest): label arah, pengelompokan sesi per interface, pemilihan
resolusi.

Fixture JSON REST diambil dari router asli (`curl` ke `/rest/routing/bgp/session`
dan `/rest/interface/vlan`) dan **disamarkan sebelum di-commit**: repo ini
publik, jadi IP publik, nama router, dan serial number diganti nilai dari
rentang dokumentasi (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24).

Setelah deploy: `/api/v1/routers` menjawab 401 (bukan 404), dan grafik
upstream cocok dengan `/interface monitor-traffic` di router.

## 9. Setup di router

Dijalankan operator; TikMan tidak mengubah router. Ditambahkan ke
`docs/operator_guide.md` sebagai bagian baru.

```
/user group add name=tikman-ro policy=read,api,rest-api
/user add name=tikman group=tikman-ro password="PASSWORD" address=IP_VPS/32
/certificate add name=tikman-rest common-name=IP_ROUTER subject-alt-name=IP:IP_ROUTER days-valid=3650 key-usage=digital-signature,key-encipherment,tls-server
/certificate sign tikman-rest
/ip service set www-ssl certificate=tikman-rest port=8443 address=IP_VPS/32 disabled=no
/snmp community add name="COMMUNITY" addresses=IP_VPS/32 read-access=yes write-access=no
```

Bila chain input punya rule drop, izinkan tcp/8443 dan udp/161 dari `IP_VPS`
di atasnya. Panduan juga menyarankan membatasi `addresses` setiap community
SNMP lain ke pemakainya.

## 10. Risiko

- **SNMP v2c lewat internet**: community terkirim tanpa enkripsi. Ditekan
  dengan community khusus yang dibatasi ke IP VPS.
- **Nama sesi BGP**: penggabungan connection ↔ session bergantung pada pola
  penamaan `<koneksi>-<n>` RouterOS v7. Bila ternyata berbeda di fixture asli,
  penggabungan memakai `remote.address` sebagai gantinya.
- **Rename interface**: karena identitasnya nama, interface yang di-rename
  di router tercatat sebagai interface baru dan riwayat grafiknya terputus.
  Rename di router border jarang; ini dipilih karena lebih aman daripada
  `ifIndex` yang dapat menukar grafik diam-diam.
- **IP VPS berubah**: semua pembatasan alamat di router ikut harus diubah.
