package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// WalkONTIPAddresses walks the IP address table for all ONTs
func WalkONTIPAddresses(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	ipAddresses := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ipOID := BaseOID2 + OnuIPAddressPrefix
	log.Printf("[IPAddress] Starting IP address walk on OID: %s", ipOID)

	err = client.Walk(ipOID, func(pdu gosnmp.SnmpPDU) error {
		trimmed := strings.TrimPrefix(pdu.Name, ".")
		baseTrimmed := strings.TrimPrefix(ipOID, ".")
		if !strings.HasPrefix(trimmed, baseTrimmed+".") {
			return nil
		}

		parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
		if len(parts) < 3 {
			return nil
		}

		ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return nil
		}
		onuID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		interfaceIdx := parts[2]

		if interfaceIdx != "1" {
			return nil
		}

		slot, port, ok := decodeOnuTypeIfIndex(uint32(ifIndexStr))
		if !ok {
			return nil
		}

		loc := ONTLocation{Slot: slot, Port: port, ONTID: onuID}

		if ipAddr, ok := pdu.Value.(string); ok && ipAddr != "0.0.0.0" && ipAddr != "" {
			ipAddresses[loc] = ipAddr
		}

		return nil
	})

	if err != nil {
		log.Printf("[IPAddress] IP address walk failed: %v", err)
		return ipAddresses, err
	}

	log.Printf("[IPAddress] Retrieved IP addresses for %d ONTs", len(ipAddresses))
	return ipAddresses, nil
}

// WalkONTMACAddresses walks the MAC address table for all ONTs
func WalkONTMACAddresses(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	macAddresses := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	macOID := BaseOID2 + OnuMACAddressPrefix
	log.Printf("[MACAddress] Starting MAC address walk on OID: %s", macOID)

	err = client.Walk(macOID, func(pdu gosnmp.SnmpPDU) error {
		trimmed := strings.TrimPrefix(pdu.Name, ".")
		baseTrimmed := strings.TrimPrefix(macOID, ".")
		if !strings.HasPrefix(trimmed, baseTrimmed+".") {
			return nil
		}

		parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
		if len(parts) < 3 {
			return nil
		}

		ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return nil
		}
		onuID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}
		interfaceIdx := parts[2]

		if interfaceIdx != "1" {
			return nil
		}

		slot, port, ok := decodeOnuTypeIfIndex(uint32(ifIndexStr))
		if !ok {
			return nil
		}

		loc := ONTLocation{Slot: slot, Port: port, ONTID: onuID}

		if macBytes, ok := pdu.Value.([]byte); ok && len(macBytes) == 6 {
			macAddr := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				macBytes[0], macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5])
			macAddresses[loc] = macAddr
		}

		return nil
	})

	if err != nil {
		log.Printf("[MACAddress] MAC address walk failed: %v", err)
		return macAddresses, err
	}

	log.Printf("[MACAddress] Retrieved MAC addresses for %d ONTs", len(macAddresses))
	return macAddresses, nil
}

// WalkONTHardwareVersions walks the hardware version table for all ONTs
func WalkONTHardwareVersions(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	hwVersions := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	hwOID := BaseOID2 + OnuHardwareVersionPrefix
	log.Printf("[HardwareVersion] Starting hardware version walk on OID: %s", hwOID)

	err = client.Walk(hwOID, func(pdu gosnmp.SnmpPDU) error {
		trimmed := strings.TrimPrefix(pdu.Name, ".")
		baseTrimmed := strings.TrimPrefix(hwOID, ".")
		if !strings.HasPrefix(trimmed, baseTrimmed+".") {
			return nil
		}

		parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
		if len(parts) < 2 {
			return nil
		}

		ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return nil
		}
		onuID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil
		}

		slot, port, ok := decodeOnuTypeIfIndex(uint32(ifIndexStr))
		if !ok {
			return nil
		}

		loc := ONTLocation{Slot: slot, Port: port, ONTID: onuID}

		hwVer := ExtractName(pdu.Value)
		if hwVer != "" {
			hwVersions[loc] = hwVer
		}

		return nil
	})

	if err != nil {
		log.Printf("[HardwareVersion] Hardware version walk failed: %v", err)
		return hwVersions, err
	}

	log.Printf("[HardwareVersion] Retrieved hardware versions for %d ONTs", len(hwVersions))
	return hwVersions, nil
}

// walkONTMetricsForPort queries optical metrics for ONTs on a specific port
// UNUSED: func walkONTMetricsForPort(client *gosnmp.GoSNMP, ipAddress, community string, snmpPort int,
// UNUSED: 	slot, port int, ontLocations []ONTLocation) (map[ONTLocation]*ONTMetrics, error) {
// UNUSED: 
// UNUSED: 	log.Printf("[Metrics] Querying metrics for slot %d port %d (%d ONTs)", slot, port, len(ontLocations))
// UNUSED: 	metrics := make(map[ONTLocation]*ONTMetrics)
// UNUSED: 
// UNUSED: 	for _, loc := range ontLocations {
// UNUSED: 		onMetrics := &ONTMetrics{}
// UNUSED: 
// UNUSED: 		// Query RX power using ZXGPON-MIB table
// UNUSED: 		zxIfIndex := encodeZxGponIfIndex(1, slot, port)
// UNUSED: 		rxOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_RX_POWER_TABLE, zxIfIndex, loc.ONTID)
// UNUSED: 
// UNUSED: 		result, err := client.Get([]string{rxOID})
// UNUSED: 		if err == nil && len(result.Variables) > 0 {
// UNUSED: 			if val, ok := toInt64(result.Variables[0].Value); ok {
// UNUSED: 				log.Printf("[Metrics] ONT %d RX raw value: %d", loc.ONTID, val)
// UNUSED: 				if val < 30000 {
// UNUSED: 					rxDbm := decodeZxGponPower(val)
// UNUSED: 					onMetrics.RxPower = rxDbm
// UNUSED: 					log.Printf("[Metrics] ONT %d RX: %.2f dBm", loc.ONTID, *rxDbm)
// UNUSED: 				} else {
// UNUSED: 					log.Printf("[Metrics] ONT %d RX: no signal (val=%d)", loc.ONTID, val)
// UNUSED: 				}
// UNUSED: 			}
// UNUSED: 		} else if err != nil {
// UNUSED: 			log.Printf("[Metrics] RX power query failed for ONT %d: %v", loc.ONTID, err)
// UNUSED: 		}
// UNUSED: 
// UNUSED: 		// Query TX power
// UNUSED: 		txOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_TX_POWER_TABLE, zxIfIndex, loc.ONTID)
// UNUSED: 		result, err = client.Get([]string{txOID})
// UNUSED: 		if err == nil && len(result.Variables) > 0 {
// UNUSED: 			if val, ok := toInt64(result.Variables[0].Value); ok && val < 30000 {
// UNUSED: 				txDbm := decodeZxGponPower(val)
// UNUSED: 				onMetrics.TxPower = txDbm
// UNUSED: 			}
// UNUSED: 		} else if err != nil {
// UNUSED: 			log.Printf("[Metrics] TX power query failed for ONT %d: %v", loc.ONTID, err)
// UNUSED: 		}
// UNUSED: 
// UNUSED: 		// Query distance
// UNUSED: 		distOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_DISTANCE_TABLE, zxIfIndex, loc.ONTID)
// UNUSED: 		result, err = client.Get([]string{distOID})
// UNUSED: 		if err == nil && len(result.Variables) > 0 {
// UNUSED: 			if val, ok := toInt64(result.Variables[0].Value); ok && val > 0 && val < 30000 {
// UNUSED: 				onMetrics.Distance = int(val)
// UNUSED: 			}
// UNUSED: 		} else if err != nil {
// UNUSED: 			log.Printf("[Metrics] Distance query failed for ONT %d: %v", loc.ONTID, err)
// UNUSED: 		}
// UNUSED: 
// UNUSED: 		metrics[loc] = onMetrics
// UNUSED: 	}
// UNUSED: 
// UNUSED: 	log.Printf("[Metrics] Retrieved metrics for %d ONTs on slot %d port %d", len(metrics), slot, port)
// UNUSED: 	return metrics, nil
// UNUSED: }
