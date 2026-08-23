package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

func (h *OLTHandler) Create(c *gin.Context) {
	var req CreateOLTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	sshPort := req.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	telnetPort := req.TelnetPort
	if telnetPort == 0 {
		telnetPort = 23
	}
	snmpPort := req.SNMPPort
	if snmpPort == 0 {
		snmpPort = 161
	}
	snmpCommunity := req.SNMPCommunity
	if snmpCommunity == "" {
		snmpCommunity = "public"
	}

	rack := 0
	shelf := 0
	slot := 0

	olt, err := h.service.Create(
		req.SiteID,
		req.Name,
		req.IPAddress,
		snmpCommunity,
		req.Username,
		req.Password,
		req.Model,
		rack,
		shelf,
		slot,
		sshPort,
		telnetPort,
		snmpPort,
		req.PreferredProtocol,
	)
	if err != nil {
		errMsg := err.Error()

		if errMsg == "site not found: record not found" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "Site not found",
				Code:  "INVALID_SITE_ID",
			})
			return
		}

		if errMsg == "IP address already exists" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "IP address already exists",
				Code:  "DUPLICATE_IP",
				Details: map[string]interface{}{
					"ip_address": req.IPAddress,
				},
			})
			return
		}

		if len(errMsg) > 21 && errMsg[:21] == "OLT validation failed" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "OLT validation failed",
				Code:    "VALIDATION_FAILED",
				Details: errMsg,
			})
			return
		}

		// An unreachable OLT is the operator's input to correct, not a server
		// fault, and the reason has to travel: "Failed to create OLT" with a 500
		// gave no hint that SNMP was the problem.
		if strings.HasPrefix(errMsg, "SNMP connection test failed") {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "SNMP connection test failed",
				Code:    "SNMP_TEST_FAILED",
				Details: errMsg,
			})
			return
		}

		log.Printf("[OLT Create] Failed to create OLT %s (%s): %v", req.Name, req.IPAddress, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create OLT",
			Code:  "CREATE_FAILED",
		})
		return
	}

	go autoDiscoverONTMetrics(h.service.GetDB(), olt)

	response := ToOLTResponse(h.service.GetDB(), olt)
	c.JSON(http.StatusCreated, response)
}

func autoDiscoverONTMetrics(db *gorm.DB, olt *models.OLT) {
	log.Printf("[AutoDiscovery] Starting immediate ONT metrics polling for OLT %s (%s)", olt.Name, olt.IPAddress)

	metricsService := services.NewMetricsService(db)

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot poll OLT %s: %v", olt.Name, err)
		return
	}

	allMetrics, err := driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		log.Printf("[AutoDiscovery] Failed to walk metrics from OLT %s: %v", olt.Name, err)
		return
	}

	log.Printf("[AutoDiscovery] Discovered %d ONT devices via SNMP walk", len(allMetrics))

	var successCount int

	for loc, metrics := range allMetrics {
		var onts []models.ONT
		if err := db.Where("olt_id = ? AND port_id = ? AND ont_id = ?", olt.ID, loc.Port, loc.ONTID).Find(&onts).Error; err != nil {
			continue
		}

		if len(onts) == 0 {
			log.Printf("[AutoDiscovery] Skipping unregistered ONT at port=%d ont=%d", loc.Port, loc.ONTID)
			continue
		}

		ont := onts[0]

		storeMetrics := false
		if metrics.RxPower != nil || metrics.TxPower != nil || metrics.Distance > 0 {
			storeMetrics = true
		}

		if storeMetrics {
			if err := metricsService.StoreMetrics(ont.ID, &metrics, nil); err != nil {
				log.Printf("[AutoDiscovery] Failed to store metrics for ONT %s: %v", ont.SerialNumber, err)
				continue
			}
			rxStr := "-"
			if metrics.RxPower != nil {
				rxStr = fmt.Sprintf("%.2f", *metrics.RxPower)
			}
			txStr := "-"
			if metrics.TxPower != nil {
				txStr = fmt.Sprintf("%.2f", *metrics.TxPower)
			}
			log.Printf("[AutoDiscovery] ✅ Polled metrics: serial=%s port=%d/%d rx_power=%s dBm tx_power=%s dBm distance=%dm",
				ont.SerialNumber, loc.Port, loc.ONTID, rxStr, txStr, metrics.Distance)
			successCount++
		}
	}

	log.Printf("[AutoDiscovery] Completed: polled metrics for %d ONTs from OLT %s", successCount, olt.Name)
}
