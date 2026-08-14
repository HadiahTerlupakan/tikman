package models

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Site{},
		&OLT{},
		&ServiceProfile{},
		&LineProfile{},
		&ONT{},
		&AuditLog{},
	)
}
