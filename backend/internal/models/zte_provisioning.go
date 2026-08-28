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
	// WANMode says whether the OLT writes the ONU's WAN over OMCI or leaves it
	// to be set up on the ONT itself; WANIPMode is how that WAN gets its address
	// and only applies to the former.
	WANMode       string `json:"wan_mode"`
	WANIPMode     string `json:"wan_ip_mode"`
	VLANProfile   string `json:"vlan_profile"`
	PPPoEUsername string `json:"pppoe_username"`
	PPPoEPassword string `json:"pppoe_password"`
}

// VLAN tagging modes on the UNI side of an ONU.
const (
	ZTEVLANModeTag   = "tag"
	ZTEVLANModeUntag = "untag"
)

// Service types the form offers. Bridge carries no OMCI WAN at all.
const (
	ZTEServiceInternet = "internet"
	ZTEServiceBridge   = "bridge"
)

// Where the WAN is configured.
const (
	ZTEWANModeWANIP       = "wan_ip"
	ZTEWANModeSetupViaONT = "setup_via_ont"
)

// How an OMCI-configured WAN obtains its address. The OLT's own help lists
// exactly these three.
const (
	ZTEWANIPModePPPoE  = "pppoe"
	ZTEWANIPModeDHCP   = "dhcp"
	ZTEWANIPModeStatic = "static"
)
