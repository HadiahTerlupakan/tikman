package models

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Site{},
		&OLT{},
		&ONT{},
		&ONTEvent{},
		&AuditLog{},
		&ConfigTemplate{},
		&ProvisioningJob{},
		&BatchJob{},
		&WireGuardServer{},
		&WireGuardPeer{},
		&WorkerHeartbeat{},
		&AppSetting{},
		&OLTPollJob{},
		&ONTTrapEvent{},
		&ODC{},
		&ODCFeed{},
		&ODP{},
		&WAAccount{},
		&CSConversation{},
		&CSMessage{},
		&CSQuickReply{},
	)
}
