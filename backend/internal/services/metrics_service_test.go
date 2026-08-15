package services

import (
	"testing"
)

func TestMetricsService_StoreMetrics(t *testing.T) {
	// This test requires PostgreSQL with TimescaleDB extension
	// SQLite used in setupTestDB doesn't support TimescaleDB hypertables
	t.Skip("Integration test - requires PostgreSQL with TimescaleDB")
}

func TestMetricsService_GetMetricsHistory(t *testing.T) {
	// This test requires PostgreSQL with TimescaleDB extension
	// SQLite used in setupTestDB doesn't support TimescaleDB hypertables
	t.Skip("Integration test - requires PostgreSQL with TimescaleDB")
}
