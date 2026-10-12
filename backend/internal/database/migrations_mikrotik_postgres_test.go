package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deleting a router has to take its interfaces, sessions, events and queue rows
// with it. SQLite builds no foreign keys, so only Postgres can show this.
func TestDeletingARouterRemovesWhatHangsOffIt(t *testing.T) {
	db := freshPostgres(t)

	routerID := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO mikrotik_routers
		(id, name, host, snmp_community, snmp_port, api_username, api_password, api_port,
		 tls_fingerprint, enabled, status, api_status, created_at, updated_at)
		VALUES (?, 'border', '192.0.2.1', 'x', 161, 'tikman', 'x', 8443, ?, true,
		 'unknown', 'unknown', now(), now())`,
		routerID, "0000000000000000000000000000000000000000000000000000000000000000").Error)
	require.NoError(t, db.Exec(`INSERT INTO router_interfaces
		(id, router_id, name, comment, type, disabled, oper_status, addresses, created_at, updated_at)
		VALUES (?, ?, 'vlan-ix', '', 'vlan', false, 'up', '', now(), now())`, uuid.New(), routerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO router_poll_jobs (router_id, kind, due_at)
		VALUES (?, 'bgp', now())`, routerID).Error)

	require.NoError(t, db.Exec(`DELETE FROM mikrotik_routers WHERE id = ?`, routerID).Error)

	for _, table := range []string{"router_interfaces", "router_poll_jobs"} {
		var left int64
		require.NoError(t, db.Raw("SELECT count(*) FROM "+table+" WHERE router_id = ?", routerID).Scan(&left).Error)
		assert.Zero(t, left, table)
	}
}

func TestRouterInterfaceMetricsIsAHypertableWithRollups(t *testing.T) {
	db := freshPostgres(t)

	// Scoped to this schema: the mapping backfill tests build their own copy
	// of every migration, and TimescaleDB's views list both.
	var hypertables int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.hypertables
		WHERE hypertable_schema = current_schema() AND hypertable_name = 'router_interface_metrics'`).Scan(&hypertables).Error)
	assert.EqualValues(t, 1, hypertables)

	var views int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_schema = current_schema() AND view_name IN ('router_interface_metrics_5min', 'router_interface_metrics_hourly')`).Scan(&views).Error)
	assert.EqualValues(t, 2, views)
}
