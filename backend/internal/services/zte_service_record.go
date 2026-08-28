package services

import (
	"encoding/json"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// zteServiceFromRequest is the service a job just applied, in the shape the
// configure form reads back.
func zteServiceFromRequest(req models.ZTEGPONRegisterRequest) connectivity.ZTEONUService {
	return connectivity.ZTEONUService{
		ONUType:       req.ONUType,
		VLANID:        req.VLANID,
		VLANMode:      req.VLANMode,
		ServiceType:   req.ServiceType,
		TCONTProfile:  req.DownloadProfile,
		WANMode:       req.WANMode,
		WANIPMode:     req.WANIPMode,
		VLANProfile:   req.VLANProfile,
		UseVEIP:       req.UseVEIP,
		PPPoEUsername: req.PPPoEUsername,
		PPPoEPassword: req.PPPoEPassword,
	}
}

// recordZTEService stores what a successful job applied, so the configure form
// opens on it straight away.
//
// This used to wait for the discovery poll to read the service back off the
// OLT, which is gated at thirty minutes: an ONU configured a minute after a
// poll had no stored service for the next twenty-nine, and reopening its form
// showed nothing. The job already knows what it sent.
func recordZTEService(db *gorm.DB, encryptionKey []byte, ont models.ONT, req models.ZTEGPONRegisterRequest) error {
	service := zteServiceFromRequest(req)
	encoded, err := json.Marshal(service)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"service_config":    datatypes.JSON(encoded),
		"service_config_at": time.Now(),
	}
	// Sealed with the same key as the OLT's own credentials. The marshalled
	// service deliberately carries no password field.
	if service.PPPoEPassword != "" && len(encryptionKey) > 0 {
		sealed, err := utils.Encrypt(service.PPPoEPassword, string(encryptionKey))
		if err != nil {
			return err
		}
		updates["pppoe_password"] = sealed
	}

	return db.Model(&models.ONT{}).Where("id = ?", ont.ID).Updates(updates).Error
}
