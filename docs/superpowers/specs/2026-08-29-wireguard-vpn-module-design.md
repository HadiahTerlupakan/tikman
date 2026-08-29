# Modul VPN WireGuard — Desain

Tanggal: 2026-08-29
Status: disetujui, siap dibuatkan rencana implementasi

## 1. Konteks dan tujuan

TikMan akan dijalankan di sebuah VPS dengan Docker Compose. Sebagian besar site
tidak punya IP publik: OLT di sana hanya bisa dihubungi dari dalam jaringan
lokal site. Akibatnya `api` dan `worker` yang berjalan di VPS tidak punya jalan
untuk melakukan ping, SSH/Telnet, maupun SNMP ke OLT tersebut.

Modul ini menjadikan VPS sebagai server WireGuard. Tiap site memasang satu peer
yang menghubungi VPS dari balik NAT, dan sesudah tunnel berdiri, `api` dan
`worker` dapat menjangkau subnet lokal site seperti jaringan biasa.

Sasaran kegunaan: operator TikMan tidak perlu memahami WireGuard. Ia mengisi
sedikit data di TikMan, menyalin satu blok konfigurasi ke perangkat di site,
dan selesai.

## 2. Ruang lingkup

Masuk versi pertama:

- Konfigurasi server WireGuard sekali jalan (keypair dibuat otomatis).
- CRUD peer, satu peer per site.
- Alokasi alamat tunnel otomatis.
- Saran subnet site otomatis dari alamat OLT yang sudah terdaftar.
- Penerapan konfigurasi ke interface kernel dan rute, dengan rekonsiliasi penuh.
- Pembacaan status handshake berkala dan penyajiannya dalam bahasa manusia.
- Generator konfigurasi sisi site untuk MikroTik dan Linux (wg-quick).
- Worker melewati polling site yang tunnelnya mati.
- Audit log untuk setiap perubahan dan setiap pengambilan konfigurasi.

Tidak masuk versi pertama:

- Interface atau server WireGuard lebih dari satu.
- Rotasi kunci terjadwal.
- Penyimpanan trafik tunnel ke TimescaleDB dan grafiknya.
- Pengiriman konfigurasi otomatis ke MikroTik lewat API perangkat.

## 3. Arsitektur dan deployment

Interface WireGuard berada di dalam network namespace container `api`:

- `api` mendapat `cap_add: NET_ADMIN` dan akses `/dev/net/tun`.
- `worker` diubah menjadi `network_mode: service:api` sehingga otomatis berbagi
  namespace yang sama dan melihat rute tunnel tanpa privilege tambahan.
- Port UDP WireGuard dipetakan pada service `api`.

Alasan memilih bentuk ini dibanding container gateway terpisah atau WireGuard
userspace: seluruh paket `internal/connectivity` tidak berubah sama sekali.
Kode SSH, Telnet, SNMP, dan ping tetap melakukan dial ke `OLT.IPAddress`, dan
kernel yang menentukan trafik itu lewat `wg0`. Paket itu justru bagian yang
paling sulit diverifikasi tanpa perangkat asli, sehingga membiarkannya utuh
adalah keputusan pengurangan risiko, bukan penghematan usaha.

Konsekuensi yang diterima: restart `api` ikut mematikan `worker`. Dengan
`restart: unless-stopped` worker naik kembali sendiri, dengan jeda beberapa
detik pada setiap deploy `api`.

VPS tidak perlu mengaktifkan IP forwarding. Trafik ke site berasal dari proses
di dalam namespace itu sendiri, bukan diteruskan dari jaringan lain.

Dependensi baru, keduanya dipin versinya:

- `golang.zx2c4.com/wireguard/wgctrl` — konfigurasi kunci dan peer.
- `github.com/vishvananda/netlink` — pembuatan interface, alamat, dan rute.

## 4. Model data

Migration baru: `backend/migrations/25_add_wireguard.sql`.

### `wireguard_server`

Satu baris. Menyimpan identitas server di VPS.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `interface_name` | varchar(15), unique | default `wg0` |
| `listen_port` | int | default 51820 |
| `private_key` | text | terenkripsi AES-256-GCM |
| `public_key` | text | |
| `endpoint_host` | varchar(255) | alamat publik VPS untuk konfigurasi sisi site |
| `tunnel_subnet` | varchar(45) | default `10.88.0.0/24` |
| `address` | varchar(45) | alamat server di tunnel, default `10.88.0.1` |
| `created_at`, `updated_at` | timestamptz | |

Keypair dibuat aplikasi saat baris pertama dibuat, sehingga private key tidak
pernah melewati input pengguna.

Tipe kolom mengikuti pola yang sudah dipakai repo: alamat disimpan sebagai
`varchar(45)` seperti `OLT.IPAddress`, dan daftar disimpan sebagai `jsonb`
lewat `datatypes.JSON` seperti `OLT.VLANs`. Tipe khusus PostgreSQL seperti
`inet` dan `text[]` dihindari karena test backend memakai SQLite in-memory
melalui `models.AutoMigrate`.

### `wireguard_peers`

Satu peer per site.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `site_id` | uuid, unique, FK `sites(id)` ON DELETE RESTRICT | |
| `name` | varchar(255) | |
| `public_key` | text | |
| `private_key` | text | terenkripsi; disimpan agar konfigurasi sisi site dapat diambil ulang |
| `preshared_key` | text, nullable | terenkripsi |
| `tunnel_address` | varchar(45), unique | dialokasikan otomatis |
| `allowed_ips` | jsonb | daftar subnet lokal site yang boleh dijangkau |
| `persistent_keepalive` | int | default 25 |
| `enabled` | boolean | default true |
| `last_handshake_at` | timestamptz, nullable | hasil pembacaan kernel |
| `rx_bytes`, `tx_bytes` | bigint | hasil pembacaan kernel |
| `created_at`, `updated_at` | timestamptz | |

`ON DELETE RESTRICT` disengaja: menghapus site yang masih punya tunnel harus
gagal dengan pesan jelas, bukan diam-diam memutus akses.

Peer tidak menyimpan endpoint. Site berada di balik NAT dan yang menginisiasi
koneksi, sehingga VPS mempelajari alamat site dari handshake.

Model `OLT` dan `Site` tidak berubah. Tidak ada kolom "lewat tunnel mana" yang
harus dijaga konsistensinya; keputusan routing sepenuhnya berasal dari
`allowed_ips`.

## 5. Alur runtime

### Penulis tunggal

Hanya proses `api` yang menyentuh kernel. `worker` membaca status tunnel dari
kolom `last_handshake_at` di database. Dengan begitu tidak ada dua proses yang
saling menimpa konfigurasi device, dan `worker` tidak memerlukan privilege.

### Rekonsiliasi

Database adalah sumber kebenaran. `WireGuardService.Reconcile(ctx)` melakukan,
secara berurutan:

1. Pastikan interface ada, punya alamat server, dan berstatus up.
2. Baca seluruh peer `enabled` dari database.
3. Satu panggilan `ConfigureDevice` dengan `ReplacePeers: true`.
4. Sinkronkan rute: setiap CIDR pada `allowed_ips` diarahkan ke interface, dan
   rute milik modul ini yang tidak lagi ada di database dihapus.

`Reconcile` dipanggil saat `api` start dan sesudah setiap perubahan peer atau
konfigurasi server. Tidak ada jalur kode "tambahkan satu peer" yang terpisah,
sehingga keadaan kernel tidak dapat menyimpang dari database walaupun container
di-restart atau seseorang mengubah `wg0` dari shell.

### Pembaruan status

Satu goroutine di `api` membaca device setiap 30 detik dan menulis
`last_handshake_at`, `rx_bytes`, `tx_bytes`. Peer dianggap terhubung bila
handshake terakhir kurang dari 3 menit yang lalu; WireGuard melakukan
rehandshake sekitar tiap 2 menit, sehingga ambang itu memberi satu siklus
kelonggaran.

### Perilaku worker

Sebelum melakukan polling sebuah OLT, worker memeriksa apakah site OLT tersebut
punya peer `enabled`. Bila ada dan peer itu tidak terhubung, siklus polling
untuk OLT tersebut dilewati tanpa mengubah status OLT maupun ONT, dengan satu
baris log. Site tanpa peer diperlakukan seperti sekarang.

Ini melanjutkan perbaikan pada commit `9d8c9ee`: satu tunnel yang putus tidak
boleh menghasilkan gelombang alarm palsu untuk seluruh ONT di site itu.

## 6. Validasi

Validasi berikut berjalan sebelum peer disimpan dan sebelum `Reconcile`:

- `allowed_ips` tidak boleh memuat `0.0.0.0/0` atau `::/0`. Bila lolos, seluruh
  trafik keluar VPS akan masuk ke tunnel satu site dan TikMan terputus dari
  internet.
- `allowed_ips` antar-peer tidak boleh tumpang tindih. WireGuard memilih peer
  berdasarkan allowed-ips, sehingga dua site yang sama-sama memakai subnet
  seragam seperti `192.168.1.0/24` membuat routing ambigu dan salah alamat
  tanpa error. Pesan kesalahan menyebut nama site yang bentrok.
- `allowed_ips` tidak boleh tumpang tindih dengan `tunnel_subnet` maupun subnet
  jaringan Docker yang dipakai container.
- `tunnel_address` harus berada di dalam `tunnel_subnet`, tidak sama dengan
  alamat server, dan unik antar-peer.
- `persistent_keepalive` harus di antara 10 dan 120 detik.

## 7. API dan RBAC

Grup route baru mengikuti pola `router.go` yang ada.

| Endpoint | Akses | Keterangan |
|---|---|---|
| `GET /api/v1/wireguard/server` | semua role | tanpa private key |
| `PUT /api/v1/wireguard/server` | Admin | endpoint publik, port, subnet tunnel |
| `GET /api/v1/wireguard/peers` | semua role | daftar peer beserta status |
| `POST /api/v1/wireguard/peers` | Admin | keypair dibuat server-side |
| `PUT /api/v1/wireguard/peers/:id` | Admin | |
| `DELETE /api/v1/wireguard/peers/:id` | Admin | |
| `GET /api/v1/wireguard/peers/:id/config` | Admin | parameter `format=wg-quick\|mikrotik` |
| `GET /api/v1/wireguard/sites/:site_id/suggested-subnets` | semua role | subnet yang disarankan dari alamat OLT site |

Seluruh mutasi dibatasi Admin, bukan Admin dan Technician seperti pada OLT.
Peer VPN adalah jalan masuk ke jaringan pelanggan, dan `allowed_ips` yang salah
dapat memutus rute site lain.

## 8. Generator konfigurasi sisi site

Keluaran harus dapat ditempel sekali tanpa penyuntingan. Nilai di bawah ini
adalah contoh untuk site dengan subnet lokal `10.10.10.0/24` dan alamat tunnel
`10.88.0.5`.

### wg-quick (Linux)

```
[Interface]
PrivateKey = <private key peer>
Address = 10.88.0.5/24
PostUp = iptables -t nat -A POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE

[Peer]
PublicKey = <public key server>
Endpoint = vpn.contoh.id:51820
AllowedIPs = 10.88.0.0/24
PersistentKeepalive = 25
```

### MikroTik

```
/interface/wireguard/add name=wg-tikman private-key="<private key peer>" listen-port=13231
/ip/address/add address=10.88.0.5/24 interface=wg-tikman
/interface/wireguard/peers/add interface=wg-tikman public-key="<public key server>" \
    endpoint-address=vpn.contoh.id endpoint-port=51820 allowed-address=10.88.0.0/24 \
    persistent-keepalive=25s
/ip/firewall/nat/add chain=srcnat src-address=10.88.0.0/24 dst-address=10.10.10.0/24 \
    action=masquerade comment="TikMan VPN"
```

Baris masquerade adalah bagian yang membuat pemasangan cukup sekali tempel.
Tanpa itu OLT harus punya rute balik ke subnet tunnel, dan pada kebanyakan
pemasangan OLT tidak memilikinya: tunnel akan terlihat hidup sementara OLT
tetap tidak terjangkau. Aturan NAT ditulis dengan `src-address` dan
`dst-address` dan tanpa nama interface, sehingga tidak menuntut operator
mengetahui nama interface LAN di perangkatnya.

`AllowedIPs` di sisi site sengaja hanya subnet tunnel, bukan `0.0.0.0/0`, agar
trafik internet site tidak ikut dialihkan ke VPS.

## 9. Frontend

Halaman baru `VpnPage.tsx` dengan dua bagian.

Kartu server menampilkan alamat publik, port, public key, dan subnet tunnel.
Saat belum ada konfigurasi, halaman menampilkan formulir sekali jalan berisi dua
isian: alamat publik VPS, yang sudah terisi otomatis dari alamat yang dipakai
mengakses TikMan, dan port UDP dengan nilai bawaan 51820. Keypair dibuat di
latar tanpa ditanyakan.

Tabel peer memuat nama site, alamat tunnel, subnet site, dan status. Status
ditulis sebagai "Terhubung" atau "Tidak terhubung sejak 12 menit lalu", disertai
petunjuk singkat apa yang perlu diperiksa, bukan angka handshake mentah.

Formulir peer hanya meminta site dan subnet lokal site. Subnet diisi otomatis
sebagai saran yang diturunkan dari alamat OLT yang sudah terdaftar pada site
tersebut: OLT `10.10.10.5` menghasilkan saran `10.10.10.0/24`. Alamat tunnel
dialokasikan otomatis dan hanya tampil pada bagian lanjutan yang terlipat.

Aksi per baris: sunting, hapus, dan "Unduh konfigurasi" yang
membuka modal dengan tab MikroTik dan Linux, teks siap salin, serta peringatan
bahwa isinya memuat private key.

Struktur mengikuti yang sudah ada: entity `domain/entities/WireguardPeer.ts`,
repository di `infrastructure/repositories`, hook `useWireguard` di
`application/hooks`, route di `presentation/routes/index.tsx`, dan butir menu di
`components/layout/Sidebar.tsx`.

## 10. Keamanan

- Private key server, private key peer, dan preshared key dienkripsi
  AES-256-GCM melalui `internal/utils`, mekanisme yang sama dengan password OLT.
- Private key tidak pernah muncul pada response daftar maupun detail. Satu-satunya
  jalan keluarnya adalah endpoint konfigurasi, yang khusus Admin.
- Setiap pembuatan, perubahan, penghapusan peer, dan setiap pengambilan
  konfigurasi dicatat melalui `AuditService`.
- Port UDP WireGuard harus dibuka pada firewall penyedia VPS. Ini di luar kode
  dan dicatat pada dokumentasi deploy yang sudah ada.

## 11. Pembagian berkas

Backend:

- `internal/models/wireguard.go` — dua model dan hook UUID.
- `internal/services/wireguard_service.go` — siklus hidup server dan `Reconcile`.
- `internal/services/wireguard_peers.go` — CRUD peer dan render konfigurasinya. Dipisah
  karena satu berkas untuk keduanya melewati batas 350 baris.
- `internal/services/wireguard_validate.go` — seluruh aturan pada bagian 6.
- `internal/services/wireguard_alloc.go` — alokasi alamat tunnel dan saran
  subnet dari alamat OLT.
- `internal/services/wireguard_render.go` — generator kedua format konfigurasi.
- `internal/services/wireguard_status.go` — aturan "terhubung".
- `internal/services/wireguard_refresher.go` — goroutine pembaruan status.
- `internal/connectivity/wireguard_device.go` — tipe dan interface
  `TunnelDevice`, batas antara keputusan dan kernel. Berada di `connectivity`
  dan bukan `services` karena `services` sudah mengimpor `connectivity`;
  arah sebaliknya akan menjadi import cycle.
- `internal/connectivity/wireguard_device_memory.go` — implementasi in-memory
  yang dipakai test.
- `internal/connectivity/wireguard_device_linux.go` dan
  `wireguard_device_other.go` — lapisan tipis yang menyentuh netlink dan
  wgctrl. Dipisah build tag karena `netlink` hanya dapat dibangun di Linux
  sementara pengembangan berjalan di macOS.
- `internal/api/wireguard_handler.go`, `internal/api/wireguard_dto.go`.

Lapisan yang menyentuh kernel sengaja dipisah dan dibuat setipis mungkin,
sejalan dengan pengecualian kode network-bound pada CLAUDE.md. Semua keputusan
berada di kode yang dapat diuji.

Frontend: `presentation/pages/VpnPage.tsx` beserta komponen tabel, formulir
peer, dan modal konfigurasi yang dipisah agar tiap berkas tetap di bawah batas
ukuran.

## 12. Pengujian

Diuji dengan test otomatis:

- Setiap aturan validasi pada bagian 6, termasuk deteksi tumpang tindih antar
  peer dan pesan yang menyebut site yang bentrok.
- Alokasi alamat tunnel, termasuk saat ada lubang di tengah rentang dan saat
  subnet habis.
- Penurunan saran subnet dari alamat OLT, termasuk site tanpa OLT.
- Kedua generator konfigurasi, dibandingkan dengan keluaran contoh yang benar.
- Aturan "terhubung" terhadap `last_handshake_at`, termasuk nilai kosong.
- Keputusan worker melewati site yang tunnelnya mati, dan tetap melakukan
  polling untuk site tanpa peer.
- Handler beserta RBAC-nya memakai SQLite in-memory seperti test yang ada,
  termasuk pembuktian bahwa private key tidak muncul pada response daftar.

Tidak diuji otomatis: pemanggilan netlink dan wgctrl yang sesungguhnya, karena
memerlukan kernel dengan privilege. Bagian itu diverifikasi lewat build dan
pemasangan pada satu site nyata.

## 13. Batas yang diketahui

Di tiap site tetap dibutuhkan satu perangkat MikroTik atau Linux yang menerima
tempelan konfigurasi satu kali. TikMan tidak dapat memasang dirinya pada
perangkat yang belum pernah terhubung.
