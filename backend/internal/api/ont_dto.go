package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ONT DTOs
type CreateONTRequest struct {
	OLTID        uuid.UUID        `json:"olt_id" binding:"required"`
	PortID       int              `json:"port_id" binding:"required,min=0,max=15"`
	ONTID        int              `json:"ont_id" binding:"required,min=0,max=127"`
	SerialNumber string           `json:"serial_number" binding:"required,min=1,max=20"`
	Description  string           `json:"description" binding:"omitempty,max=255"`
	Status       models.ONTStatus `json:"status" binding:"omitempty,oneof=online offline los dying_gasp unknown"`
	// Phone is validated and normalized by ONTService.Create, not here: the
	// binding tag can check length but not that it's a real Indonesian number.
	Phone string `json:"phone" binding:"omitempty,max=20"`
}

type UpdateONTRequest struct {
	Description *string           `json:"description" binding:"omitempty,max=255"`
	Status      *models.ONTStatus `json:"status" binding:"omitempty,oneof=online offline los dying_gasp unknown"`
	// A pointer, like Description: nil leaves the stored number untouched,
	// while an empty string is a deliberate clear.
	Phone *string `json:"phone" binding:"omitempty,max=20"`
}

type ONTResponse struct {
	ID      uuid.UUID `json:"id"`
	OLTID   uuid.UUID `json:"olt_id"`
	OLTName string    `json:"olt_name"`
	// Slot is the OLT card. The ONU's address reads card/port:id, and the
	// configure form fills its Card from this; leaving it out of the response
	// had the form fall back to card 1 for an ONU on card 3.
	Slot                 *int             `json:"slot,omitempty"`
	PortID               int              `json:"port_id"`
	ONTID                int              `json:"ont_id"`
	SerialNumber         string           `json:"serial_number"`
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	Phone                string           `json:"phone,omitempty"`
	DeviceType           string           `json:"device_type,omitempty"`
	HardwareVersion      string           `json:"hardware_version,omitempty"`
	SoftwareVersion      string           `json:"software_version,omitempty"`
	IPAddress            string           `json:"ip_address,omitempty"`
	MACAddress           string           `json:"mac_address,omitempty"`
	Status               models.ONTStatus `json:"status"`
	LastSeenAt           *time.Time       `json:"last_seen_at"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
	Distance             *int             `json:"distance,omitempty"`
	RxPower              *float64         `json:"rx_power,omitempty"`
	TxPower              *float64         `json:"tx_power,omitempty"`
	LastOnline           *time.Time       `json:"last_online,omitempty"`
	LastOffline          *time.Time       `json:"last_offline,omitempty"`
	LastOfflineReason    string           `json:"last_offline_reason,omitempty"`
	Uptime               int64            `json:"uptime,omitempty"`
	LastDownTimeDuration int64            `json:"last_down_time_duration,omitempty"`
	Temperature          *float64         `json:"temperature,omitempty"`
	Voltage              *float64         `json:"voltage,omitempty"`
	TxBiasCurrent        *float64         `json:"tx_bias_current,omitempty"`
	RxBytes              *uint64          `json:"rx_bytes,omitempty"`
	TxBytes              *uint64          `json:"tx_bytes,omitempty"`
	RxPackets            *uint64          `json:"rx_packets,omitempty"`
	TxPackets            *uint64          `json:"tx_packets,omitempty"`
	RxErrors             *uint64          `json:"rx_errors,omitempty"`
	TxErrors             *uint64          `json:"tx_errors,omitempty"`
}

func ToONTResponse(ont *models.ONT) ONTResponse {
	var distance *int
	if ont.Distance != 0 {
		distance = &ont.Distance
	}

	return ONTResponse{
		ID:                   ont.ID,
		OLTID:                ont.OLTID,
		OLTName:              "",
		Slot:                 ont.Slot,
		PortID:               ont.PortID,
		ONTID:                ont.ONTID,
		SerialNumber:         ont.SerialNumber,
		Name:                 ont.Name,
		Description:          ont.Description,
		Phone:                ont.Phone,
		DeviceType:           ont.DeviceType,
		HardwareVersion:      ont.HardwareVersion,
		SoftwareVersion:      ont.SoftwareVersion,
		IPAddress:            ont.IPAddress,
		MACAddress:           ont.MACAddress,
		Status:               ont.Status,
		LastSeenAt:           ont.LastSeenAt,
		CreatedAt:            ont.CreatedAt,
		UpdatedAt:            ont.UpdatedAt,
		Distance:             distance,
		RxPower:              ont.RxPower,
		TxPower:              ont.TxPower,
		LastOnline:           ont.LastOnline,
		LastOffline:          ont.LastOffline,
		LastOfflineReason:    ont.LastOfflineReason,
		Uptime:               ont.Uptime,
		LastDownTimeDuration: ont.LastDownTimeDuration,
	}
}

func ToONTResponseWithMetrics(ont *models.ONT, metrics *services.ONTMetricsRow) ONTResponse {
	resp := ToONTResponse(ont)
	if metrics != nil {
		// A reading the walk did not return must not replace the one already
		// stored on the ONT. The OLT drops varbinds under load, so an
		// unguarded overlay blanked the distance and optical power of rows that
		// had perfectly good values a minute earlier — the list appeared to
		// lose data at random.
		if metrics.Distance != 0 {
			resp.Distance = &metrics.Distance
		}
		if metrics.RxPower != nil {
			resp.RxPower = metrics.RxPower
		}
		if metrics.TxPower != nil {
			resp.TxPower = metrics.TxPower
		}
		if metrics.Temperature != 0 {
			resp.Temperature = &metrics.Temperature
		}
		if metrics.Voltage != 0 {
			resp.Voltage = &metrics.Voltage
		}
		if metrics.TxBiasCurrent != 0 {
			resp.TxBiasCurrent = &metrics.TxBiasCurrent
		}
		if metrics.RxBytes != 0 {
			resp.RxBytes = &metrics.RxBytes
		}
		if metrics.TxBytes != 0 {
			resp.TxBytes = &metrics.TxBytes
		}
		if metrics.RxPackets != 0 {
			resp.RxPackets = &metrics.RxPackets
		}
		if metrics.TxPackets != 0 {
			resp.TxPackets = &metrics.TxPackets
		}
		if metrics.RxErrors != 0 {
			resp.RxErrors = &metrics.RxErrors
		}
		if metrics.TxErrors != 0 {
			resp.TxErrors = &metrics.TxErrors
		}
	}
	return resp
}

// ONT Metrics DTOs
// RxPower/TxPower are nullable: null means the ONT reported no optical signal,
// which is different from a real 0.00 dBm reading.
type ONTMetricsResponse struct {
	Time        time.Time `json:"time"`
	RxPower     *float64  `json:"rx_power"`
	TxPower     *float64  `json:"tx_power"`
	Temperature float64   `json:"temperature"`
	Voltage     float64   `json:"voltage"`
	Distance    int       `json:"distance"`
	RxBytes     uint64    `json:"rx_bytes"`
	TxBytes     uint64    `json:"tx_bytes"`
	RxPackets   uint64    `json:"rx_packets"`
	TxPackets   uint64    `json:"tx_packets"`
	RxErrors    uint64    `json:"rx_errors"`
	TxErrors    uint64    `json:"tx_errors"`
	// Pointers for the same reason RxPower is one: a model whose rate gauges are
	// unsupported has no reading, and flattening that to 0 renders as "0.00 Mbps"
	// - an idle link - instead of "-". The HSGQ counter table is keyed by physical
	// port rather than by ONU, so this is the normal case there, not an edge case.
	RxMbps *float64 `json:"rx_mbps"`
	TxMbps *float64 `json:"tx_mbps"`
}

func ToONTMetricsResponse(metrics *services.ONTMetricsRow) ONTMetricsResponse {
	return ONTMetricsResponse{
		Time:        metrics.Time,
		RxPower:     metrics.RxPower,
		TxPower:     metrics.TxPower,
		Temperature: metrics.Temperature,
		Voltage:     metrics.Voltage,
		Distance:    metrics.Distance,
		RxBytes:     metrics.RxBytes,
		TxBytes:     metrics.TxBytes,
		RxPackets:   metrics.RxPackets,
		TxPackets:   metrics.TxPackets,
		RxErrors:    metrics.RxErrors,
		TxErrors:    metrics.TxErrors,
		RxMbps:      metrics.RxRateMbps,
		TxMbps:      metrics.TxRateMbps,
	}
}

// ONTTrafficTimeSeriesResponse represents a single data point in traffic time series
type ONTTrafficTimeSeriesResponse struct {
	Time    time.Time `json:"time"`
	RxBytes uint64    `json:"rx_bytes"`
	TxBytes uint64    `json:"tx_bytes"`
	RxMbps  float64   `json:"rx_mbps"`
	TxMbps  float64   `json:"tx_mbps"`
	RxMax   float64   `json:"rx_max_mbps"`
	TxMax   float64   `json:"tx_max_mbps"`
}
