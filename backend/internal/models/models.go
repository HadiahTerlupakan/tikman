package models

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	// Only migrate core models needed for frontend
	// ONT, ServiceProfile, LineProfile, AuditLog will be added in Phase 2
	return db.AutoMigrate(
		&User{},
		&Site{},
		&OLT{},
	)
}
