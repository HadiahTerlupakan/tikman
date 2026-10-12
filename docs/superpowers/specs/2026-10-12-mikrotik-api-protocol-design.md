# MikroTik: pilihan protokol REST / API-SSL / API

Tanggal: 2026-10-12
Status: disetujui di percakapan, menunggu review spec tertulis
Melanjutkan: `2026-10-12-mikrotik-monitoring-design.md` (sub-proyek 1)

## Latar belakang

Sub-proyek 1 membaca BGP, resource, clock, dan inventaris router lewat REST
API RouterOS v7, yang hanya dilayani `www-ssl` dan membutuhkan sertifikat di
router. Operator ingin bisa memilih, per router, untuk memakai RouterOS API
biner (`api`, TCP tanpa enkripsi) atau `api-ssl` (TLS), karena port API sering
sudah aktif sementara `www-ssl` belum disiapkan.

Yang tidak berubah: SNMP tetap untuk counter interface; poller, discovery,
BGP, rollup, dan UI grafik tidak tahu protokol mana yang dipakai.

## Keputusan

| Hal | Keputusan |
|---|---|
| Pilihan | `rest` (default), `api-ssl`, `api` — per router |
| Klien API | ditulis sendiri dengan stdlib, tanpa dependency baru |
| Fingerprint | wajib dan di-pin untuk `rest` dan `api-ssl`; tidak dipakai untuk `api` |
| Port default di form | `rest` 8443, `api-ssl` 8729, `api` 8728; bebas diubah |
| Perintah | hanya baca (`print`); tidak ada perintah tulis |
| RouterOS | v7 saja (login API pasca-6.43: nama dan password dalam satu `/login`) |

`api-ssl` dengan `certificate=none` di router memakai cipher anonim (ADH),
yang tidak didukung `crypto/tls` Go dan memang tidak aman terhadap
man-in-the-middle. Karena itu `api-ssl` tetap butuh sertifikat di router; UI
dan panduan operator menyebutnya.

`api` mengirim username dan password tanpa enkripsi. UI menampilkan
peringatan, dan panduan operator meminta `address=` service `api` dibatasi ke
IP VPS. User router tetap grup read-only.

## Arsitektur

`internal/mikrotik` dipisah menjadi dua lapis supaya fitur baca berikutnya
cukup satu method dan langsung jalan di ketiga protokol.

**Transport** — interface kecil:

```go
type Transport interface {
    // List reads every row under a RouterOS menu path such as
    // "/routing/bgp/session" into out (a pointer to a struct or slice of
    // structs whose json tags are RouterOS property names).
    List(path string, out interface{}) error
    Close() error
}
```

- `restTransport` — klien REST yang sudah ada, dipindah di belakang
  interface ini. `List` = `GET /rest<path>`. `Close` tidak melakukan apa-apa
  (koneksi sudah tanpa keep-alive).
- `apiTransport` — RouterOS API. `List` mengirim `<path>/print`, mengumpulkan
  setiap `!re` sebagai `map[string]string`, lalu mengisi `out` lewat json tag
  yang sama (nilai API, seperti REST, semuanya string, dan nama propertinya
  sama: `remote.address`, `cpu-load`, ...). Satu koneksi TCP per klien; login
  sekali saat dibuka; `Close` menutup koneksi.

Path dengan satu objek (`/system/resource`, `/system/clock`): REST
mengembalikan objek, API mengembalikan satu `!re`. `apiTransport` mengisi
struct dari baris pertama bila `out` bukan slice.

**Reader** — `Client` membungkus sebuah `Transport` dan memegang method yang
sudah ada (`SystemResource`, `Clock`, `Interfaces`, `VLANs`, `Addresses`,
`BGPConnections`, `BGPSessions`), masing-masing satu panggilan `List`. Tipe di
`types.go` dan parser di `parse.go` tidak berubah. `Client` menambah `Close`.

**Konstruktor tunggal:**

```go
type Protocol string // "rest", "api-ssl", "api"

type Config struct {
    Protocol    Protocol
    Host        string
    Port        int
    Username    string
    Password    string
    Fingerprint string // diabaikan untuk "api"
}

func Open(cfg Config) (*Client, error)
```

`Open` untuk `api`/`api-ssl` langsung dial dan login, sehingga kredensial yang
ditolak muncul sebagai `ErrUnauthorized` di sini. Untuk `rest` tidak ada I/O
sampai panggilan pertama. `FetchFingerprint` tetap dan dipakai untuk `rest`
maupun `api-ssl` (keduanya TLS).

### Protokol API (yang diimplementasikan)

- Kata = panjang + byte. Encoding panjang: `< 0x80` 1 byte; `< 0x4000` 2 byte
  (`| 0x8000`); `< 0x200000` 3 byte (`| 0xC00000`); `< 0x10000000` 4 byte
  (`| 0xE0000000`); selain itu `0xF0` + 4 byte. Kalimat diakhiri kata kosong.
- Login: `/login`, `=name=<user>`, `=password=<pass>` → `!done`. Balasan
  `!trap` saat login → `ErrUnauthorized`.
- Balasan perintah: nol atau lebih `!re` lalu `!done`. `!trap` →
  error dengan `=message=`; `!fatal` → error dan koneksi dianggap mati.
  `!empty` (v7.18+) diperlakukan sama dengan nol baris.
- Atribut `=key=value`; nilai boleh berisi `=`, hanya `=` kedua yang memisah.
  Kata `.tag` dan atribut `.id` diabaikan.
- Deadline per panggilan 5 detik (sama dengan `requestTimeout` REST), juga
  untuk dial dan login. Panjang kata di atas 1 MiB ditolak sebagai balasan
  rusak.
- `api-ssl` memakai `pinnedTLS(fingerprint)` yang sama dengan REST; sertifikat
  yang berbeda → `ErrCertificateChanged`.

## Data

Migrasi `57_mikrotik_api_protocol.sql`:

```sql
ALTER TABLE mikrotik_routers
  ADD COLUMN IF NOT EXISTS api_protocol varchar(16) NOT NULL DEFAULT 'rest';
ALTER TABLE mikrotik_routers
  ADD CONSTRAINT mikrotik_routers_api_protocol_check
  CHECK (api_protocol IN ('rest', 'api-ssl', 'api'));
```

(satu file maju tanpa `down`, seperti migrasi 56). Router yang sudah ada
menjadi `rest`, perilakunya tidak berubah. Model `MikrotikRouter` mendapat
`APIProtocol` (json `api_protocol`).

`tls_fingerprint` tetap `NOT NULL`; untuk `api` disimpan string kosong.

## Service dan API HTTP

- `RouterInput`, `ConnectionTestInput`, `RouterUpdate`, dan DTO-nya mendapat
  `api_protocol` (`oneof=rest api-ssl api`, kosong = `rest`).
- Validasi buat: fingerprint 64-hex wajib untuk `rest`/`api-ssl`; untuk `api`
  fingerprint diabaikan dan disimpan kosong.
- **Uji koneksi:**
  - `rest`/`api-ssl`: dua langkah seperti sekarang (langkah 1 membaca
    fingerprint tanpa kredensial; langkah 2 login ter-pin dan membaca board).
  - `api`: satu langkah — SNMP, lalu `Open` + `SystemResource`. Hasil tanpa
    fingerprint.
- **Poller:** `newAPI` membangun `mikrotik.Config` dari router dan memanggil
  `Open`; klien ditutup setelah poll. `recordAPIError` hanya membaca
  `presented_fingerprint` untuk protokol ber-TLS.
- **Ubah router (Admin):** `api_protocol` boleh diubah. Pindah ke protokol
  ber-TLS memakai fingerprint yang tersimpan; bila tidak cocok (atau kosong,
  dari `api`), poll berikutnya mencatat `certificate_changed` dan
  `presented_fingerprint`, lalu Admin menerimanya lewat tombol **Terima**
  yang sudah ada. Tidak ada alur baru.
- Audit `create`/`update` mencatat `api_protocol`.

## UI

- `RouterFormFields`: field **Protokol** (Radio) — *REST (www-ssl) —
  disarankan* / *API-SSL* / *API (tanpa enkripsi)*. Mengganti protokol
  mengisi port default bila operator belum mengubah port secara manual.
- Saat **API** dipilih: `Alert` kuning — password user read-only dikirim
  tanpa enkripsi lewat internet; batasi service `api` di router ke IP VPS.
- Saat **API-SSL** dipilih: keterangan bahwa `api-ssl` butuh sertifikat di
  router (bukan `none`).
- `useRouterTestSteps`: untuk `api` langsung satu langkah (tidak ada layar
  konfirmasi fingerprint); Simpan aktif setelah uji itu lulus.
- Tab **Router** di halaman detail menampilkan protokol; label status
  "REST" menjadi "API" agar netral.
- Entitas, repository, dan hook frontend membawa `apiProtocol`.

## Dokumentasi

`docs/operator_guide.md` bagian MikroTik mendapat dua blok setup tambahan:
`api-ssl` (sertifikat + `/ip service set api-ssl certificate=... address=IP_VPS/32`)
dan `api` (`/ip service set api address=IP_VPS/32`, dengan peringatan
enkripsi). Contoh hanya memakai rentang dokumentasi (repo publik).

## Test

- Encoding/decoding panjang kata di setiap batas (0x7F/0x80, 0x3FFF/0x4000,
  0x1FFFFF/0x200000, 0xFFFFFFF/0x10000000).
- Server API palsu (`net.Listen` di test): login sukses; login ditolak →
  `ErrUnauthorized`; beberapa `!re` → slice terisi; satu `!re` → struct
  terisi; `!trap` → error dengan pesan; `!empty`; nilai berisi `=`;
  server diam → timeout.
- `api-ssl` dengan server TLS palsu: fingerprint cocok → terbaca; berbeda →
  `ErrCertificateChanged`.
- REST tetap lulus semua test lamanya lewat `Transport`.
- Service: uji koneksi per protokol; validasi fingerprint per protokol;
  update protokol; poller memanggil `Open` dengan protokol router.
- Migrasi: router lama terbaca sebagai `rest`; nilai di luar tiga pilihan
  ditolak constraint.
- Frontend: field protokol, peringatan API, port default mengikuti protokol
  kecuali sudah diubah, uji satu langkah untuk API.

## Di luar cakupan

Perintah tulis, RouterOS v6 (login challenge MD5), streaming `listen`/`follow`,
dan fitur baca baru — arsitektur di atas disiapkan agar fitur baca baru cukup
menambah satu method di `Client`.
