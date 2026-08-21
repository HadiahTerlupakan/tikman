package utils

// EncodeZxGponIfIndex calculates ZXGPON-MIB ifIndex encoding
// Verified per NetManeger: ifIndex = (0x10 << 24) | (slot << 16) | (port << 8)
// Returns uint32 encoded ifIndex for ZTE C300 C320 GPON OLT
func EncodeZxGponIfIndex(slot, port int) uint32 {
	return (0x10 << 24) | (uint32(slot&0xff) << 16) | (uint32(port&0xff) << 8)
}

// DecodeZxGponIfIndex decodes ZXGPON-MIB ifIndex back to slot and port
func DecodeZxGponIfIndex(ifIndex uint32) (slot, port int, ok bool) {
	if (ifIndex>>24)&0xff != 0x10 {
		return 0, 0, false
	}
	return int((ifIndex >> 16) & 0xff), int((ifIndex >> 8) & 0xff), true
}

// StatusMap maps SNMP phase state integer values to ONT status strings
// Based on ZTE C300 V2.1.0 verified values from ZXGPON-MIB (.1012.3.28.2.1.4):
//   3 = working/online - ONU teregistrasi dan lewat trafik
//   4 = dying_gasp     - ONU baru kehilangan power
//   6 = offline        - ONU mati atau kabel putus
//   1 = los            - Loss of Signal
//   other = unknown    - nilai tidak dikenali
func StatusMap(phaseState int) string {
	switch phaseState {
	case 3:
		return "online"
	case 4:
		return "dying_gasp"
	case 6:
		return "offline"
	case 1:
		return "los"
	default:
		return "unknown"
	}
}


