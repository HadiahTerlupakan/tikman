# Operator Guide

Panduan operasional untuk sistem provisioning ONT TikMan.

## Konsep Dasar

### Alur Provisioning
```
1. Login → pilih ONT → klik "Provision"
2. Pilih template (opsional) atau gunakan manual config
3. Aktifkan konfirmasi "Saya sudah memeriksa konfigurasi"
4. Sistem menjalankan provisioning:
   a. Capture Before Snapshot (SNMP read)
   b. Buat job (status: pending → running)
   c. Kirim CLI commands ke OLT (Telnet)
   d. Capture After Snapshot
   e. Compare sebelum vs sesudah
5. Jika drift terdeteksi → otomatis rollback
6. Jika sukses → job ditandai success
```

### Status Job
| Status | Arti |
|--------|------|
| `pending` | Job dibuat, menunggu eksekusi |
| `running` | Provisioning sedang berjalan |
| `success` | Provisioning sukses & terverifikasi |
| `failed` | Provisioning gagal, rollback dijalankan |
| `rolled_back` | Sudah di-rollback ke snapshot sebelumnya |

---

## Template Konfigurasi

### Membuat Template
1. Buka menu **Config Templates**
2. Klik **Create Template**
3. Isi nama (3-100 karakter, unik), vendor (ZTE/HSGQ), deskripsi
4. Aktifkan **Default Template** jika ingin jadi default vendor tersebut
5. Simpan

### Menghapus Template
Template yang sudah digunakan oleh job provisioning **tidak bisa dihapus** (HTTP 409). Hapus job terkait terlebih dahulu atau arsipkan template dengan rename.

---

## Provisioning Single ONT

### Langkah-langkah
1. Buka menu **ONT Monitoring**
2. Cari ONT target (filter by serial/status/OLT)
3. Klik **Provision** pada baris ONT
4. Pilih template dari dropdown (atau kosongkan untuk manual)
5. Nyalakan switch **"Saya sudah memeriksa konfigurasi"**
6. Submit

### Melihat History
Klik **History** pada baris ONT untuk melihat semua job provisioning sebelumnya beserta error message.

---

## Troubleshooting

### "SNMP_UNREACHABLE" (502)
Before snapshot gagal dibaca. Cek:
- OLT dapat di-ping dari server backend?
- SNMP community di OLT benar?
- Port SNMP (default 161) terbuka?

### "provision execution failed" (500)
CLI command ditolak OLT. Cek:
- Kredensial Telnet OLT benar?
- OLT tidak sedang dikonfigurasi oleh operator lain?
- Lihat error detail di Provision History

### "config drift detected"
Konfigurasi setelah push tidak sesuai snapshot. Kemungkinan:
- OLT menolak command secara diam-diam (commit gagal)
- Vendor firmware menormalisasi nilai tertentu
- Job otomatis di-rollback; periksa log untuk detail diff

### "another provisioning job is already running" (409)
Hanya satu job aktif per ONT. Tunggu job sebelumnya selesai atau periksa statusnya di History.

### "config template referenced by N provisioning job(s)" (409)
Template dipakai job historis. Tidak bisa dihapus — ini by design untuk menjaga referensi audit.

---

## Rollback

Rollback berjalan otomatis pada kegagalan. Operator tidak perlu manual rollback. Job yang di-rollback berstatus `rolled_back` dengan error message aslinya tetap tercatat.

Rollback bersifat idempotent — menjalankan ulang snapshot restore menghasilkan state akhir yang sama.

---

## VPN Site (WireGuard)

### Prasyarat sekali jalan di host VPS

Interface `wg0` dibuat oleh container `api`, tetapi modul kernel WireGuard hanya
bisa dimuat dari host. Tanpa modul itu, `Reconcile` saat startup gagal dengan
`create wg0 (load the wireguard kernel module on the VPS host: modprobe
wireguard): operation not supported`, API tetap hidup, dan halaman VPN
melaporkan semua site "Belum pernah terhubung" walaupun perangkat di site sudah
benar.

Jalankan di host VPS, bukan di dalam container:

```bash
# Muat modul sekarang
sudo modprobe wireguard

# Pastikan termuat kembali setiap reboot
echo wireguard | sudo tee /etc/modules-load.d/wireguard.conf

# Verifikasi
lsmod | grep wireguard
```

Bila `modprobe` gagal, kernel VPS belum punya modulnya. Pada Debian/Ubuntu
pasang `wireguard-tools` beserta paket header kernel yang sesuai, lalu ulangi.

### Port UDP

Port UDP harus sama di tiga tempat, dan salah satu saja berbeda membuat tidak
ada site yang bisa handshake:

1. `WIREGUARD_PORT` pada berkas `.env` deployment (dipublikasikan oleh
   `docker-compose.yml`).
2. Port yang disimpan pada halaman VPN di TikMan.
3. Aturan izin UDP masuk di firewall penyedia VPS (Security Group, Cloud
   Firewall, atau sejenisnya).

Nilai bawaan `51820`. Bila penyedia hanya mengizinkan port lain, ubah ketiganya
bersamaan lalu jalankan ulang `docker compose up -d`.

### Troubleshooting

**Semua site "Belum pernah terhubung" sejak awal.** Hampir selalu masalah di
sisi VPS, bukan site: periksa modul kernel dan port UDP di atas sebelum
memeriksa perangkat di lokasi.

**Satu site saja yang tidak terhubung.** Baru periksa sisi site: konfigurasi
sudah ditempel, perangkat menyala, dan internet site hidup.

**Site tidak bisa dihapus karena masih punya tunnel.** Hapus dulu tunnel site
tersebut di halaman VPN. Menghapus site lebih dulu akan meninggalkan tunnel yang
masih aktif di kernel dan menahan subnetnya.

## Memasang tunnel di perangkat site

Halaman VPN sudah menampilkan langkah-langkah ini di atas teks konfigurasi, jadi
tidak perlu membuka dokumen ini saat berada di lokasi. Dicatat di sini sebagai
rujukan.

### MikroTik

Teks yang dihasilkan adalah perintah RouterOS, bukan berkas.

1. Buka Winbox atau SSH ke MikroTik di lokasi tersebut.
2. Masuk ke menu **New Terminal**.
3. Tempel seluruh blok sekaligus, lalu tekan Enter.

Periksa hasilnya:

```
/interface/wireguard/peers print
```

Kolom `last-handshake` akan terisi dalam beberapa detik. Kalau tetap kosong,
periksa apakah router bisa menjangkau internet dan apakah alamat endpoint di
halaman VPN menunjuk ke IP publik server, bukan ke nama yang di-proxy.

### Linux

Teks yang dihasilkan adalah isi berkas konfigurasi.

```bash
sudo apt install wireguard              # bila belum terpasang
sudo nano /etc/wireguard/wg0.conf       # tempel isinya di sini
sudo chmod 600 /etc/wireguard/wg0.conf  # berkas ini memuat kunci privat
sudo wg-quick up wg0
sudo systemctl enable wg-quick@wg0      # nyala lagi setelah reboot
```

Nama berkas menentukan nama interface: `wg0.conf` menghasilkan `wg0`. Menyimpan
dengan nama lain berarti perintah `wg-quick up wg0` tidak menemukannya.

Periksa hasilnya:

```bash
sudo wg show
```

Baris `latest handshake` akan muncul dalam beberapa detik.

### Kalau tunnel hidup tapi OLT tetap tidak terjangkau

Konfigurasi yang dihasilkan sudah memuat aturan NAT dan penerusan paket, jadi
kasus ini biasanya berarti subnet yang diisi di halaman VPN tidak sama dengan
jaringan tempat OLT sebenarnya berada. Periksa dari perangkat site sendiri bahwa
alamat OLT memang bisa dijangkau, lalu cocokkan subnetnya.

## Deploy ke VPS lewat Jenkins

`Jenkinsfile` di root repo menjalankan pipeline di VPS tempat Jenkins berada:
build image, jalankan stack, lalu pastikan API sehat dan route VPN benar-benar
terpasang. Tidak ada registry di jalurnya, jadi tidak ada kredensial image yang
perlu dijaga.

Pipeline memakai `docker-compose.vps.yml` sebagai override. Isinya tiga hal:
port 8080 tidak dipublikasikan (Jenkins biasanya sudah memakainya, dan frontend
mem-proxy `/api` secara internal), frontend tidak dipublikasikan sama sekali
karena diakses lewat Cloudflare Tunnel, dan container `cloudflared` ditambahkan.

### Sekali di awal

1. Muat modul kernel dan buat permanen:
   `sudo modprobe wireguard && echo wireguard | sudo tee /etc/modules-load.d/wireguard.conf`
2. Buat `/opt/tikman/.env` berisi nilai dari `.env.example` — termasuk
   `ENCRYPTION_KEY` 32 byte, `SESSION_SECRET`, password database, dan
   `CLOUDFLARE_TUNNEL_TOKEN`. Berkas ini tidak pernah masuk repo maupun Jenkins.
3. Di Jenkins, buat Pipeline job yang menunjuk ke repo ini dengan
   *Script Path* `Jenkinsfile`. Biarkan pemicunya manual.

### Dua catatan DNS yang mudah terlewat

Cloudflare Tunnel hanya membawa HTTP dan HTTPS. Handshake WireGuard adalah UDP
dan tidak bisa lewat sana, sehingga port UDP tetap harus terbuka langsung ke
internet.

Karena itu perlu dua nama yang berbeda:

- **Web UI** diarahkan lewat tunnel (dikonfigurasi di dasbor Cloudflare Zero
  Trust, bukan sebagai A record).
- **Endpoint VPN** perlu A record tersendiri yang menunjuk ke IP VPS dengan
  **proxy dimatikan** (awan abu-abu). Nama inilah yang diisi di halaman VPN.
  Memakai nama yang di-proxy akan menunjuk ke server Cloudflare, dan tidak ada
  satu pun site yang bisa terhubung.

## Peta Jaringan

Peta jaringan (`/network-map`) menggambar seluruh infrastruktur di atas citra
satelit — server/OLT, ODC, ODP, dan ONT sebagai pin berwarna beda per jenis,
disambungkan dengan jalur kabel yang ditelusuri titik demi titik. Halaman ini
menggantikan peta OLT yang lama sepenuhnya; **OLT tidak lagi punya halaman peta
sendiri**. Sebuah OLT muncul di sini sebagai node bertipe **server**, ditempatkan
manual sama seperti ODC/ODP/ONT — bukan digambar otomatis dari data OLT yang
tersimpan di halaman OLTs.

### Setup awal

1. Di Google Cloud Console, buat project dan aktifkan billing. Places
   autocomplete memerlukan billing bahkan dalam kuota gratis bulanan.
2. Aktifkan **Maps JavaScript API** dan **Places API (New)**.

   Perhatikan kata **(New)**. Google punya dua layanan bernama mirip, dan
   autocomplete alamat di TikMan hanya bicara dengan yang (New) — yang lama
   sudah tidak diberikan Google ke project baru sejak Maret 2025. Kalau yang
   diaktifkan yang salah, kolom alamat tetap muncul tapi tidak pernah
   memberi saran.
3. Buat API key.
4. Batasi: Application restrictions → Websites → tambahkan
   `https://<domain-noc-anda>/*`; API restrictions → hanya **Maps JavaScript API**
   dan **Places API (New)**.

   Sekali lagi **(New)**. Memilih "Places API" yang lama di sini membuat key
   menolak permintaan autocomplete dengan `API_KEY_SERVICE_BLOCKED`, yang hanya
   terlihat di console browser — di layar, kolomnya tampak normal tapi diam.
5. Di TikMan, buka **Settings** dan simpan key di **Google Maps API key**.

Langkah 4 tidak opsional. Key dikirimkan ke setiap browser yang membuka peta
dan dapat dibaca dari developer tools; pembatasan adalah satu-satunya hal yang
mencegah orang lain menghabiskan kuota Anda.

### Menempatkan node

Dari tampilan **Peta**, pilih salah satu dari empat tombol jenis node
(Server/OLT, ODC, ODP, ONT) lalu klik di peta untuk menempatkannya di titik
itu. Form yang terbuka meminta nama, dan untuk ODC/ODP jumlah slot serta rasio
splitter — kapasitas boleh dikosongkan (berarti tidak terbatas; ODP tanpa
kapasitas tetap bisa dipasangi ONT di port berapa pun, backend menerimanya).

Posisi terisi otomatis dari titik yang diklik dan, secara bawaan, hanya-baca.
Untuk mengoreksi posisi — baik sedang menempatkan node baru maupun mengedit
node yang sudah ada lewat Daftar — centang **"Isi koordinat manual"**:
latitude dan longitude menjadi bisa diketik, dan diperiksa terhadap rentang
sah (latitude -90 s.d. 90, longitude -180 s.d. 180) sebelum bisa disimpan.
Ini satu-satunya cara memindahkan node yang salah tempat; menghapus dan
menempatkan ulang akan melepaskan setiap kabel yang sudah tersambung ke sana.

### Menelusuri kabel

Tombol **Tarik kabel** mengunci mode menyambung. Klik satu node sebagai
sumber, lalu — bila rute kabel di lapangan berbelok — klik titik-titik
belokan di peta, dan tutup dengan klik pada node tujuan. Satu klik titik yang
salah dibatalkan satu titik dengan tombol **Batal titik**, bukan seluruh
jalur yang sudah ditelusuri.

Setelah node tujuan diklik, pilih salah satu dari tujuh jenis serat (feeder,
distribusi, drop, ODP ke ODP, ODP ke ODP splitter, ODC ke ODC, ODC ke ODC
splitter) sebelum kabel tersimpan. Panjangnya dihitung dari jalur yang
benar-benar ditelusuri — sumber, tiap belokan, tujuan — bukan garis lurus
antar kedua ujungnya.

ODP ke ODP dan ODC ke ODC punya aturan kapasitas sendiri: menyambung kabel ke
kotak yang slotnya sudah penuh ditolak dan pesannya menyebutkan kapasitasnya,
mis. `"ODC-X" sudah penuh (8/8)` — beda dari pesan "kabel sudah ada", yang
berarti pasangan node yang sama sudah pernah disambung sebelumnya.

### Kartu jumlah dan Daftar

Di bawah peta ada kartu ringkas yang menghitung tiap jenis node (Server/OLT,
ODC, ODP, ONT) — cara cepat memastikan sebuah sesi pemetaan lapangan benar-
benar tersimpan.

Tampilan **Daftar** menyajikan node dan kabel sebagai dua tabel terpisah.
Tiap baris punya tombol **Ubah**, yang membuka form yang sama dengan
penempatan tapi terisi data tersimpan, dan **Hapus**, yang meminta
konfirmasi sebelum benar-benar menghapus. Menghapus node yang masih ditunjuk
oleh satu atau lebih ONT ditolak, dengan pesan yang menyebutkan berapa ONT
yang masih memakainya — node itu harus dilepaskan dari ONT-ONT tersebut
lebih dulu.

## Memantau router MikroTik border

TikMan membaca router lewat SNMP (trafik interface dan VLAN) dan, untuk status
BGP, daftar interface, dan CPU/RAM, lewat REST API, API-SSL, atau API RouterOS v7
(dipilih per router). Aksesnya hanya baca.
Jalankan perintah berikut di router, ganti `IP_VPS`, `IP_ROUTER`, `PASSWORD`,
dan `COMMUNITY`:

```
/user group add name=tikman-ro policy=read,api,rest-api
/user add name=tikman group=tikman-ro password="PASSWORD" address=IP_VPS/32
/certificate add name=tikman-rest common-name=IP_ROUTER subject-alt-name=IP:IP_ROUTER days-valid=3650 key-usage=digital-signature,key-encipherment,tls-server
/certificate sign tikman-rest
/ip service set www-ssl certificate=tikman-rest port=8443 address=IP_VPS/32 disabled=no
/snmp community add name="COMMUNITY" addresses=IP_VPS/32 read-access=yes write-access=no
```

REST memakai port 8443 karena 443 biasanya dipakai `reverse-proxy` RouterOS 7.
Bila chain input punya rule drop, izinkan port API yang dipilih (tcp/8443
REST, tcp/8729 API-SSL, atau tcp/8728 API) dan udp/161 dari `IP_VPS` di
atasnya. Batasi juga `addresses` setiap community SNMP lain ke pemakainya:
community yang terbuka ke `::/0` bisa dibaca siapa saja dan dipakai untuk
serangan amplifikasi.

### Memilih protokol

Di form router, **Protokol** menentukan cara TikMan membaca BGP, daftar
interface, dan CPU/RAM. SNMP tetap dipakai untuk trafik di ketiganya.

| Protokol | Service router | Port default | Sertifikat |
|---|---|---|---|
| REST (disarankan) | `www-ssl` | 8443 | wajib, fingerprint dicocokkan |
| API-SSL | `api-ssl` | 8729 | wajib, fingerprint dicocokkan |
| API | `api` | 8728 | tidak ada — password lewat tanpa enkripsi |

Untuk **API-SSL**, pakai sertifikat yang sama seperti REST:

```
/ip service set api-ssl certificate=tikman-rest address=IP_VPS/32 disabled=no
```

`api-ssl` dengan `certificate=none` tidak bisa dipakai: RouterOS lalu hanya
menawarkan cipher anonim, yang ditolak TikMan karena tidak melindungi dari
penyadapan.

Untuk **API**, cukup batasi service-nya ke VPS:

```
/ip service set api address=IP_VPS/32 disabled=no
```

API mengirim username dan password tanpa enkripsi, jadi siapa pun di jalur
antara VPS dan router bisa membacanya. Pakai hanya bila REST atau API-SSL
tidak memungkinkan, dan tetap dengan user grup `tikman-ro` yang read-only.
Uji koneksi untuk API hanya satu langkah karena tidak ada sertifikat yang
dicocokkan.

Mengganti protokol router yang sudah ada dari API ke REST atau API-SSL membuat
TikMan menampilkan fingerprint sertifikat di tab Router pada poll berikutnya;
Admin menerimanya dengan **Terima** setelah mencocokkannya di router.

### Menambah router

Menambah, mengubah, dan menghapus router, serta menerima sertifikat baru,
hanya bisa dilakukan Admin. Teknisi bisa menjalankan **Refresh discovery**;
semua peran bisa melihat.

Buka **MikroTik → Tambah router**, isi data, lalu uji koneksi dua langkah:

1. Klik **Uji koneksi**. Untuk REST dan API-SSL, langkah ini hanya menguji
   SNMP dan menampilkan fingerprint sertifikat router. Password belum
   dikirim ke mana pun.
2. Cocokkan fingerprint itu dengan
   `/certificate print detail where name=tikman-rest` di router. Bila sama,
   klik **Fingerprint cocok, uji REST** (atau **uji API-SSL**). TikMan lalu
   menguji protokol itu hanya terhadap sertifikat tersebut.

Untuk API, uji koneksi langsung login dan membaca board dalam satu langkah.

Saat menambah router, tombol **Simpan** baru aktif setelah kedua langkah lulus
(untuk API, satu langkah sudah cukup); mengubah router yang sudah ada tidak
memakai uji ini. Kalau sertifikat router diganti, TikMan berhenti membaca BGP
dan menampilkan fingerprint baru di tab Router sampai Admin menerimanya.

### Tujuan trafik (NetFlow)

Tab **Tujuan Trafik** menunjukkan ke mana trafik pelanggan pergi —
organisasi/AS tujuan, dari POP mana, lewat upstream mana, ke arah mana — dari
15 menit terakhir sampai setahun, atau rentang bebas. Datanya IPFIX yang
dikirim router border ke service `flowd` di VPS (UDP 2055).

Setiap POP me-NAT pelanggannya sendiri, jadi di border yang terlihat adalah
IP publik POP, bukan IP pelanggan: data ini per POP, tidak per pelanggan.

#### Mengaktifkan NetFlow

Di router, mulai dengan sampling 1:100 (ganti `IP_VPS` dan `IP_ROUTER`):

```
/ip traffic-flow set enabled=yes interfaces=all active-flow-timeout=1m inactive-flow-timeout=15s packet-sampling=yes sampling-interval=1 sampling-space=99
/ip traffic-flow target add dst-address=IP_VPS port=2055 version=ipfix src-address=IP_ROUTER
```

- `src-address` harus sama dengan IP router di TikMan. `flowd` mengenali
  router dari alamat pengirim dan membuang paket dari alamat lain; karena itu
  router harus didaftarkan dengan IP, bukan hostname.
- **Host** router di TikMan harus alamat publik tempat IPFIX router sampai ke
  VPS, yaitu alamat yang sama dengan `src-address`. `flowd` mendengarkan
  `2055/udp` di alamat publik VPS, di luar namespace WireGuard, jadi router yang
  didaftarkan dengan alamat tunnel/privat belum bisa memakai NetFlow sampai ada
  ekspor tunnel yang terautentikasi.
- `active-flow-timeout=1m`: tanpa ini RouterOS mengirim flow yang panjang
  setelah 30 menit sekaligus, dan grafik menampilkannya sebagai lonjakan.
- Rasio sampling = (sampling-interval + sampling-space) / sampling-interval;
  contoh di atas 1:100. Pantau CPU router setelah mengaktifkannya, dan naikkan
  rasionya bila CPU tinggi.

Di TikMan (Admin): buka router → **Ubah** → nyalakan **Kumpulkan NetFlow** dan
isi **Rasio sampling (1:N)** dengan N yang sama persis dengan router. Rasio
yang berbeda membuat setiap angka meleset sebesar selisihnya.

Di VPS: buka `2055/udp` di firewall provider, dibatasi ke IP router bila
memungkinkan.

IPFIX lewat UDP tidak mengautentikasi pengirim. Host yang bisa memalsukan
alamat sumber router dapat menyuntikkan atau mengubah angka flow (dibatasi per
menit), atau sebentar mengganggu pembacaan sampai router mengirim ulang
template-nya. Karena itu data ini untuk pemantauan dan perencanaan, bukan
penagihan. Jalur ekspor yang terautentikasi, misalnya lewat tunnel, akan
menjadi perubahan tersendiri.

#### Memulihkan rollup setelah database mati

Rollup dihitung ulang dari menit mentah, dan menit mentah dihapus setelah 7
hari. Bila database atau job TimescaleDB mati lebih lama dari jendela refresh
sebuah rollup, jalankan perintah berikut dalam 7 hari, sebelum menit mentahnya
hilang:

```sql
CALL refresh_continuous_aggregate('router_flow_5min', now() - interval '7 days', now() - interval '1 hour');
CALL refresh_continuous_aggregate('router_flow_hourly', now() - interval '7 days', now() - interval '1 hour');
CALL refresh_continuous_aggregate('router_flow_daily', now() - interval '7 days', now() - interval '1 day');
```

#### Memastikan data masuk

Tab **Router** → bagian **NetFlow** menampilkan **Flow terakhir diterima**
(diperbarui tiap menit) dan **Dataset ASN** yang dipakai. Bila NetFlow aktif
tetapi lebih dari 5 menit tidak ada flow, muncul peringatan. Cek kewajaran:
total download di Tujuan Trafik selama satu jam kira-kira sama dengan grafik
download upstream di tab **Upstream & BGP**; selisih besar hampir selalu
berarti rasio sampling di TikMan tidak sama dengan di router. Untuk
membandingkan satu hari penuh, bandingkan total upstream hari itu di Tujuan
Trafik dengan grafik SNMP upstream untuk hari yang sama.

#### Membaca datanya

- POP dikenali dari alamat di interface POP dan dari rute statis ke router POP,
  keduanya diambil saat discovery (tiap 6 jam, atau segera lewat **Refresh
  discovery**). Blok baru dikenali paling lama 5 menit setelah discovery;
  sebelum itu trafiknya masuk **Lainnya (di luar blok POP)**.
- Upstream adalah interface yang membawa sesi BGP.
- **Lainnya** di daftar tujuan berarti AS di luar 20 teratas per POP per
  menit, atau alamat yang tidak ada di dataset ASN.
- IP tujuan hanya disimpan 7 hari. Ringkasan per AS disimpan per menit selama
  7 hari, per 5 menit 90 hari, per jam 2 tahun, dan per hari selamanya.
- Dataset ASN dari iptoasn.com diunduh ulang tiap 24 jam; bila gagal, dataset
  sebelumnya tetap dipakai.
