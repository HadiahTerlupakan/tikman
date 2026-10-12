package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFlowTablesAreHypertablesWithRealTimeRollups(t *testing.T) {
	db := freshPostgres(t)

	var hypertables int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.hypertables
		WHERE hypertable_schema = current_schema()
		AND hypertable_name IN ('router_flow_minute', 'router_flow_top_ips')`).Scan(&hypertables).Error)
	assert.EqualValues(t, 2, hypertables)

	var realTime int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_schema = current_schema()
		AND view_name IN ('router_flow_5min', 'router_flow_hourly', 'router_flow_daily')
		AND NOT materialized_only`).Scan(&realTime).Error)
	assert.EqualValues(t, 3, realTime)
}

// flowRetention names each flow relation's retention. A rollup's policy sits
// on its materialization hypertable in _timescaledb_internal, so views are
// matched through that.
func flowRetention(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var rows []struct {
		Name      string
		DropAfter string
	}
	require.NoError(t, db.Raw(`SELECT COALESCE(c.view_name, j.hypertable_name) AS name,
			j.config->>'drop_after' AS drop_after
		FROM timescaledb_information.jobs j
		LEFT JOIN timescaledb_information.continuous_aggregates c
			ON c.materialization_hypertable_schema = j.hypertable_schema
			AND c.materialization_hypertable_name = j.hypertable_name
		WHERE j.proc_name = 'policy_retention'
		AND (j.hypertable_schema = current_schema() OR c.view_schema = current_schema())
		AND COALESCE(c.view_name, j.hypertable_name) LIKE 'router_flow%'`).Scan(&rows).Error)
	out := map[string]string{}
	for _, r := range rows {
		out[r.Name] = r.DropAfter
	}
	return out
}

func TestFlowRetentionMatchesTheSpec(t *testing.T) {
	db := freshPostgres(t)

	assert.Equal(t, map[string]string{
		"router_flow_minute":  "7 days",
		"router_flow_top_ips": "7 days",
		"router_flow_5min":    "90 days",
		"router_flow_hourly":  "2 years",
	}, flowRetention(t, db), "the daily rollup has no retention: it is kept forever")
}

func TestFlowRollupsAddUpTheMinutes(t *testing.T) {
	db := freshPostgres(t)
	routerID, upstream := uuid.New(), uuid.New()
	hour := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Exec(`INSERT INTO router_flow_minute
			(time, router_id, upstream_interface_id, direction, asn, bytes, packets)
			VALUES (?, ?, ?, 'download', 64496, 1000, 10)`,
			hour.Add(time.Duration(i)*time.Minute), routerID, upstream).Error)
	}

	buckets := map[string]time.Time{
		"router_flow_5min":   hour,
		"router_flow_hourly": hour,
		"router_flow_daily":  hour.Truncate(24 * time.Hour),
	}
	for view, bucket := range buckets {
		require.NoError(t, db.Exec("CALL refresh_continuous_aggregate('"+view+"', NULL, NULL)").Error, view)
		var bytes int64
		require.NoError(t, db.Raw("SELECT bytes FROM "+view+" WHERE router_id = ? AND bucket = ?",
			routerID, bucket).Scan(&bytes).Error, view)
		assert.EqualValues(t, 3000, bytes, view)
	}
}

func TestFlowSamplingAndDirectionAreChecked(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)

	for _, ratio := range []int{1, 100, 10000} {
		assert.NoError(t, db.Exec(`UPDATE mikrotik_routers SET flow_sampling = ? WHERE id = ?`, ratio, routerID).Error, ratio)
	}
	for _, ratio := range []int{0, 10001} {
		assert.Error(t, db.Exec(`UPDATE mikrotik_routers SET flow_sampling = ? WHERE id = ?`, ratio, routerID).Error, ratio)
	}
	assert.Error(t, db.Exec(`INSERT INTO router_flow_minute
		(time, router_id, upstream_interface_id, direction, asn, bytes, packets)
		VALUES (now(), ?, ?, 'sideways', 64496, 1, 1)`, routerID, uuid.New()).Error)
}

// Routers added before flows existed collect nothing until an Admin turns it
// on, and then sample at the suggested 1:100.
func TestExistingRoutersDefaultToNoFlowsAtOneInAHundred(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)

	var row struct {
		FlowEnabled  bool
		FlowSampling int
	}
	require.NoError(t, db.Raw(`SELECT flow_enabled, flow_sampling FROM mikrotik_routers WHERE id = ?`,
		routerID).Scan(&row).Error)
	assert.False(t, row.FlowEnabled)
	assert.Equal(t, 100, row.FlowSampling)
}

// A routed block belongs to the interface its gateway sits behind; with that
// interface gone the block means nothing, and it goes too.
func TestRoutedPrefixesGoWithTheirInterface(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)
	interfaceID := insertTestInterface(t, db, routerID)
	require.NoError(t, db.Exec(`INSERT INTO router_routed_prefixes (id, router_id, prefix, interface_id, updated_at)
		VALUES (?, ?, '203.0.113.64/27', ?, now())`, uuid.New(), routerID, interfaceID).Error)

	require.NoError(t, db.Exec(`DELETE FROM router_interfaces WHERE id = ?`, interfaceID).Error)

	var left int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM router_routed_prefixes WHERE router_id = ?`, routerID).Scan(&left).Error)
	assert.Zero(t, left)
}

// cidr refuses host bits, which is why discovery stores the masked prefix.
func TestRoutedPrefixMustBeANetwork(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)
	interfaceID := insertTestInterface(t, db, routerID)

	err := db.Exec(`INSERT INTO router_routed_prefixes (id, router_id, prefix, interface_id, updated_at)
		VALUES (?, ?, '203.0.113.70/27', ?, now())`, uuid.New(), routerID, interfaceID).Error

	assert.Error(t, err)
}
