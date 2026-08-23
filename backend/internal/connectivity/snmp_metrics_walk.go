package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// zteWalkStatuses walks the ZXGPON phase state table and returns the raw phase
// state for every ONT the OLT knows about, keyed by its physical location.
//
// Walking is used rather than a per-ONT GET because the ZXGPON ifIndex encodes
// the line-card slot, which the OLT reports authoritatively - deriving it from
// stored rack/shelf/slot values is guesswork and silently yields wrong OIDs.
func zteWalkStatuses(ipAddress, community string, snmpPort int) (map[ONTLocation]int, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	statuses := make(map[ONTLocation]int)

	// Walk the entire ZXGPON phase state table
	err = client.Walk(OID_ZXAN_ONU_PHASE_STATE_TABLE, func(pdu gosnmp.SnmpPDU) error {
		loc, ok := parseZxGponSuffix(pdu.Name, OID_ZXAN_ONU_PHASE_STATE_TABLE)
		if !ok {
			return nil // skip non-ZXGPON entries
		}

		value, ok := toInt64(pdu.Value)
		if !ok || value >= 30000 {
			return nil // ignore invalid or "no signal" readings
		}

		statuses[loc] = int(value)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	log.Printf("[Walk] Collected %d ONT statuses", len(statuses))
	return statuses, nil
}

// zteWalkMetrics walks the optical metrics tables (power, distance) and returns
// collected metrics keyed by the ONT's location, decoded from each OID.
func zteWalkMetrics(ipAddress, community string, snmpPort int) (map[ONTLocation]ONTMetrics, error) {
	metrics := make(map[ONTLocation]ONTMetrics)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	log.Printf("[Metrics] Starting RX power walk on OID: %s", OID_ZXGPON_ONU_RX_POWER_TABLE)
	rxCount := 0
	rxValidCount := 0
	rxDecodeOkCount := 0
	err = client.Walk(OID_ZXGPON_ONU_RX_POWER_TABLE, func(pdu gosnmp.SnmpPDU) error {
		rxCount++
		trimmed := strings.TrimPrefix(pdu.Name, ".")
		baseTrimmed := strings.TrimPrefix(OID_ZXGPON_ONU_RX_POWER_TABLE, ".")
		if !strings.HasPrefix(trimmed, baseTrimmed+".") {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: prefix check failed", rxCount)
			}
			return nil
		}

		parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
		if len(parts) < 3 {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: parts check failed, got %d parts", rxCount, len(parts))
			}
			return nil
		}

		ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: ifIndex parse failed: %v", rxCount, err)
			}
			return nil
		}
		onuIndex, err := strconv.Atoi(parts[1])
		if err != nil {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: onuIndex parse failed: %v", rxCount, err)
			}
			return nil
		}

		slot, port, ok := decodeOnuIDIfIndex(uint32(ifIndexStr))
		if !ok {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: decode ifIndex failed for %d", rxCount, ifIndexStr)
			}
			return nil
		}

		value, ok := toInt64(pdu.Value)
		if !ok {
			if rxCount <= 3 {
				log.Printf("[Metrics] DEBUG RX #%d: toInt64 failed", rxCount)
			}
			return nil
		}

		rxDecodeOkCount++

		loc := ONTLocation{Slot: slot, Port: port, ONTID: onuIndex}

		rxPowerPtr := decodeZxGponPower(value)

		rxValidCount++
		if rxValidCount <= 3 {
			rxVal := "nil"
			if rxPowerPtr != nil {
				rxVal = fmt.Sprintf("%.2f", *rxPowerPtr)
			}
			log.Printf("[Metrics] DEBUG RX #%d: Slot=%d Port=%d ONTID=%d raw=%d decoded=%s", rxValidCount, slot, port, onuIndex, value, rxVal)
		}

		if m, found := metrics[loc]; found {
			m.RxPower = rxPowerPtr
			metrics[loc] = m
		} else {
			metrics[loc] = ONTMetrics{RxPower: rxPowerPtr}
		}
		return nil
	})

	log.Printf("[Metrics] RX power walk completed: %d entries found", rxCount)
	if err != nil {
		log.Printf("[Metrics] RX power walk failed: %v", err)
	}

	txMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_POWER_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxPower: decodeZxGponPower(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] TX power walk failed: %v", err)
	} else {
		for k, v := range txMetrics {
			if m, found := metrics[k]; found {
				m.TxPower = v.TxPower
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	distMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_DISTANCE_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{Distance: int(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] Distance walk failed: %v", err)
	} else {
		for k, v := range distMetrics {
			if m, found := metrics[k]; found {
				m.Distance = v.Distance
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	tempMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TEMPERATURE_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{Temperature: float64(raw) / 256.0}
	})
	if err != nil {
		log.Printf("[Metrics] Temperature walk failed: %v", err)
	} else {
		for k, v := range tempMetrics {
			if m, found := metrics[k]; found {
				m.Temperature = v.Temperature
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	voltMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_VOLTAGE_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{Voltage: float64(raw) / 10000.0}
	})
	if err != nil {
		log.Printf("[Metrics] Voltage walk failed: %v", err)
	} else {
		for k, v := range voltMetrics {
			if m, found := metrics[k]; found {
				m.Voltage = v.Voltage
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	biasMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_BIAS_CURRENT_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxBiasCurrent: float64(raw) / 500.0}
	})
	if err != nil {
		log.Printf("[Metrics] TX bias current walk failed: %v", err)
	} else {
		for k, v := range biasMetrics {
			if m, found := metrics[k]; found {
				m.TxBiasCurrent = v.TxBiasCurrent
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	rxBytesMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_RX_BYTES_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{RxBytes: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] RX bytes walk failed: %v", err)
	} else {
		for k, v := range rxBytesMetrics {
			if m, found := metrics[k]; found {
				m.RxBytes = v.RxBytes
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	txBytesMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_BYTES_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxBytes: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] TX bytes walk failed: %v", err)
	} else {
		for k, v := range txBytesMetrics {
			if m, found := metrics[k]; found {
				m.TxBytes = v.TxBytes
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	rxPacketsMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_RX_PACKETS_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{RxPackets: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] RX packets walk failed: %v", err)
	} else {
		for k, v := range rxPacketsMetrics {
			if m, found := metrics[k]; found {
				m.RxPackets = v.RxPackets
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	txPacketsMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_PACKETS_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxPackets: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] TX packets walk failed: %v", err)
	} else {
		for k, v := range txPacketsMetrics {
			if m, found := metrics[k]; found {
				m.TxPackets = v.TxPackets
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	rxErrorsMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_RX_ERRORS_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{RxErrors: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] RX errors walk failed: %v", err)
	} else {
		for k, v := range rxErrorsMetrics {
			if m, found := metrics[k]; found {
				m.RxErrors = v.RxErrors
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	txErrorsMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_ERRORS_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxErrors: uint64(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] TX errors walk failed: %v", err)
	} else {
		for k, v := range txErrorsMetrics {
			if m, found := metrics[k]; found {
				m.TxErrors = v.TxErrors
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	equipmentIDs, err := walkONTStringTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_EQUIPMENT_ID_TABLE)
	if err != nil {
		log.Printf("[Metrics] Equipment ID walk failed: %v", err)
	} else {
		log.Printf("[Metrics] Walked %d equipment IDs", len(equipmentIDs))
		for k, v := range equipmentIDs {
			if m, found := metrics[k]; found {
				m.SoftwareVersion = v
				metrics[k] = m
			} else {
				metrics[k] = ONTMetrics{SoftwareVersion: v}
			}
		}
	}

	log.Printf("[Metrics] Walked %d ONTs", len(metrics))
	return metrics, nil
}
