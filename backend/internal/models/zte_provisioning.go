package models

import "github.com/google/uuid"

// ZTEONUIDMode controls whether the OLT assigns an ONU ID or the request supplies it.
type ZTEONUIDMode string

const (
	ZTEONUIDAuto   ZTEONUIDMode = "auto"
	ZTEONUIDCustom ZTEONUIDMode = "custom"
)

// ZTEGPONRegisterRequest contains the identity and Internet service settings for a ZTE GPON ONU.
type ZTEGPONRegisterRequest struct {
	OLTID           uuid.UUID    `json:"olt_id"`
	Card            int          `json:"card"`
	PON             int          `json:"pon"`
	ONUIDMode       ZTEONUIDMode `json:"onu_id_mode"`
	ONUID           int          `json:"onu_id"`
	SerialNumber    string       `json:"serial_number"`
	ONUType         string       `json:"onu_type"`
	UseVEIP         bool         `json:"use_veip"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	ServiceEnabled  bool         `json:"service_enabled"`
	VLANMode        string       `json:"vlan_mode"`
	ServiceType     string       `json:"service_type"`
	VLANID          int          `json:"vlan_id"`
	DownloadProfile string       `json:"download_profile"`
	UploadProfile   string       `json:"upload_profile"`
	WANMode         string       `json:"wan_mode"`
	VLANProfile     string       `json:"vlan_profile"`
	PPPoEUsername   string       `json:"pppoe_username"`
	PPPoEPassword   string       `json:"pppoe_password"`
}
