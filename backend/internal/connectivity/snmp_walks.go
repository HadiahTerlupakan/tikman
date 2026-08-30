package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// zteWalkIPAddresses walks the IP address table for all ONTs
func zteWalkIPAddresses(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	ipAddresses := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ipOID := BaseOID2 + OnuIPAddressPrefix
	log.Printf("[IPAddress] Starting IP address walk on OID: %s", ipOID)

	err = bulkWalk(client, ipOID, func(pdu gosnmp.SnmpPDU) error {
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

// zteWalkMACAddresses walks the MAC address table for all ONTs
func zteWalkMACAddresses(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	macAddresses := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	macOID := BaseOID2 + OnuMACAddressPrefix
	log.Printf("[MACAddress] Starting MAC address walk on OID: %s", macOID)

	err = bulkWalk(client, macOID, func(pdu gosnmp.SnmpPDU) error {
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

// zteWalkHardwareVersions walks the hardware version table for all ONTs
func zteWalkHardwareVersions(ipAddress, community string, snmpPort int) (map[ONTLocation]string, error) {
	hwVersions := make(map[ONTLocation]string)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	hwOID := BaseOID2 + OnuHardwareVersionPrefix
	log.Printf("[HardwareVersion] Starting hardware version walk on OID: %s", hwOID)

	err = bulkWalk(client, hwOID, func(pdu gosnmp.SnmpPDU) error {
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
