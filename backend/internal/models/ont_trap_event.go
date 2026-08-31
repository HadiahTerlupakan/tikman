package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ONTTrapEvent is one SNMP trap an OLT sent.
//
// Identity is recorded as the trap reported it rather than as we resolved it. A
// trap naming an ONU that is not in the table is evidence — of a device the
// poller has not seen, or of a serial we store differently — and rewriting it
// into our own terms would destroy exactly that.
//
// The tags must match migrations/34_add_ont_trap_events.sql. AutoMigrate runs
// before the SQL migrations, so a model that disagrees with its migration wins
// and the migration silently does nothing.
type ONTTrapEvent struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OLTID      uuid.UUID `gorm:"type:uuid;not null;index:idx_ont_trap_events_olt_time,priority:1" json:"olt_id"`
	ReceivedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;index:idx_ont_trap_events_olt_time,priority:2,sort:desc" json:"received_at"`

	// column: is stated for both of these. GORM's naming strategy splits TrapOID
	// into trap_o_id and ONUID into on_uid, and because AutoMigrate runs before
	// the SQL migrations it built the table that way first — leaving migration
	// 34's CREATE TABLE IF NOT EXISTS with nothing to do and recording it as
	// applied. The same shape of mistake has now cost this codebase three
	// migrations.
	TrapOID       string `gorm:"column:trap_oid;type:varchar(128);not null" json:"trap_oid"`
	SourceAddress string `gorm:"type:varchar(45);not null" json:"source_address"`

	// No index tag: migration 34 builds a partial one that skips the rows with no
	// serial, and a second index over the same column would be paid for on every
	// insert and read by nothing.
	SerialNumber *string `gorm:"type:varchar(32)" json:"serial_number"`
	ONULabel     *string `gorm:"type:varchar(64)" json:"onu_label"`
	ONUName      *string `gorm:"type:varchar(255)" json:"onu_name"`
	IfIndex      *int64  `json:"if_index"`
	ONUID        *int    `gorm:"column:onu_id" json:"onu_id"`

	Varbinds string `gorm:"type:text;not null" json:"varbinds"`
}

// TableName returns the table name for ONTTrapEvent.
func (ONTTrapEvent) TableName() string {
	return "ont_trap_events"
}

// BeforeCreate assigns the row's id.
func (t *ONTTrapEvent) BeforeCreate(*gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
