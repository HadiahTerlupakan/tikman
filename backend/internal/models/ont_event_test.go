package models

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ONTEvent.ONTID holds an ONT's uuid while ONT.ONTID holds the integer ONU
// number on the PON port. With the same field name on both sides and no
// references: tag, AutoMigrate resolved the belongs-to backwards and emitted
//
//	onts.ont_id -> ont_events.id
//
// Every attempt to register a new ONT then failed with a foreign key violation,
// because an ONU number is not an event id. This pins the direction: onts must
// not carry a foreign key into ont_events.
func TestAutoMigrateDoesNotPointONTsAtEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	var ddl string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='onts'").Scan(&ddl).Error; err != nil {
		t.Fatalf("read onts DDL: %v", err)
	}
	if ddl == "" {
		t.Fatal("no DDL for onts")
	}

	if strings.Contains(ddl, "ont_events") {
		t.Errorf("onts references ont_events, so the relation is reversed:\n%s", ddl)
	}

	// The correct constraint (ont_events.ont_id -> onts.id) is created by
	// migrations/04_create_ont_events_table.sql rather than by AutoMigrate, so it
	// is deliberately not asserted here - this test guards the direction GORM
	// gets to invent.
}
