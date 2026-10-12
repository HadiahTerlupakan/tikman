package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertTestRouter(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO mikrotik_routers
		(id, name, host, snmp_community, snmp_port, api_username, api_password, api_port,
		 tls_fingerprint, enabled, status, api_status, created_at, updated_at)
		VALUES (?, 'border', '192.0.2.1', 'x', 161, 'tikman', 'x', 8443, ?, true,
		 'unknown', 'unknown', now(), now())`,
		id, "0000000000000000000000000000000000000000000000000000000000000000").Error)
	return id
}

func insertTestInterface(t *testing.T, db *gorm.DB, routerID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO router_interfaces
		(id, router_id, name, comment, type, disabled, oper_status, addresses, created_at, updated_at)
		VALUES (?, ?, 'vlan-ix', '', 'vlan', false, 'up', '', now(), now())`, id, routerID).Error)
	return id
}

func insertTestSession(t *testing.T, db *gorm.DB, routerID, interfaceID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO router_bgp_sessions
		(id, router_id, name, remote_address, remote_as, local_address, state,
		 uptime_seconds, prefix_count, interface_id, updated_at)
		VALUES (?, ?, 'peer-ix', '198.51.100.2', 64501, '198.51.100.3', 'established', 0, 0, ?, now())`,
		id, routerID, interfaceID).Error)
	return id
}

// Deleting a router has to take its interfaces, sessions, events and queue rows
// with it. SQLite builds no foreign keys, so only Postgres can show this.
func TestDeletingARouterRemovesWhatHangsOffIt(t *testing.T) {
	db := freshPostgres(t)

	routerID := insertTestRouter(t, db)
	insertTestSession(t, db, routerID, insertTestInterface(t, db, routerID))
	require.NoError(t, db.Exec(`INSERT INTO router_bgp_events
		(id, router_id, session_name, from_state, to_state, occurred_at)
		VALUES (?, ?, 'peer-ix', 'established', 'down', now())`, uuid.New(), routerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO router_poll_jobs (router_id, kind, due_at)
		VALUES (?, 'bgp', now())`, routerID).Error)

	require.NoError(t, db.Exec(`DELETE FROM mikrotik_routers WHERE id = ?`, routerID).Error)

	for _, table := range []string{"router_interfaces", "router_bgp_sessions", "router_bgp_events", "router_poll_jobs"} {
		var left int64
		require.NoError(t, db.Raw("SELECT count(*) FROM "+table+" WHERE router_id = ?", routerID).Scan(&left).Error)
		assert.Zero(t, left, table)
	}
}

// An interface that disappears in discovery must not take its BGP session with
// it; the session stays and loses only its interface.
func TestDeletingAnInterfaceKeepsItsSession(t *testing.T) {
	db := freshPostgres(t)

	routerID := insertTestRouter(t, db)
	interfaceID := insertTestInterface(t, db, routerID)
	sessionID := insertTestSession(t, db, routerID, interfaceID)

	require.NoError(t, db.Exec(`DELETE FROM router_interfaces WHERE id = ?`, interfaceID).Error)

	var unlinked int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM router_bgp_sessions
		WHERE id = ? AND interface_id IS NULL`, sessionID).Scan(&unlinked).Error)
	assert.EqualValues(t, 1, unlinked)
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

	var realTime int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_schema = current_schema() AND view_name LIKE 'router_interface_metrics_%'
		AND NOT materialized_only`).Scan(&realTime).Error)
	assert.EqualValues(t, 2, realTime, "both rollups answer up to the latest sample")
}

func insertRouterRow(db *gorm.DB, protocolColumn bool, protocol string) (uuid.UUID, error) {
	id := uuid.New()
	columns := "id, name, host, snmp_community, snmp_port, api_username, api_password, api_port, enabled, tls_fingerprint, status, api_status, created_at, updated_at"
	values := "?, ?, '192.0.2.1', 'x', 161, 'u', 'p', 8728, true, '', 'unknown', 'unknown', now(), now()"
	args := []interface{}{id, "router-" + id.String()}
	if protocolColumn {
		columns += ", api_protocol"
		values += ", ?"
		args = append(args, protocol)
	}
	return id, db.Exec("INSERT INTO mikrotik_routers ("+columns+") VALUES ("+values+")", args...).Error
}

func TestDatabaseRefusesAnUnknownRouterProtocol(t *testing.T) {
	db := freshPostgres(t)
	for _, protocol := range []string{"rest", "api-ssl", "api"} {
		_, err := insertRouterRow(db, true, protocol)
		require.NoError(t, err, protocol)
	}
	_, err := insertRouterRow(db, true, "telnet")
	assert.Error(t, err)
}

// Routers added before the choice existed were all read over REST.
func TestRoutersWithoutAProtocolReadAsREST(t *testing.T) {
	db := freshPostgres(t)
	id, err := insertRouterRow(db, false, "")
	require.NoError(t, err)

	var protocol string
	require.NoError(t, db.Raw("SELECT api_protocol FROM mikrotik_routers WHERE id = ?", id).Scan(&protocol).Error)
	assert.Equal(t, "rest", protocol)
}
