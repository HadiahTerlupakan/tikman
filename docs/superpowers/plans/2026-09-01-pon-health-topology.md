# PON Health Topology Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show where a fault sits — OLT → card → PON → subscriber — drawing only the branches that are actually in trouble.

**Architecture:** One aggregate query filters PON ports in SQL by two independent rules and returns a pruned tree. A hand-rolled SVG component draws it; layout is pure column arithmetic in a separate module so it can be tested without a DOM. Clicking a PON filters the existing subscriber tab.

**Tech Stack:** Go 1.25, Gin, GORM, PostgreSQL/TimescaleDB. React 18, TypeScript, Ant Design 5, TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-09-01-pon-health-topology-design.md`

## Global Constraints

- **No new runtime dependencies.** The SVG is hand-rolled. The repo has 13 runtime dependencies and adding a diagram library for one page was rejected during design.
- **Files ≤ 350 lines, functions ≤ 50 lines** (CLAUDE.md). Test files are exempt from the line limit.
- **TDD.** Every task writes the failing test first and runs it to watch it fail before implementing.
- **Postgres-only SQL is tested against Postgres.** `EXTRACT(EPOCH …)`, window functions and lateral joins have no SQLite equivalent. Use the `TEST_POSTGRES_DSN` harness; it fails hard under `CI` and skips locally.
- **Comments explain why, never what** (CLAUDE.md).
- **UI copy is Indonesian**, matching the existing page.
- **Thresholds:** outage share `> 0.05`; trap rate `> 5 × OLT median` **and** `> 100`; PONs with `< 5` ONTs excluded; at most 5 worst subscribers per PON.

**Local Postgres for tests:**

```bash
docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=tikman_test --name tikman-test-pg postgres:15-alpine
export TEST_POSTGRES_DSN="host=localhost port=55432 user=postgres password=test dbname=tikman_test sslmode=disable"
```

---

### Task 1: Status filter on the subscriber tab

Replaces the growing parameter list with a filter struct, following the repo's own `ONTListFilter` pattern, and adds the status filter approved separately.

**Files:**
- Modify: `backend/internal/services/ont_trouble.go`
- Modify: `backend/internal/api/ont_handler_troubled.go`
- Modify: `backend/internal/services/ont_trouble_postgres_test.go`
- Modify: `backend/internal/api/ont_handler_troubled_test.go`
- Modify: `frontend/src/infrastructure/repositories/OntRepository.ts`
- Modify: `frontend/src/application/hooks/useOnts.ts`
- Modify: `frontend/src/presentation/pages/TroubledOntsPage.tsx`
- Modify: `frontend/src/presentation/pages/__tests__/TroubledOntsPage.test.tsx`

**Interfaces:**
- Consumes: `TroubledONTs(window time.Duration, limit int, oltID *uuid.UUID) ([]TroubledONT, TroubledSummary, error)` — the current signature, being replaced.
- Produces: `TroubledFilter{Window time.Duration; Limit int; OLTID *uuid.UUID; Status *models.ONTStatus}` and `TroubledONTs(filter TroubledFilter) ([]TroubledONT, TroubledSummary, error)`. Task 2 reuses `TroubledFilter`'s `Window` and `OLTID` fields.

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/services/ont_trouble_postgres_test.go`:

```go
func TestTroubledONTsFiltersByStatus(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)

	up := f.ont(t, "SN-UP", 1, 1)
	f.traps(t, up.SerialNumber, 10, time.Hour)

	down := f.ont(t, "SN-DOWN-NOW", 1, 2)
	f.traps(t, down.SerialNumber, 10, time.Hour)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", down.ID).
		Update("status", models.ONTStatusLOS).Error)

	los := models.ONTStatusLOS
	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{
		Window: 24 * time.Hour, Limit: 10, Status: &los,
	})
	require.NoError(t, err)

	require.Len(t, troubled, 1)
	assert.Equal(t, "SN-DOWN-NOW", troubled[0].SerialNumber)
}

func TestTroubledSummaryFollowsTheStatusFilter(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)

	for i := 0; i < 3; i++ {
		ont := f.ont(t, "SN-ON-"+uuid.NewString()[:6], 1, i+1)
		f.traps(t, ont.SerialNumber, 5, time.Hour)
	}
	down := f.ont(t, "SN-OFF", 2, 1)
	f.traps(t, down.SerialNumber, 5, time.Hour)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", down.ID).
		Update("status", models.ONTStatusLOS).Error)

	los := models.ONTStatusLOS
	_, summary, err := NewONTService(db).TroubledONTs(TroubledFilter{
		Window: 24 * time.Hour, Limit: 10, Status: &los,
	})
	require.NoError(t, err)

	// A summary that ignored the filter would report four while the table
	// showed one, and the operator would trust the larger number.
	assert.EqualValues(t, 1, summary.ONTCount)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/services/ -run TestTroubledONTsFiltersByStatus
```

Expected: FAIL — `undefined: TroubledFilter`.

- [ ] **Step 3: Replace the parameter list with the filter struct**

In `backend/internal/services/ont_trouble.go`, add above `TroubledONTs`:

```go
// TroubledFilter is what a request asks the ranking for.
//
// A struct rather than a parameter list because the list had reached three and
// was about to reach four, and ONTListFilter in this same service already
// settled how this codebase carries query options.
type TroubledFilter struct {
	Window time.Duration
	Limit  int
	OLTID  *uuid.UUID
	Status *models.ONTStatus
}
```

Change the signature to `func (s *ONTService) TroubledONTs(filter TroubledFilter) ([]TroubledONT, TroubledSummary, error)`, replace the body's `window` with `filter.Window` and `limit` with `filter.Limit`, and add the status predicate to the `WHERE` clause immediately after the OLT one:

```sql
		  AND (?::uuid IS NULL OR n.olt_id = ?::uuid)
		  AND (?::text IS NULL OR n.status = ?::text)
```

The argument list becomes `since, since, filter.OLTID, filter.OLTID, filter.Status, filter.Status, filter.Limit`.

- [ ] **Step 4: Update the existing call sites in the test file**

Replace every `TroubledONTs(24*time.Hour, N, X)` call in `ont_trouble_postgres_test.go` with `TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: N, OLTID: X})`.

- [ ] **Step 5: Run the service tests**

```bash
cd backend && go test ./internal/services/ -run TestTroubled -v
```

Expected: PASS, including the two new tests.

- [ ] **Step 6: Write the failing handler test**

Append to `backend/internal/api/ont_handler_troubled_test.go`:

```go
func TestTroubledStatusRejectsAnUnknownValue(t *testing.T) {
	// Rejected rather than ignored: a filter that fails silently shows an empty
	// table, and the operator reads that as "nothing is wrong".
	_, err := parseTroubledStatus("rusak")
	require.Error(t, err)
}

func TestTroubledStatusAcceptsAKnownValue(t *testing.T) {
	status, err := parseTroubledStatus("los")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, models.ONTStatusLOS, *status)
}

func TestTroubledStatusIsOptional(t *testing.T) {
	status, err := parseTroubledStatus("")
	require.NoError(t, err)
	assert.Nil(t, status)
}
```

`parseTroubledStatus` is tested directly rather than through HTTP because it is where the decision lives; the handler merely turns its error into a 400.

- [ ] **Step 7: Implement the status parameter**

In `backend/internal/api/ont_handler_troubled.go`, add:

```go
// parseTroubledStatus reads the status filter, refusing a value no ONT can hold.
func parseTroubledStatus(raw string) (*models.ONTStatus, error) {
	if raw == "" {
		return nil, nil
	}
	for _, known := range []models.ONTStatus{
		models.ONTStatusOnline, models.ONTStatusOffline,
		models.ONTStatusLOS, models.ONTStatusDyingGas,
	} {
		if models.ONTStatus(raw) == known {
			return &known, nil
		}
	}
	return nil, fmt.Errorf("unknown status: %s", raw)
}
```

In `ListTroubled`, after the OLT id block:

```go
	status, err := parseTroubledStatus(c.Query("status"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_STATUS",
			Error: err.Error(),
		})
		return
	}

	troubled, summary, err := h.ontService.TroubledONTs(services.TroubledFilter{
		Window: window, Limit: limit, OLTID: oltID, Status: status,
	})
```

Add `"fmt"` and the services import if absent.

- [ ] **Step 8: Run the handler tests**

```bash
cd backend && go test ./internal/api/ -run TestTroubled -v
```

Expected: PASS.

- [ ] **Step 9: Add the frontend status filter**

In `frontend/src/infrastructure/repositories/OntRepository.ts`, change the signature to `getTroubled(hours: number, oltId?: string, status?: string, limit = 50)` and add `status` to `params`.

In `frontend/src/application/hooks/useOnts.ts`:

```ts
export function useTroubledOnts(hours: number, oltId?: string, status?: string) {
  return useQuery({
    queryKey: ["onts", "troubled", hours, oltId ?? "all", status ?? "all"],
    queryFn: () => ontRepository.getTroubled(hours, oltId, status),
    refetchInterval: 60000,
    staleTime: 30000,
  });
}
```

In `frontend/src/presentation/pages/TroubledOntsPage.tsx`, add `const [status, setStatus] = useState<string | undefined>();`, pass it to the hook, and add a `Select` beside the OLT one:

```tsx
<Select
  allowClear
  style={{ width: 150 }}
  placeholder="Semua status"
  value={status}
  onChange={setStatus}
  options={[
    { value: "online", label: "Online" },
    { value: "los", label: "LOS" },
    { value: "dying_gasp", label: "Dying gasp" },
    { value: "offline", label: "Offline" },
  ]}
/>
```

- [ ] **Step 10: Add the page test**

Append to `frontend/src/presentation/pages/__tests__/TroubledOntsPage.test.tsx`:

```tsx
it("asks for one status when the operator picks one", () => {
  render(<TroubledOntsPage />);

  fireEvent.mouseDown(screen.getByText("Semua status"));
  fireEvent.click(screen.getByText("LOS"));

  expect(useTroubledOnts).toHaveBeenLastCalledWith(24, undefined, "los");
});
```

Update the two existing `toHaveBeenCalledWith(24, undefined)` assertions to `(24, undefined, undefined)`.

- [ ] **Step 11: Run every gate**

```bash
cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
cd ../backend && go test ./... && go vet ./... && gofmt -s -l .
```

Expected: all pass, `gofmt` prints nothing.

- [ ] **Step 12: Commit**

```bash
git add backend/internal frontend/src
git commit -m "feat(onts): filter the troubled list by status

The parameter list had reached three and was about to reach four, so it
becomes TroubledFilter — the shape ONTListFilter already settled in this
same service.

An unknown status is refused rather than ignored. A filter that fails
silently returns an empty table, and an empty table reads as good news."
```

---

### Task 2: PON health query and endpoint

**Files:**
- Create: `backend/internal/services/pon_health.go`
- Create: `backend/internal/services/pon_health_postgres_test.go`
- Create: `backend/internal/api/pon_health_handler.go`
- Create: `backend/internal/api/pon_health_handler_test.go`
- Modify: `backend/internal/api/router.go`
- Modify: `backend/internal/api/olt_handler.go`

**Interfaces:**
- Consumes: `TroubledFilter` from Task 1 (uses `Window` and reads `OLTID` from the route parameter instead).
- Produces:

```go
type PonSubscriber struct {
	ONTID       uuid.UUID `json:"ont_id"`
	Label       string    `json:"label"`
	Name        string    `json:"name"`
	TrapCount   int64     `json:"trap_count"`
	DownMinutes int64     `json:"down_minutes"`
}

type PonNode struct {
	Port         int             `json:"port"`
	ONTCount     int64           `json:"ont_count"`
	TrapPerONT   int64           `json:"trap_per_ont"`
	OutageShare  float64         `json:"outage_share"`
	Worst        []PonSubscriber `json:"worst"`
}

type CardNode struct {
	Slot     int       `json:"slot"`
	PonCount int       `json:"pon_count"`
	Pons     []PonNode `json:"pons"`
}

type PonHealth struct {
	OLTID            uuid.UUID  `json:"olt_id"`
	OLTName          string     `json:"olt_name"`
	MedianTrapPerONT int64      `json:"median_trap_per_ont"`
	TrapThreshold    int64      `json:"trap_threshold"`
	OutageThreshold  float64    `json:"outage_threshold"`
	Cards            []CardNode `json:"cards"`
}

func (s *ONTService) PonHealthFor(oltID uuid.UUID, window time.Duration) (PonHealth, error)
```

Task 3 consumes this JSON shape verbatim as its TypeScript types.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/services/pon_health_postgres_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ponFixture builds ONTs on a given card and port, with traps and outages, so a
// test can state a PON's shape in one line.
func ponOnPort(t *testing.T, f troubleFixture, slot, port, count int, trapsEach int, downSeconds int64) {
	t.Helper()
	for i := 0; i < count; i++ {
		serial := "SN-" + uuid.NewString()[:8]
		ont := models.ONT{
			ID: uuid.New(), OLTID: f.oltID, Slot: &slot, PortID: port, ONTID: i + 1,
			SerialNumber: serial, Name: "Pelanggan", Status: models.ONTStatusOnline,
		}
		require.NoError(t, f.db.Create(&ont).Error)
		if trapsEach > 0 {
			f.traps(t, serial, trapsEach, time.Hour)
		}
		if downSeconds > 0 {
			f.outage(t, ont.ID, downSeconds, time.Hour)
		}
	}
}

func healthFor(t *testing.T, db *gorm.DB, oltID uuid.UUID) PonHealth {
	t.Helper()
	health, err := NewONTService(db).PonHealthFor(oltID, 24*time.Hour)
	require.NoError(t, err)
	return health
}

func TestPonHealthShowsAPortLosingService(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	// Quiet ports set the median low; the last one loses a tenth of the day on
	// almost no traps, which is the Depok 3/2 shape that one criterion misses.
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 2, 7, 6, 1, 9000)

	health := healthFor(t, db, f.oltID)

	require.Len(t, health.Cards, 1)
	assert.Equal(t, 2, health.Cards[0].Slot)
	require.Len(t, health.Cards[0].Pons, 1)
	assert.Equal(t, 7, health.Cards[0].Pons[0].Port)
}

func TestPonHealthShowsAPortThatChurnsWithoutOutage(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 10, 0)
	ponOnPort(t, f, 1, 2, 10, 10, 0)
	ponOnPort(t, f, 9, 8, 10, 900, 0)

	health := healthFor(t, db, f.oltID)

	// 900 per ONT is far past five times the median of ten and past the floor
	// of a hundred, with no outage at all: the Cariu 9/8 shape.
	require.Len(t, health.Cards, 1)
	assert.Equal(t, 9, health.Cards[0].Slot)
	assert.EqualValues(t, 900, health.Cards[0].Pons[0].TrapPerONT)
}

func TestPonHealthLeavesOutAnOutlierInAQuietNetwork(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 2, 2, 10, 13, 0)

	health := healthFor(t, db, f.oltID)

	// Thirteen is thirteen times this network's median and still not a fault.
	// The floor is what keeps a quiet OLT from reporting one.
	assert.Empty(t, health.Cards)
}

func TestPonHealthLeavesOutAPortWithTooFewONTs(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 3, 3, 4, 5000, 0)

	// One bad ONT on a port serving four would top any per-ONT ranking and say
	// nothing about the port.
	health := healthFor(t, db, f.oltID)
	assert.Empty(t, health.Cards)
}

func TestPonHealthNamesTheWorstSubscribersOnAPort(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 4, 4, 8, 900, 0)

	health := healthFor(t, db, f.oltID)

	require.Len(t, health.Cards, 1)
	// Eight subscribers on the port, five named: a port can hold seventy, and
	// drawing them all restores the problem this view exists to remove.
	assert.Len(t, health.Cards[0].Pons[0].Worst, 5)
}

func TestPonHealthReportsTheThresholdsItApplied(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 20, 0)
	ponOnPort(t, f, 1, 2, 10, 20, 0)

	health := healthFor(t, db, f.oltID)

	// Shown on screen rather than hidden, so the operator can judge the rule
	// instead of trusting it.
	assert.EqualValues(t, 20, health.MedianTrapPerONT)
	assert.EqualValues(t, 100, health.TrapThreshold)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd backend && go test ./internal/services/ -run TestPonHealth
```

Expected: FAIL — `undefined: PonHealth`.

- [ ] **Step 3: Implement the aggregate**

Create `backend/internal/services/pon_health.go` with the types from the Interfaces block above plus:

```go
// The rules a PON has to break to be drawn. Both exist because neither sees the
// other's fault: a port whose subscribers lose a tenth of the day on one trap
// each, and a port churning at nine hundred traps per ONT while losing nothing.
const (
	ponOutageShareThreshold = 0.05
	ponTrapMedianMultiple   = 5
	ponTrapFloor            = 100
	ponMinONTs              = 5
	ponWorstSubscribers     = 5
)

func (s *ONTService) PonHealthFor(oltID uuid.UUID, window time.Duration) (PonHealth, error) {
	since := time.Now().Add(-window)
	windowMinutes := window.Minutes()

	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return PonHealth{}, err
	}

	type ponRow struct {
		Slot        int
		Port        int
		ONTCount    int64
		TrapPerONT  int64
		OutageShare float64
		Median      float64
	}
	var rows []ponRow

	err := s.db.Raw(`
		WITH trap AS (
			SELECT serial_number, count(*) AS c
			FROM ont_trap_events
			WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
			GROUP BY serial_number
		),
		outage AS (
			SELECT ont_id,
			       sum(COALESCE(duration_seconds, EXTRACT(EPOCH FROM (now() - event_time))))
			         FILTER (WHERE event_type = 'offline') AS s
			FROM ont_events WHERE event_time > ? GROUP BY ont_id
		),
		pon AS (
			SELECT COALESCE(n.slot, 0) AS slot, n.port_id AS port,
			       count(*) AS ont_count,
			       (sum(COALESCE(t.c, 0))::numeric / count(*)) AS trap_per_ont,
			       (sum(COALESCE(g.s, 0)) / 60 / count(*) / ?) AS outage_share
			FROM onts n
			LEFT JOIN trap t ON t.serial_number = n.serial_number
			LEFT JOIN outage g ON g.ont_id = n.id
			WHERE n.olt_id = ?
			GROUP BY COALESCE(n.slot, 0), n.port_id
			HAVING count(*) >= ?
		),
		reference AS (
			SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY trap_per_ont) AS median FROM pon
		)
		SELECT p.slot, p.port, p.ont_count,
		       round(p.trap_per_ont)::bigint AS trap_per_ont,
		       p.outage_share, r.median
		FROM pon p CROSS JOIN reference r
		WHERE p.outage_share > ?
		   OR (p.trap_per_ont > r.median * ? AND p.trap_per_ont > ?)
		ORDER BY p.slot, p.port
	`, oltID, since, since, windowMinutes, oltID, ponMinONTs,
		ponOutageShareThreshold, ponTrapMedianMultiple, ponTrapFloor).Scan(&rows).Error
	if err != nil {
		return PonHealth{}, err
	}

	health := PonHealth{
		OLTID: olt.ID, OLTName: olt.Name,
		TrapThreshold:   ponTrapFloor,
		OutageThreshold: ponOutageShareThreshold,
	}
	if len(rows) > 0 {
		health.MedianTrapPerONT = int64(rows[0].Median)
	} else if median, err := s.ponMedian(oltID, since, windowMinutes); err == nil {
		health.MedianTrapPerONT = median
	}

	for _, row := range rows {
		worst, err := s.worstOnPon(oltID, row.Slot, row.Port, since)
		if err != nil {
			return PonHealth{}, err
		}
		node := PonNode{
			Port: row.Port, ONTCount: row.ONTCount,
			TrapPerONT: row.TrapPerONT, OutageShare: row.OutageShare, Worst: worst,
		}
		health.Cards = appendToCard(health.Cards, row.Slot, node)
	}
	return health, nil
}

// appendToCard groups a PON under its card, keeping the card order the query
// already sorted by.
func appendToCard(cards []CardNode, slot int, node PonNode) []CardNode {
	for i := range cards {
		if cards[i].Slot == slot {
			cards[i].Pons = append(cards[i].Pons, node)
			cards[i].PonCount = len(cards[i].Pons)
			return cards
		}
	}
	return append(cards, CardNode{Slot: slot, PonCount: 1, Pons: []PonNode{node}})
}
```

- [ ] **Step 4: Implement the two helpers**

Still in `pon_health.go`, keeping each under fifty lines:

```go
// ponMedian answers what a normal port looks like on this OLT when no port
// broke a rule, so the screen can still state the reference it judged against.
func (s *ONTService) ponMedian(oltID uuid.UUID, since time.Time, windowMinutes float64) (int64, error) {
	var median float64
	err := s.db.Raw(`
		WITH trap AS (
			SELECT serial_number, count(*) AS c FROM ont_trap_events
			WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
			GROUP BY serial_number
		),
		pon AS (
			SELECT (sum(COALESCE(t.c, 0))::numeric / count(*)) AS trap_per_ont
			FROM onts n LEFT JOIN trap t ON t.serial_number = n.serial_number
			WHERE n.olt_id = ? GROUP BY COALESCE(n.slot, 0), n.port_id
			HAVING count(*) >= ?
		)
		SELECT COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY trap_per_ont), 0) FROM pon
	`, oltID, since, oltID, ponMinONTs).Scan(&median).Error
	return int64(median), err
}

// worstOnPon names the subscribers a technician would look at first.
func (s *ONTService) worstOnPon(oltID uuid.UUID, slot, port int, since time.Time) ([]PonSubscriber, error) {
	var worst []PonSubscriber
	err := s.db.Raw(`
		WITH trap AS (
			SELECT serial_number, count(*) AS c FROM ont_trap_events
			WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
			GROUP BY serial_number
		),
		outage AS (
			SELECT ont_id, sum(COALESCE(duration_seconds, EXTRACT(EPOCH FROM (now() - event_time))))
			         FILTER (WHERE event_type = 'offline') AS s
			FROM ont_events WHERE event_time > ? GROUP BY ont_id
		)
		SELECT n.id AS ont_id,
		       'ONU-' || n.port_id || ':' || n.ont_id AS label,
		       n.name,
		       COALESCE(t.c, 0) AS trap_count,
		       (COALESCE(g.s, 0) / 60)::bigint AS down_minutes
		FROM onts n
		LEFT JOIN trap t ON t.serial_number = n.serial_number
		LEFT JOIN outage g ON g.ont_id = n.id
		WHERE n.olt_id = ? AND COALESCE(n.slot, 0) = ? AND n.port_id = ?
		ORDER BY trap_count DESC, down_minutes DESC
		LIMIT ?
	`, oltID, since, since, oltID, slot, port, ponWorstSubscribers).Scan(&worst).Error
	return worst, err
}
```

- [ ] **Step 5: Run the service tests**

```bash
cd backend && go test ./internal/services/ -run TestPonHealth -v
```

Expected: all six PASS.

- [ ] **Step 6: Write the failing handler test**

Create `backend/internal/api/pon_health_handler_test.go`:

```go
package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPonHealthRejectsAnInvalidOLTID(t *testing.T) {
	handler, _, _ := SetupOLTHandlerTest(t)

	w, c := SetupTestContext("GET", "/api/v1/olts/bukan-uuid/pon-health", nil)
	c.Params = gin.Params{{Key: "id", Value: "bukan-uuid"}}
	handler.PonHealth(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPonHealthReportsAnOLTThatDoesNotExist(t *testing.T) {
	handler, _, _ := SetupOLTHandlerTest(t)

	missing := uuid.New().String()
	w, c := SetupTestContext("GET", "/api/v1/olts/"+missing+"/pon-health", nil)
	c.Params = gin.Params{{Key: "id", Value: missing}}
	handler.PonHealth(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 7: Implement the handler and route**

Create `backend/internal/api/pon_health_handler.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PonHealth draws the fault down to the port it sits on.
//
// The subscriber ranking answers who is failing. Forty-one subscribers churning
// on one PON are one fault at the port, not forty-one in homes, and this is the
// view that says so.
func (h *OLTHandler) PonHealth(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code: "INVALID_ID", Error: "Invalid OLT ID format",
		})
		return
	}

	window, _ := troubledQuery(c)

	health, err := h.ontService.PonHealthFor(oltID, window)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code: "OLT_NOT_FOUND", Error: "OLT not found",
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code: "PON_HEALTH_FAILED", Error: err.Error(),
		})
	default:
		c.JSON(http.StatusOK, gin.H{"data": health, "hours": int(window.Hours())})
	}
}
```

In `backend/internal/api/router.go`, beside the other OLT routes:

```go
			olts.GET("/:id/pon-health", oltHandler.PonHealth)
```

`OLTHandler` already holds `ontService`; no constructor change is needed.

- [ ] **Step 8: Run the handler tests and the whole backend**

```bash
cd backend && go test ./internal/api/ -run TestPonHealth -v && go test ./... && go vet ./... && gofmt -s -l .
```

Expected: PASS, `gofmt` prints nothing.

- [ ] **Step 9: Commit**

```bash
git add backend/internal
git commit -m "feat(onts): rank PON ports by the two ways they fail

Two rules, because neither sees the other's fault. A port whose
subscribers lose a tenth of the day on one trap each is invisible to a
churn rule; a port churning at nine hundred traps per ONT while losing
nothing is invisible to an outage rule. Both were measured on this
network before either was written.

The floor under the churn rule is a judgement, not arithmetic: without
it the relative rule reports a quiet OLT's outlier as a fault. It is
returned to the caller so the screen can show the rule rather than
apply it invisibly."
```

---

### Task 3: Layout arithmetic and the SVG component

**Files:**
- Create: `frontend/src/domain/entities/PonHealth.ts`
- Create: `frontend/src/presentation/components/onts/ponLayout.ts`
- Create: `frontend/src/presentation/components/onts/__tests__/ponLayout.test.ts`
- Create: `frontend/src/presentation/components/onts/PonTopology.tsx`
- Create: `frontend/src/presentation/components/onts/__tests__/PonTopology.test.tsx`
- Modify: `frontend/src/domain/entities/index.ts` (only if it does not already `export * from "./PonHealth"` via a wildcard)

**Interfaces:**
- Consumes: the JSON from Task 2, typed as `PonHealth` with `cards[].pons[].worst[]`.
- Produces: `layoutPonTree(health: PonHealth): PonLayout` where

```ts
export interface LaidOutNode {
  id: string;
  kind: "olt" | "card" | "pon" | "ont";
  label: string;
  detail: string;
  x: number;
  y: number;
  width: number;
  height: number;
  severity: number;
}

export interface LaidOutEdge {
  id: string;
  from: string;
  to: string;
  path: string;
}

export interface PonLayout {
  nodes: LaidOutNode[];
  edges: LaidOutEdge[];
  width: number;
  height: number;
}
```

Task 4 renders `<PonTopology health={…} onSelectPon={…} />` and needs no layout types.

- [ ] **Step 1: Write the failing layout tests**

Create `frontend/src/presentation/components/onts/__tests__/ponLayout.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { PonHealth } from "@/domain/entities";
import { layoutPonTree } from "../ponLayout";

const health: PonHealth = {
  oltId: "olt-1",
  oltName: "Cariu",
  medianTrapPerOnt: 19,
  trapThreshold: 100,
  outageThreshold: 0.05,
  cards: [
    {
      slot: 8,
      ponCount: 1,
      pons: [
        {
          port: 12,
          ontCount: 41,
          trapPerOnt: 686,
          outageShare: 0.12,
          worst: [
            {
              ontId: "ont-1",
              label: "ONU-8:12",
              name: "PELANGGAN SATU",
              trapCount: 1204,
              downMinutes: 340,
            },
          ],
        },
      ],
    },
  ],
};

describe("layoutPonTree", () => {
  it("places one node per level of the tree", () => {
    const { nodes } = layoutPonTree(health);

    expect(nodes.map((n) => n.kind)).toEqual(["olt", "card", "pon", "ont"]);
  });

  it("puts each level in its own column, left to right", () => {
    const { nodes } = layoutPonTree(health);
    const x = nodes.map((n) => n.x);

    expect(x[0]).toBeLessThan(x[1]);
    expect(x[1]).toBeLessThan(x[2]);
    expect(x[2]).toBeLessThan(x[3]);
  });

  it("connects every node to its parent", () => {
    const { edges } = layoutPonTree(health);

    // Three edges for four nodes: a tree, not a mesh.
    expect(edges).toHaveLength(3);
    expect(edges[0].path).toMatch(/^M /);
  });

  it("scores severity against the worst port drawn", () => {
    const { nodes } = layoutPonTree(health);
    const pon = nodes.find((n) => n.kind === "pon");

    // The only port drawn is by definition the worst one, so it anchors the
    // scale the colours read from.
    expect(pon?.severity).toBe(1);
  });

  it("returns a canvas big enough for every node", () => {
    const { nodes, width, height } = layoutPonTree(health);

    for (const node of nodes) {
      expect(node.x + node.width).toBeLessThanOrEqual(width);
      expect(node.y + node.height).toBeLessThanOrEqual(height);
    }
  });

  it("draws nothing for an OLT with no troubled port", () => {
    const { nodes, edges } = layoutPonTree({ ...health, cards: [] });

    expect(nodes).toHaveLength(0);
    expect(edges).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd frontend && npx vitest run src/presentation/components/onts/__tests__/ponLayout.test.ts
```

Expected: FAIL — cannot resolve `../ponLayout`.

- [ ] **Step 3: Add the entity types**

Create `frontend/src/domain/entities/PonHealth.ts`:

```ts
/** PonSubscriber is one customer a technician would look at first. */
export interface PonSubscriber {
  ontId: string;
  label: string;
  name: string;
  trapCount: number;
  downMinutes: number;
}

/** PonNode is one PON port that broke at least one of the two rules. */
export interface PonNode {
  port: number;
  ontCount: number;
  trapPerOnt: number;
  outageShare: number;
  worst: PonSubscriber[];
}

/** CardNode is a line card, present only when one of its ports is in trouble. */
export interface CardNode {
  slot: number;
  ponCount: number;
  pons: PonNode[];
}

/**
 * PonHealth is the pruned tree: only branches in trouble, plus the thresholds
 * that pruned it, so the screen can show the rule instead of applying it
 * invisibly.
 */
export interface PonHealth {
  oltId: string;
  oltName: string;
  medianTrapPerOnt: number;
  trapThreshold: number;
  outageThreshold: number;
  cards: CardNode[];
}
```

If `frontend/src/domain/entities/index.ts` does not re-export with a wildcard, add `export * from "./PonHealth";`.

- [ ] **Step 4: Implement the layout**

Create `frontend/src/presentation/components/onts/ponLayout.ts`. Keep it under 150 lines; it is arithmetic only, with no React import:

```ts
import type { PonHealth } from "@/domain/entities";

const COLUMN_X = [0, 200, 400, 640];
const NODE_WIDTH = [150, 150, 200, 240];
const NODE_HEIGHT = 52;
const GAP = 14;

export interface LaidOutNode {
  id: string;
  kind: "olt" | "card" | "pon" | "ont";
  label: string;
  detail: string;
  x: number;
  y: number;
  width: number;
  height: number;
  severity: number;
}

export interface LaidOutEdge {
  id: string;
  from: string;
  to: string;
  path: string;
}

export interface PonLayout {
  nodes: LaidOutNode[];
  edges: LaidOutEdge[];
  width: number;
  height: number;
}

/** curve draws the S-bend between two columns, as in a pipeline diagram. */
function curve(x1: number, y1: number, x2: number, y2: number): string {
  const mid = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2}`;
}

/**
 * layoutPonTree turns the pruned tree into placed boxes and connectors.
 *
 * Depth is fixed at four, so this is column arithmetic rather than graph
 * layout: each leaf takes the next row, and a parent centres on its children.
 * Kept apart from the drawing so the arithmetic can be tested directly —
 * testing placement through the DOM only tests React.
 */
export function layoutPonTree(health: PonHealth): PonLayout {
  const nodes: LaidOutNode[] = [];
  const edges: LaidOutEdge[] = [];
  if (health.cards.length === 0) return { nodes, edges, width: 0, height: 0 };

  const worstTrap = Math.max(
    ...health.cards.flatMap((c) => c.pons.map((p) => p.trapPerOnt)),
    1,
  );
  let row = 0;

  const push = (
    kind: LaidOutNode["kind"],
    id: string,
    label: string,
    detail: string,
    y: number,
    severity: number,
  ) => {
    const level = ["olt", "card", "pon", "ont"].indexOf(kind);
    nodes.push({
      id,
      kind,
      label,
      detail,
      x: COLUMN_X[level],
      y,
      width: NODE_WIDTH[level],
      height: NODE_HEIGHT,
      severity,
    });
  };

  const link = (from: string, to: string) => {
    const a = nodes.find((n) => n.id === from);
    const b = nodes.find((n) => n.id === to);
    if (!a || !b) return;
    edges.push({
      id: `${from}->${to}`,
      from,
      to,
      path: curve(
        a.x + a.width,
        a.y + a.height / 2,
        b.x,
        b.y + b.height / 2,
      ),
    });
  };

  const cardCentres: number[] = [];

  for (const card of health.cards) {
    const ponCentres: number[] = [];
    const cardId = `card-${card.slot}`;

    for (const pon of card.pons) {
      const ontCentres: number[] = [];
      const ponId = `pon-${card.slot}-${pon.port}`;

      for (const ont of pon.worst) {
        const y = row * (NODE_HEIGHT + GAP);
        push(
          "ont",
          `ont-${ont.ontId}`,
          ont.label,
          `${ont.trapCount.toLocaleString("id-ID")} trap · ${ont.downMinutes} mnt`,
          y,
          ont.trapCount / Math.max(...pon.worst.map((w) => w.trapCount), 1),
        );
        ontCentres.push(y);
        row += 1;
      }

      const ponY = centreOf(ontCentres, row);
      push(
        "pon",
        ponId,
        `PON ${pon.port}`,
        `${pon.trapPerOnt} trap/ONT · ${Math.round(pon.outageShare * 100)}% mati`,
        ponY,
        pon.trapPerOnt / worstTrap,
      );
      ponCentres.push(ponY);
      for (const ont of pon.worst) link(ponId, `ont-${ont.ontId}`);
    }

    const cardY = centreOf(ponCentres, row);
    push("card", cardId, `Kartu ${card.slot}`, `${card.ponCount} PON`, cardY, 0);
    cardCentres.push(cardY);
    for (const pon of card.pons) link(cardId, `pon-${card.slot}-${pon.port}`);
  }

  const oltY = centreOf(cardCentres, row);
  push("olt", "olt", health.oltName, `median ${health.medianTrapPerOnt} trap/ONT`, oltY, 0);
  for (const card of health.cards) link("olt", `card-${card.slot}`);

  const height = Math.max(...nodes.map((n) => n.y + n.height));
  const width = Math.max(...nodes.map((n) => n.x + n.width));
  return { nodes, edges, width, height };
}

/** centreOf puts a parent level with its children, or on its own row if none. */
function centreOf(childYs: number[], fallbackRow: number): number {
  if (childYs.length === 0) return fallbackRow * (NODE_HEIGHT + GAP);
  return (childYs[0] + childYs[childYs.length - 1]) / 2;
}
```

- [ ] **Step 5: Run the layout tests**

```bash
cd frontend && npx vitest run src/presentation/components/onts/__tests__/ponLayout.test.ts
```

Expected: all six PASS.

- [ ] **Step 6: Write the failing component test**

Create `frontend/src/presentation/components/onts/__tests__/PonTopology.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { PonHealth } from "@/domain/entities";
import { PonTopology } from "../PonTopology";

const health: PonHealth = {
  oltId: "olt-1",
  oltName: "Cariu",
  medianTrapPerOnt: 19,
  trapThreshold: 100,
  outageThreshold: 0.05,
  cards: [
    {
      slot: 8,
      ponCount: 1,
      pons: [
        {
          port: 12,
          ontCount: 41,
          trapPerOnt: 686,
          outageShare: 0.12,
          worst: [
            {
              ontId: "ont-1",
              label: "ONU-8:12",
              name: "PELANGGAN SATU",
              trapCount: 1204,
              downMinutes: 340,
            },
          ],
        },
      ],
    },
  ],
};

describe("PonTopology", () => {
  it("draws the branch down to the subscriber", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(screen.getByText("Cariu")).toBeInTheDocument();
    expect(screen.getByText("Kartu 8")).toBeInTheDocument();
    expect(screen.getByText("PON 12")).toBeInTheDocument();
    expect(screen.getByText("ONU-8:12")).toBeInTheDocument();
  });

  it("carries every figure as text, not only as colour", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(screen.getByText(/686 trap\/ONT · 12% mati/)).toBeInTheDocument();
  });

  it("hands the chosen port back to its caller", () => {
    const onSelectPon = vi.fn();
    render(<PonTopology health={health} onSelectPon={onSelectPon} />);

    fireEvent.click(screen.getByText("PON 12"));

    expect(onSelectPon).toHaveBeenCalledWith(8, 12);
  });

  it("shows the rule it applied instead of applying it invisibly", () => {
    render(<PonTopology health={health} onSelectPon={vi.fn()} />);

    expect(screen.getByText(/di atas 100 dan lima kali median/i)).toBeInTheDocument();
  });

  it("says the OLT is healthy rather than drawing an empty canvas", () => {
    render(
      <PonTopology health={{ ...health, cards: [] }} onSelectPon={vi.fn()} />,
    );

    expect(screen.getByText(/tidak ada PON bermasalah/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 7: Run the component test to verify it fails**

```bash
cd frontend && npx vitest run src/presentation/components/onts/__tests__/PonTopology.test.tsx
```

Expected: FAIL — cannot resolve `../PonTopology`.

- [ ] **Step 8: Implement the component**

Create `frontend/src/presentation/components/onts/PonTopology.tsx`. It draws what the layout placed and holds no arithmetic:

```tsx
import { Empty, theme } from "antd";
import type { PonHealth } from "@/domain/entities";
import { layoutPonTree, type LaidOutNode } from "./ponLayout";

interface PonTopologyProps {
  health: PonHealth;
  onSelectPon: (slot: number, port: number) => void;
}

/**
 * PonTopology draws the pruned tree: OLT, then only the cards and ports in
 * trouble, then the subscribers worst hit on each.
 *
 * Severity is scored against the worst port drawn rather than a fixed scale, so
 * the colours say where the trouble is concentrated on this chassis instead of
 * claiming an absolute standard this network never agreed to.
 */
export function PonTopology({ health, onSelectPon }: PonTopologyProps) {
  const { token } = theme.useToken();
  const { nodes, edges, width, height } = layoutPonTree(health);

  if (nodes.length === 0) {
    return (
      <Empty
        description={`Tidak ada PON bermasalah di ${health.oltName} pada rentang ini`}
      />
    );
  }

  const fill = (node: LaidOutNode) => {
    if (node.severity > 0.66) return token.colorErrorBg;
    if (node.severity > 0.33) return token.colorWarningBg;
    return token.colorBgContainer;
  };

  const stroke = (node: LaidOutNode) => {
    if (node.severity > 0.66) return token.colorError;
    if (node.severity > 0.33) return token.colorWarning;
    return token.colorBorderSecondary;
  };

  return (
    <div style={{ overflowX: "auto" }}>
      <svg
        width={width + 8}
        height={height + 8}
        role="img"
        aria-label={`Topologi PON bermasalah ${health.oltName}`}
      >
        {edges.map((edge) => (
          <path
            key={edge.id}
            d={edge.path}
            fill="none"
            stroke={token.colorBorderSecondary}
            strokeWidth={1}
          />
        ))}
        {nodes.map((node) => (
          <g
            key={node.id}
            onClick={() => {
              if (node.kind !== "pon") return;
              const [, slot, port] = node.id.split("-");
              onSelectPon(Number(slot), Number(port));
            }}
            style={{ cursor: node.kind === "pon" ? "pointer" : "default" }}
          >
            <rect
              x={node.x}
              y={node.y}
              width={node.width}
              height={node.height}
              rx={8}
              fill={fill(node)}
              stroke={stroke(node)}
            />
            <text
              x={node.x + 12}
              y={node.y + 21}
              fill={token.colorText}
              fontSize={13}
              fontWeight={600}
            >
              {node.label}
            </text>
            <text
              x={node.x + 12}
              y={node.y + 39}
              fill={token.colorTextSecondary}
              fontSize={11}
            >
              {node.detail}
            </text>
          </g>
        ))}
      </svg>
      <div style={{ fontSize: 11, color: token.colorTextSecondary, marginTop: 8 }}>
        Ditampilkan bila kehilangan layanan &gt;{" "}
        {Math.round(health.outageThreshold * 100)}% rentang, atau trap/ONT di
        atas {health.trapThreshold} dan lima kali median OLT ini (
        {health.medianTrapPerOnt}).
      </div>
    </div>
  );
}
```

The caption is required, not decoration: the spec keeps the rule on screen so an
operator can judge it rather than trust it, and the floor of a hundred is a
judgement that has to be visible to be argued with.

- [ ] **Step 9: Run the component tests and the frontend gates**

```bash
cd frontend && npx vitest run src/presentation/components/onts/__tests__/ \
  && npm run lint && npm run format:check && npm run build
```

Expected: all PASS, lint reports no errors.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/domain frontend/src/presentation
git commit -m "feat(onts): draw the PON topology in plain SVG

Depth is fixed at four, so placement is column arithmetic rather than
graph layout, and a diagram library for one page was not worth a
fourteenth runtime dependency.

The arithmetic lives apart from the drawing so it can be tested
directly: checking placement through the DOM tests React, not the
layout. Every figure is drawn as text beside its box, so nothing is
carried by colour alone."
```

---

### Task 4: The PON tab and the drill-down

**Files:**
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Modify: `frontend/src/infrastructure/repositories/OltRepository.ts`
- Modify: `frontend/src/application/hooks/useOlts.ts`
- Modify: `frontend/src/presentation/pages/TroubledOntsPage.tsx`
- Modify: `frontend/src/presentation/pages/__tests__/TroubledOntsPage.test.tsx`

**Interfaces:**
- Consumes: `PonTopology` from Task 3, `useTroubledOnts(hours, oltId, status)` from Task 1.
- Produces: nothing later tasks depend on. This is the last task.

- [ ] **Step 1: Add the endpoint, repository method and hook**

In `frontend/src/infrastructure/http/endpoints.ts`:

```ts
  OLT_PON_HEALTH: (id: string) => `/api/v1/olts/${id}/pon-health`,
```

In `frontend/src/infrastructure/repositories/OltRepository.ts`:

```ts
  async getPonHealth(id: string, hours: number): Promise<PonHealth> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_PON_HEALTH(id), {
      params: { hours },
    });
    return response.data.data;
  }
```

Add `PonHealth` to that file's `import type { … } from "@/domain/entities";` list.

In `frontend/src/application/hooks/useOlts.ts`:

```ts
// The pruned fault tree for one OLT. Disabled until an OLT is chosen: a
// topology of every chassis at once is the thing this view exists to avoid.
export function usePonHealth(oltId: string | undefined, hours: number) {
  return useQuery({
    queryKey: ["olts", oltId, "pon-health", hours],
    queryFn: () => oltRepository.getPonHealth(oltId as string, hours),
    enabled: Boolean(oltId),
    refetchInterval: 60000,
  });
}
```

- [ ] **Step 2: Write the failing page tests**

Append to `frontend/src/presentation/pages/__tests__/TroubledOntsPage.test.tsx`, and add `usePonHealth: () => ({ data: undefined, isLoading: false })` to the `@/application/hooks` mock:

```tsx
it("offers both views on one page", () => {
  render(<TroubledOntsPage />);

  expect(screen.getByRole("tab", { name: /Per Pelanggan/i })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: /Per PON/i })).toBeInTheDocument();
});

it("asks for an OLT before it can draw a topology", () => {
  render(<TroubledOntsPage />);

  fireEvent.click(screen.getByRole("tab", { name: /Per PON/i }));

  // One chassis at a time is the whole reason the view stays readable.
  expect(screen.getByText(/pilih OLT/i)).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd frontend && npx vitest run src/presentation/pages/__tests__/TroubledOntsPage.test.tsx
```

Expected: FAIL — no tab role found.

- [ ] **Step 4: Add the tabs and the drill-down**

In `TroubledOntsPage.tsx`, import `Tabs`, `PonTopology` and `usePonHealth`. Move the OLT `Select` and the range `Radio.Group` above the `Tabs` so both views share them, and leave the status `Select` inside the customer tab's own toolbar. Add:

```tsx
  const [ponFilter, setPonFilter] = useState<{ slot: number; port: number }>();
  const { data: ponHealth, isLoading: ponLoading } = usePonHealth(oltId, hours);

  const handleSelectPon = (slot: number, port: number) => {
    setPonFilter({ slot, port });
    setTab("pelanggan");
  };

  const shown = ponFilter
    ? rows.filter((r) => r.portId === ponFilter.port)
    : rows;
```

Render `<Tabs activeKey={tab} onChange={setTab} items={…} />` with the customer table (using `shown`) under `pelanggan` and, under `pon`, either the topology or a prompt:

```tsx
{oltId ? (
  ponLoading || !ponHealth ? (
    <Skeleton active />
  ) : (
    <PonTopology health={ponHealth} onSelectPon={handleSelectPon} />
  )
) : (
  <Empty description="Pilih OLT untuk melihat topologinya" />
)}
```

When `ponFilter` is set, show a clearable `Tag` above the customer table reading `PON ${ponFilter.port}` whose `onClose` calls `setPonFilter(undefined)`, so the filter is visible and reversible.

- [ ] **Step 5: Run every gate**

```bash
cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```

Expected: all PASS.

- [ ] **Step 6: Check the file sizes**

```bash
wc -l frontend/src/presentation/pages/TroubledOntsPage.tsx
```

If it exceeds 350 lines, extract the customer tab's body into `frontend/src/presentation/components/onts/TroubledOntTab.tsx` and re-run Step 5.

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit -m "feat(onts): put the fault tree beside the subscribers it explains

Two tabs over one filter bar. The OLT and the range belong to both
views; the status belongs only to the subscriber list, because a PON
has no status and a control that promises otherwise is a lie.

Clicking a port carries its subscribers into the other tab, which is
what closes the loop the page was built for: see the bad port, then see
who is on it."
```

---

## Deployment

After Task 4, deploy the way this session has been deploying: build and recreate `api` and `frontend` on the VPS with `--no-deps`, leaving `worker` and `trapd` alone. No migration is involved; nothing in this plan changes the schema.

```bash
git push origin main
ssh radpro 'cd /opt/tikman/src && git pull --ff-only && \
  CV="--env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; \
  sudo docker compose $CV build api frontend && \
  sudo docker compose $CV up -d --no-deps --force-recreate api frontend'
```

Then confirm against production that the rules select what the spec measured: Cariu should surface cards 8 and 9, and Depok should surface the port losing a tenth of the day on one trap per ONT.
