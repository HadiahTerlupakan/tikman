package connectivity

// ============================================================================
// ZTE C300/C320 SNMP OIDs - VERIFIED AGAINST https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte
// Tested against ZTE C300 V2.1.0 and C320 V2.1.0 production hardware
// ============================================================================
//
// TWO INDEX SPACES:
//
// 1. ONU-ID Space (.1082.x) - for name, serial, status, description, distance
//    Formula: OnuIDIfIndexBase + slot*OnuIDSlotStride + pon*OnuIDIncrement
//           = 0x11010000 + slot*0x100 + pon
//           = 285278208 + slot*256 + pon
//
// 2. TYPE Space (.1012.x) - for onu type, tx power, ip address
//    Formula: OnuTypeIfIndexBase + slot*OnuTypeSlotStride + pon*OnuTypeIncrement
//           = 0x10000000 + slot*0x10000 + pon*0x100
//
// OID CONSTANTS:
// - .1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18 = Serial number table
// - .1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2  = Name/description table
// - .1.3.6.1.4.1.3902.1012.3.28.2.1.4        = Phase state/status table
// - .1.3.6.1.4.1.3902.1012.3.50.12.1.1.10   = RX optical power
// - .1.3.6.1.4.1.3902.1012.3.50.12.1.1.14   = TX optical power
// - .1.3.6.1.4.1.3902.1012.3.11.4.1.2       = Distance in meters
// ============================================================================

const (
	// BaseOIDS for two different index spaces
	BaseOID1 = ".1.3.6.1.4.1.3902.1082" // ONU-ID space (serial/status/distance)
	BaseOID2 = ".1.3.6.1.4.1.3902.1012" // TYPE space (type/txpower/ip/name/description)
	BaseOID3 = ".1.3.6.1.4.1.3902.1015" // Vendor-specific cumulative counters (Type 3 index)

	// Common OID prefixes (same for all board/PON combinations)
	OnuIDNamePrefix              = ".3.28.1.1.2"        // ONU name (phone-customer)
	OnuDescriptionPrefix         = ".3.28.1.1.3"        // ONU description/location
	OnuSerialNumberPrefix        = ".500.10.2.3.3.1.18" // ONU serial number
	OnuRxPowerPrefix             = ".500.20.2.2.2.1.10" // RX optical power
	OnuTxPowerPrefix             = ".3.50.12.1.1.14"    // TX optical power
	OnuStatusIDPrefix            = ".500.10.2.3.8.1.4"  // ONU phase state/status
	OnuLastOnlineTimePrefix      = ".500.10.2.3.8.1.5"  // Last online time
	OnuLastOfflineTimePrefix     = ".500.10.2.3.8.1.6"  // Last offline time
	OnuLastOfflineReasonPrefix   = ".500.10.2.3.8.1.7"  // Last offline reason
	OnuGponOpticalDistancePrefix = ".500.10.2.3.10.1.2" // Optical distance

	// Device information OID prefixes
	OnuTypePrefix            = ".3.28.1.1.5"     // ONU device type/model
	OnuHardwareVersionPrefix = ".3.50.11.2.1.2"  // Hardware version (equipment version)
	OnuSoftwareVersionPrefix = ".3.28.1.1.7"     // Software/firmware version (not available on C300)
	OnuEquipmentIDPrefix     = ".3.50.11.2.1.9"  // Equipment ID (model+firmware, e.g. F660V9)
	OnuIPAddressPrefix       = ".3.50.16.1.1.10" // Management IP address (TYPE space, 2 indices)
	OnuMACAddressPrefix      = ".3.50.16.1.1.3"  // MAC address (TYPE space, 2 indices)

	// Health monitoring OID prefixes
	OnuTemperaturePrefix   = ".3.50.12.1.1.1" // Temperature in Celsius
	OnuVoltagePrefix       = ".3.50.12.1.1.2" // Supply voltage
	OnuTxBiasCurrentPrefix = ".3.50.12.1.1.3" // TX bias current in mA

	// Traffic statistics OID prefixes
	OnuRxBytesPrefix   = ".3.50.12.1.1.18" // RX bytes counter (fragment, oscillating)
	OnuTxBytesPrefix   = ".3.50.12.1.1.19" // TX bytes counter (fragment, oscillating)
	OnuRxPacketsPrefix = ".3.50.12.1.1.10" // RX packets counter
	OnuTxPacketsPrefix = ".3.50.12.1.1.14" // TX packets counter
	OnuRxErrorsPrefix  = ".3.50.12.1.1.20" // RX errors counter
	OnuTxErrorsPrefix  = ".3.50.12.1.1.22" // TX errors counter

	// Live traffic rate gauges (zxGponOntMgmt). These are Gauge32 values in
	// bytes/second read directly from the OLT, not cumulative counters — the
	// fragment counters above oscillate and their deltas are meaningless.
	// Index space is ONU-ID (0x1101SSPP), same as the serial-number table.
	OnuRxOctetRatePrefix = ".500.4.2.2.2.1.3"  // zxAnPonOnuIfRxOctetRate (upload, ONU→OLT)
	OnuTxOctetRatePrefix = ".500.4.2.2.2.1.46" // zxAnPonOnuIfTxOctetRate (download, OLT→ONU)

	// IfIndex encoding bases and per-slot strides (verified live against real hardware)
	OnuIDIfIndexBase   = 285278208 // 0x11010000 — ONU-ID space prefix 0x11, shelf 1
	OnuIDSlotStride    = 256       // 0x100      — per-slot stride (ONU-ID space)
	OnuTypeIfIndexBase = 268435456 // 0x10000000 — TYPE space prefix 0x10
	OnuTypeSlotStride  = 65536     // 0x10000    — per-slot stride (TYPE space)

	// Per-PON increments within a slot
	OnuIDIncrement   = 1   // ONU-ID space: each PON increments by 1
	OnuTypeIncrement = 256 // TYPE space: each PON increments by 256

	// MaxBoardID / MaxPonID bound the valid physical slot and PON-port range
	MaxBoardID = 30
	MaxPonID   = 16

	// For backward compatibility with existing code using different OID names
	OID_ZXAN_REGISTERED_ONU_SERIAL_TABLE = BaseOID1 + OnuSerialNumberPrefix
	OID_ZXAN_REGISTERED_ONU_NAME_TABLE   = BaseOID1 + OnuIDNamePrefix
	OID_ZXAN_ONU_PHASE_STATE_TABLE       = BaseOID2 + ".3.28.2.1.4"

	// ZTE-AN-MIB branch for optical metrics (BaseOID2 space for RX/TX power)
	OID_ZXGPON_ONU_RX_POWER_TABLE = BaseOID2 + ".3.50.12.1.1.10"
	OID_ZXGPON_ONU_TX_POWER_TABLE = BaseOID2 + OnuTxPowerPrefix
	OID_ZXGPON_ONU_DISTANCE_TABLE = BaseOID2 + ".3.11.4.1.2"

	// Device information OIDs
	OID_ZXGPON_ONU_TYPE_TABLE             = BaseOID2 + OnuTypePrefix
	OID_ZXGPON_ONU_HARDWARE_VERSION_TABLE = BaseOID2 + OnuHardwareVersionPrefix
	OID_ZXGPON_ONU_SOFTWARE_VERSION_TABLE = BaseOID2 + OnuSoftwareVersionPrefix
	OID_ZXGPON_ONU_EQUIPMENT_ID_TABLE     = BaseOID2 + OnuEquipmentIDPrefix
	OID_ZXGPON_ONU_IP_ADDRESS_TABLE       = BaseOID2 + OnuIPAddressPrefix
	OID_ZXGPON_ONU_MAC_ADDRESS_TABLE      = BaseOID2 + OnuMACAddressPrefix

	// Health monitoring OIDs
	OID_ZXGPON_ONU_TEMPERATURE_TABLE     = BaseOID2 + OnuTemperaturePrefix
	OID_ZXGPON_ONU_VOLTAGE_TABLE         = BaseOID2 + OnuVoltagePrefix
	OID_ZXGPON_ONU_TX_BIAS_CURRENT_TABLE = BaseOID2 + OnuTxBiasCurrentPrefix

	// Traffic statistics OIDs
	OID_ZXGPON_ONU_RX_BYTES_TABLE   = BaseOID2 + OnuRxBytesPrefix
	OID_ZXGPON_ONU_TX_BYTES_TABLE   = BaseOID2 + OnuTxBytesPrefix
	OID_ZXGPON_ONU_RX_PACKETS_TABLE = BaseOID2 + OnuRxPacketsPrefix
	OID_ZXGPON_ONU_TX_PACKETS_TABLE = BaseOID2 + OnuTxPacketsPrefix
	OID_ZXGPON_ONU_RX_ERRORS_TABLE  = BaseOID2 + OnuRxErrorsPrefix
	OID_ZXGPON_ONU_TX_ERRORS_TABLE  = BaseOID2 + OnuTxErrorsPrefix

	// Live traffic rate gauges
	OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE = BaseOID1 + OnuRxOctetRatePrefix
	OID_ZXGPON_ONU_TX_OCTET_RATE_TABLE = BaseOID1 + OnuTxOctetRatePrefix
)

// ONTLocation identifies an ONT by its physical position on the OLT, decoded
// from the ZXGPON ifIndex reported by the device itself.
type ONTLocation struct {
	Slot  int
	Port  int
	ONTID int
}
