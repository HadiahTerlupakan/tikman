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
| `locked` | Sudah di-rollback ke snapshot sebelumnya |

### Status Batch
| Status | Arti |
|--------|------|
| `success` | Semua ONT berhasil |
| `failed` | Ada yang gagal, semua yang sukses di-rollback |
| `partial_rollback` | Sebagian ONT di-rollback |

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

## Batch Provisioning

### Peringatan
> Batch provisioning bersifat **all-or-nothing**: jika satu ONT gagal, semua ONT lain yang sudah sukses akan di-rollback otomatis.

### Via API (UI picker belum tersedia)
```bash
curl -X POST http://localhost:8080/api/v1/batch-provision \
  -H "Content-Type: application/json" \
  -b "cookies.txt" \
  -d '{
    "template_id": "<uuid>",
    "ont_ids": ["<uuid1>", "<uuid2>"],
    "confirm": true
  }'
```

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
