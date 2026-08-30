# Mesin Polling A1 — Kebenaran dan Kecepatan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menghapus dua plafon yang membuat polling tidak bisa naik skala — SNMP
GETNEXT dan batas 1000 ONT — lalu mengganti tulis per-ONT dengan tulis batch.

**Architecture:** Satu helper `bulkWalk` di paket `connectivity` menjadi satu-satunya
tempat GETBULK, `MaxRepetitions`, dan fallback ke GETNEXT diputuskan; 18 pemanggilan
`client.Walk` memanggilnya. Worker berhenti mengambil satu halaman ONT dan mulai
memproses per OLT, menulis metrik dan perubahan status dalam batch.

**Tech Stack:** Go 1.25, gosnmp v1.44.0, GORM, PostgreSQL/TimescaleDB.

**Spec:** `docs/superpowers/specs/2026-08-30-polling-engine-scale-design.md`

## Global Constraints

- `MaxRepetitions` default **25**, dapat diatur lewat `SNMP_MAX_REPETITIONS`. Angka
  final ditetapkan dari hasil Task 1.
- Penurunan dari GETBULK ke GETNEXT **wajib tercatat di log**. Penurunan senyap
  adalah pola kegagalan yang berulang menyembunyikan masalah di sistem ini.
- Poll yang gagal atau dilewati **tidak** menandai ONT offline; pembacaan tabel yang
  tidak tuntas **tidak** memicu prune. Keduanya sudah pernah jadi bug di repo ini.
- Berkas maksimum 350 baris, fungsi maksimum 50 baris, kecuali pengecualian yang
  tertulis di CLAUDE.md untuk kode SNMP/Telnet yang terikat jaringan.
- Tidak ada dependensi baru.
- Setiap perbaikan bug disertai tes yang gagal sebelum dan lulus sesudah.

---

### Task 1: Alat ukur GETBULK

Menjawab satu-satunya risiko yang bisa menggugurkan anggaran waktu di spec: apakah
agen SNMP ZTE sanggup melayani GETBULK, dan pada `MaxRepetitions` berapa. Dijalankan
lebih dulu karena hasilnya menentukan nilai default di Task 4.

Alat ini memuat OLT lewat kode aplikasi sendiri, sehingga community SNMP tidak
pernah keluar dari proses.

**Files:**
- Create: `backend/cmd/snmpbench/main.go`

**Interfaces:**
- Consumes: `config.Load()`, `database.Connect()`, `models.OLT`, `utils.Decrypt`
- Produces: tidak ada; keluarannya angka untuk manusia

- [ ] **Step 1: Tulis alat ukurnya**

Program menerima nama OLT lewat flag, membaca barisnya dari database,
mendekripsi community seperti yang dilakukan worker, lalu mengukur satu tabel
phase-state dengan GETNEXT dan dengan GETBULK pada beberapa nilai
`MaxRepetitions`.

```go
// cmd/snmpbench measures what an OLT's SNMP agent will actually serve.
//
// The scaling budget assumes GETBULK cuts round trips by roughly its repetition
// count. Some agents refuse large GETBULK requests, and the whole plan rests on
// which is true here — so this measures it against real hardware before any of
// the polling code is changed.
package main

func main() {
	name := flag.String("olt", "", "OLT name to measure")
	flag.Parse()
	// load config, connect DB, find OLT by name, decrypt community
	// for each mode: GETNEXT, GETBULK@10, @25, @50, @100
	//   time a full walk of OID_ZXAN_ONU_PHASE_STATE_TABLE
	//   print: mode, values returned, elapsed, values/second, error if any
}
```

- [ ] **Step 2: Jalankan terhadap Cariu**

Run: `go run ./cmd/snmpbench -olt Cariu`
Expected: satu baris per mode. GETNEXT jadi garis dasar; tiap nilai GETBULK
melaporkan jumlah nilai yang sama atau sebuah error.

- [ ] **Step 3: Catat hasilnya di spec**

Tambahkan tabel hasil ke bagian "Risiko" di spec, dan tetapkan nilai default
`SNMP_MAX_REPETITIONS` dari pengukuran itu — nilai tertinggi yang masih
mengembalikan jumlah nilai yang sama persis dengan GETNEXT.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/snmpbench docs/superpowers/specs/2026-08-30-polling-engine-scale-design.md
git commit -m "feat(snmpbench): measure what the OLT's agent will serve before trusting GETBULK"
```

---

### Task 2: Helper bulkWalk dengan fallback

**Files:**
- Create: `backend/internal/connectivity/snmp_bulkwalk.go`
- Test: `backend/internal/connectivity/snmp_bulkwalk_test.go`

**Interfaces:**
- Produces: `func bulkWalk(w snmpWalker, oid string, fn gosnmp.WalkFunc) error` dan
  `type snmpWalker interface { BulkWalk(string, gosnmp.WalkFunc) error; Walk(string, gosnmp.WalkFunc) error }`.
  `*gosnmp.GoSNMP` memenuhi `snmpWalker` tanpa perubahan.

- [ ] **Step 1: Tulis tes yang gagal**

```go
package connectivity

import (
	"errors"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

type fakeWalker struct {
	bulkErr    error
	bulkCalls  int
	walkCalls  int
	deliver    int
}

func (f *fakeWalker) BulkWalk(oid string, fn gosnmp.WalkFunc) error {
	f.bulkCalls++
	if f.bulkErr != nil {
		return f.bulkErr
	}
	for i := 0; i < f.deliver; i++ {
		if err := fn(gosnmp.SnmpPDU{Name: oid + ".1"}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeWalker) Walk(oid string, fn gosnmp.WalkFunc) error {
	f.walkCalls++
	for i := 0; i < f.deliver; i++ {
		if err := fn(gosnmp.SnmpPDU{Name: oid + ".1"}); err != nil {
			return err
		}
	}
	return nil
}

func TestBulkWalkUsesGetBulkWhenTheAgentAllowsIt(t *testing.T) {
	w := &fakeWalker{deliver: 3}
	seen := 0

	require.NoError(t, bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { seen++; return nil }))

	require.Equal(t, 1, w.bulkCalls)
	require.Equal(t, 0, w.walkCalls, "GETNEXT was used even though GETBULK worked")
	require.Equal(t, 3, seen)
}

func TestBulkWalkFallsBackWhenTheAgentRefusesGetBulk(t *testing.T) {
	// Some ZTE agents answer a large GETBULK with tooBig rather than serving it.
	// Losing the whole table over that would be worse than a slower walk.
	w := &fakeWalker{bulkErr: errors.New("request too big"), deliver: 2}
	seen := 0

	require.NoError(t, bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { seen++; return nil }))

	require.Equal(t, 1, w.bulkCalls)
	require.Equal(t, 1, w.walkCalls)
	require.Equal(t, 2, seen)
}

func TestBulkWalkDoesNotRetryWhenTheCallbackItselfFailed(t *testing.T) {
	// A decode error in the callback is our bug, not the agent's. Retrying with
	// GETNEXT would run the whole table again and fail the same way.
	w := &fakeWalker{deliver: 1}
	boom := errors.New("decode failed")

	err := bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { return boom })

	require.ErrorIs(t, err, boom)
	require.Equal(t, 0, w.walkCalls)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `go test ./internal/connectivity/ -run TestBulkWalk -count=1`
Expected: FAIL — `undefined: bulkWalk`

- [ ] **Step 3: Implementasi**

```go
package connectivity

import (
	"errors"
	"log"

	"github.com/gosnmp/gosnmp"
)

// snmpWalker is the part of a GoSNMP client this package walks tables with.
// Narrow enough that a test can supply an agent which refuses GETBULK.
type snmpWalker interface {
	BulkWalk(rootOid string, fn gosnmp.WalkFunc) error
	Walk(rootOid string, fn gosnmp.WalkFunc) error
}

// callbackError marks an error as ours rather than the agent's, so a decode
// failure is not mistaken for a refusal to serve GETBULK.
type callbackError struct{ err error }

func (c callbackError) Error() string { return c.err.Error() }
func (c callbackError) Unwrap() error { return c.err }

// bulkWalk reads a table with GETBULK, falling back to GETNEXT if the agent
// refuses.
//
// GETBULK returns many values per request where GETNEXT returns one, which on a
// populated chassis is the difference between seconds and minutes. Not every
// agent serves it, so a refusal costs one wasted request rather than the table.
//
// The fallback is logged. A silent downgrade would leave the system quietly slow
// with nothing to explain why.
func bulkWalk(w snmpWalker, oid string, fn gosnmp.WalkFunc) error {
	wrapped := func(pdu gosnmp.SnmpPDU) error {
		if err := fn(pdu); err != nil {
			return callbackError{err}
		}
		return nil
	}

	err := w.BulkWalk(oid, wrapped)
	if err == nil {
		return nil
	}

	var cb callbackError
	if errors.As(err, &cb) {
		return cb.err
	}

	log.Printf("[SNMP] GETBULK refused for %s (%v); falling back to GETNEXT for this walk", oid, err)
	if err := w.Walk(oid, wrapped); err != nil {
		if errors.As(err, &cb) {
			return cb.err
		}
		return err
	}
	return nil
}
```

- [ ] **Step 4: Jalankan, pastikan lulus**

Run: `go test ./internal/connectivity/ -run TestBulkWalk -count=1 -v`
Expected: tiga tes PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/connectivity/snmp_bulkwalk.go backend/internal/connectivity/snmp_bulkwalk_test.go
git commit -m "feat(snmp): read tables with GETBULK, falling back audibly when refused"
```

---

### Task 3: MaxRepetitions yang dapat diatur

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/connectivity/snmp_client.go:79-93`
- Test: `backend/internal/connectivity/snmp_client_test.go`

**Interfaces:**
- Consumes: pola `viper.SetDefault` yang sudah ada di `config.go`
- Produces: `connectivity.SetMaxRepetitions(n uint8)` dipanggil sekali saat worker
  dan API mulai; `newSNMPClientWithContext` memakainya.

- [ ] **Step 1: Tulis tes yang gagal**

```go
func TestClientCarriesTheConfiguredRepetitionCount(t *testing.T) {
	// A GETBULK with MaxRepetitions of zero degenerates into one value per
	// request, which is the GETNEXT cost this change exists to remove.
	SetMaxRepetitions(25)
	t.Cleanup(func() { SetMaxRepetitions(defaultMaxRepetitions) })

	client, err := newSNMPClient("127.0.0.1", "public", 161)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Conn.Close() })

	require.Equal(t, uint8(25), client.MaxRepetitions)
}

func TestRepetitionCountNeverFallsToZero(t *testing.T) {
	SetMaxRepetitions(0)
	t.Cleanup(func() { SetMaxRepetitions(defaultMaxRepetitions) })

	client, err := newSNMPClient("127.0.0.1", "public", 161)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Conn.Close() })

	require.Equal(t, uint8(defaultMaxRepetitions), client.MaxRepetitions)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `go test ./internal/connectivity/ -run TestClientCarries -count=1`
Expected: FAIL — `undefined: SetMaxRepetitions`

- [ ] **Step 3: Implementasi**

Di `snmp_client.go`:

```go
// defaultMaxRepetitions is how many values one GETBULK asks for. Measured
// against a ZTE C320 in cmd/snmpbench: high enough to cut round trips by an
// order of magnitude, low enough that the agent still answers.
const defaultMaxRepetitions uint8 = 25

var maxRepetitions = defaultMaxRepetitions

// SetMaxRepetitions overrides how many values one GETBULK asks for. A zero is
// ignored: gosnmp would then return a single value per request, which is the
// GETNEXT cost this exists to avoid.
func SetMaxRepetitions(n uint8) {
	if n == 0 {
		n = defaultMaxRepetitions
	}
	maxRepetitions = n
}
```

dan tambahkan `MaxRepetitions: maxRepetitions,` ke literal `gosnmp.GoSNMP` di
`newSNMPClientWithContext`.

Di `config.go` tambahkan `viper.SetDefault("SNMP_MAX_REPETITIONS", 25)` dan field
`SNMPMaxRepetitions int` pada struct config, mengikuti pola field yang sudah ada.

Di `cmd/worker/main.go` dan `cmd/api/main.go`, setelah config dimuat:
`connectivity.SetMaxRepetitions(uint8(cfg.SNMPMaxRepetitions))`.

- [ ] **Step 4: Jalankan, pastikan lulus**

Run: `go test ./internal/connectivity/ -run 'TestClientCarries|TestRepetitionCount' -count=1 -v`
Expected: dua tes PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config backend/internal/connectivity backend/cmd
git commit -m "feat(snmp): make the GETBULK repetition count configurable"
```

---

### Task 4: Pindahkan 18 pemanggilan walk ke helper

**Files:**
- Modify: `backend/internal/connectivity/snmp_metrics_walk.go`, `snmp_walks.go`,
  `snmp_metrics_table.go`, `snmp_traffic_rates.go`, `snmp_ports.go`, `snmp_vlans.go`,
  `snmp_tcont_profiles.go`, `snmp_system.go`, `snmp_card_health.go`,
  `snmp_walk_uncfg.go`, `driver_hsgq.go`

**Interfaces:**
- Consumes: `bulkWalk` dari Task 2

- [ ] **Step 1: Ganti tiap pemanggilan**

Setiap `client.Walk(OID, func(pdu gosnmp.SnmpPDU) error {` menjadi
`bulkWalk(client, OID, func(pdu gosnmp.SnmpPDU) error {`. Tidak ada perubahan lain
— badan callback tetap apa adanya.

- [ ] **Step 2: Pastikan tidak ada yang tertinggal**

Run: `grep -rn "client\.Walk(" backend/internal/connectivity/*.go | grep -v _test`
Expected: tidak ada keluaran

- [ ] **Step 3: Jalankan seluruh suite**

Run: `cd backend && go build ./... && go test ./internal/connectivity/ -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/connectivity
git commit -m "perf(snmp): every table read now goes through GETBULK"
```

---

### Task 5: Cabut plafon 1000 ONT di worker

Plafon ini membuat worker memantau 1000 ONT pertama saja. Pada 930 ONT ia kebetulan
pas; pada 100 ribu, 99% ONT tidak terpantau dan tidak ada yang memberi tahu.

**Files:**
- Modify: `backend/cmd/worker/main.go:100-140`
- Create: `backend/internal/services/ont_iterate.go`
- Test: `backend/internal/services/ont_iterate_test.go`

**Interfaces:**
- Produces: `func (s *ONTService) EachONTOfOLT(oltID uuid.UUID, batch int, fn func([]models.ONT) error) error`

- [ ] **Step 1: Tulis tes yang gagal**

```go
func TestEachONTOfOLTVisitsEveryRowBeyondOnePage(t *testing.T) {
	// The worker used to take one page of 1000 and call it the network. At any
	// real size that silently stops monitoring most of it.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	for i := 0; i < 2500; i++ {
		require.NoError(t, db.Create(&models.ONT{
			ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: i,
			SerialNumber: fmt.Sprintf("SN%08d", i), Status: models.ONTStatusOnline,
		}).Error)
	}

	seen := 0
	require.NoError(t, ontService.EachONTOfOLT(oltID, 1000, func(batch []models.ONT) error {
		seen += len(batch)
		return nil
	}))

	require.Equal(t, 2500, seen)
}

func TestEachONTOfOLTStopsWhenTheCallbackFails(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: 1,
		SerialNumber: "SN1", Status: models.ONTStatusOnline,
	}).Error)

	boom := errors.New("write failed")
	require.ErrorIs(t, ontService.EachONTOfOLT(oltID, 100, func([]models.ONT) error {
		return boom
	}), boom)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `go test ./internal/services/ -run TestEachONTOfOLT -count=1`
Expected: FAIL — `undefined: EachONTOfOLT`

- [ ] **Step 3: Implementasi**

```go
// defaultONTBatch is how many ONTs one page of the walk carries when the caller
// names no size. Large enough that a populated chassis costs few round trips,
// small enough that a batch stays cheap to hold.
const defaultONTBatch = 1000

// EachONTOfOLT hands every ONT of one OLT to fn in batches, ordered by id so a
// row inserted mid-walk cannot make the paging skip or repeat one.
//
// The worker reads ONTs to write metrics against; taking a single page instead
// meant it stopped monitoring everything past the page size without saying so.
func (s *ONTService) EachONTOfOLT(oltID uuid.UUID, batch int, fn func([]models.ONT) error) error {
	if batch < 1 {
		batch = defaultONTBatch
	}

	var cursor uuid.UUID
	for {
		var rows []models.ONT
		q := s.db.Where("olt_id = ?", oltID)
		if cursor != uuid.Nil {
			q = q.Where("id > ?", cursor)
		}
		if err := q.Order("id").Limit(batch).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := fn(rows); err != nil {
			return err
		}
		cursor = rows[len(rows)-1].ID
	}
}
```

- [ ] **Step 4: Jalankan, pastikan lulus**

Run: `go test ./internal/services/ -run TestEachONTOfOLT -count=1 -v`
Expected: dua tes PASS

- [ ] **Step 5: Pakai di worker**

Ganti `onts, _, err := ontService.List(nil, nil, 1000, 0)` dan loop `for _, ont := range onts`
menjadi loop luar per OLT yang memanggil `EachONTOfOLT`, sehingga cache walk per OLT
yang sudah ada (`oltMetricsCache` dan kawan-kawan) terisi sekali per OLT alih-alih
diindeks ulang per ONT.

- [ ] **Step 6: Jalankan tes worker**

Run: `go test ./cmd/worker/ -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services backend/cmd/worker
git commit -m "fix(worker): poll every ONT, not the first thousand"
```

---

### Task 6: Tulis metrik secara batch

**Files:**
- Modify: `backend/internal/services/metrics_service.go`
- Test: `backend/internal/services/metrics_batch_test.go`

**Interfaces:**
- Consumes: `connectivity.ONTMetrics`, `connectivity.ONUTrafficRates`
- Produces: `type MetricSample struct { ONTID uuid.UUID; Metrics *connectivity.ONTMetrics; Rates *connectivity.ONUTrafficRates }`
  dan `func (s *MetricsService) StoreMetricsBatch(samples []MetricSample) error`

- [ ] **Step 1: Tulis tes yang gagal**

```go
func TestStoreMetricsBatchWritesEverySample(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	samples := make([]MetricSample, 0, 2500)
	for i := 0; i < 2500; i++ {
		rx := -20.0
		samples = append(samples, MetricSample{
			ONTID:   uuid.New(),
			Metrics: &connectivity.ONTMetrics{RxPower: &rx},
		})
	}

	require.NoError(t, service.StoreMetricsBatch(samples))

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics").Scan(&count).Error)
	require.Equal(t, int64(2500), count, "a chunk boundary dropped rows")
}

func TestStoreMetricsBatchOnNoSamplesTouchesNothing(t *testing.T) {
	db := setupMetricsTestDB(t)
	require.NoError(t, NewMetricsService(db).StoreMetricsBatch(nil))
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `go test ./internal/services/ -run TestStoreMetricsBatch -count=1`
Expected: FAIL — `undefined: StoreMetricsBatch`

- [ ] **Step 3: Implementasi**

Bangun satu `INSERT` multi-baris per potongan. `metricsInsertChunk` adalah 1000:
batas 65535 parameter Postgres dibagi 13 kolom memberi plafon sekitar 5000, dan
1000 menyisakan ruang bila kolom bertambah.

```go
// metricsInsertChunk is how many samples go into one INSERT. Postgres caps a
// statement at 65535 parameters, which across 13 columns allows about 5000;
// 1000 leaves room for a column to be added without silently crossing it.
const metricsInsertChunk = 1000
```

`StoreMetrics` yang lama tetap ada dan memanggil `StoreMetricsBatch` dengan satu
sampel, sehingga pemanggil lain tidak ikut berubah dan hanya ada satu jalur tulis.

Baris log per sampel dihapus dari jalur ini; pemanggil mencatat satu ringkasan
per OLT.

- [ ] **Step 4: Jalankan, pastikan lulus**

Run: `go test ./internal/services/ -run TestStoreMetrics -count=1 -v`
Expected: tes baru dan tes `StoreMetrics` lama sama-sama PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services
git commit -m "perf(metrics): write a poll cycle's samples in batches, not one row at a time"
```

---

### Task 7: Worker memakai tulis batch dan berhenti mencatat per ONT

**Files:**
- Modify: `backend/cmd/worker/main.go`, `backend/cmd/worker/ont_poll.go`

**Interfaces:**
- Consumes: `EachONTOfOLT` (Task 5), `StoreMetricsBatch` (Task 6)

- [ ] **Step 1: Kumpulkan sampel, jangan tulis satu-satu**

`processOnt` berhenti memanggil `StoreMetrics` dan sebagai gantinya mengembalikan
`(MetricSample, bool)`. Pemanggil mengumpulkannya per OLT dan menulis sekali di
akhir OLT itu.

Perubahan status tetap lewat jalur yang sudah ada, karena aturan "poll yang gagal
tidak menandai ONT offline" hidup di sana dan tidak boleh ikut berpindah.

- [ ] **Step 2: Ganti log per-ONT dengan ringkasan per OLT**

Hapus `log.Printf` per ONT di `ont_poll.go`. Setelah satu OLT selesai, catat satu
baris: nama OLT, jumlah ONT diproses, jumlah sampel ditulis, jumlah perubahan
status, dan durasi.

- [ ] **Step 3: Jalankan tes worker**

Run: `go test ./cmd/worker/ -count=1 -v`
Expected: PASS, termasuk tes yang menjaga aturan "siklus yang dilewati tidak
menandai ONT offline"

- [ ] **Step 4: Gerbang penuh**

Run:
```bash
cd backend
gofmt -s -l . && go vet ./... && go test ./... -race -count=1
```
Expected: gofmt tidak mengeluarkan apa-apa, vet bersih, semua paket ok

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/worker
git commit -m "perf(worker): one batched write and one log line per OLT, not per ONT"
```

---

### Task 8: Ukur ulang dan catat hasilnya

Tahap A2 memerlukan angka nyata, bukan perkiraan: berapa ONT per menit yang sanggup
dibaca satu worker lewat tunnel ini. Tanpa ini, jumlah worker di A2 hanya tebakan.

**Files:**
- Modify: `docs/superpowers/specs/2026-08-30-polling-engine-scale-design.md`

- [ ] **Step 1: Deploy dan amati satu siklus penuh**

Run: `ssh radpro 'sudo docker logs tikman-worker --since 15m | grep "cycle"'`
Expected: durasi siklus sebelum dan sesudah, untuk 930 ONT yang sama

- [ ] **Step 2: Catat di spec**

Tambahkan bagian "Hasil A1": durasi siklus sebelum dan sesudah, ONT per menit per
worker, dan nilai `MaxRepetitions` yang dipakai. Angka ini yang menentukan berapa
worker diperlukan di A2.

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-08-30-polling-engine-scale-design.md
git commit -m "docs: record what A1 actually bought, as the input to A2"
```
