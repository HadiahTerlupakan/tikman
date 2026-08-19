package models

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Site{},
		&OLT{},
		&ONT{},
		&ONTEvent{},
		&Alarm{},
		&AlertRule{},
		&NotificationSettings{},
	)
}
