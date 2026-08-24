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
