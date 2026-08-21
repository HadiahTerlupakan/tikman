package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// DiscoverOLTTopology handles POST /api/v1/olts/:id/topology discovery
func (h *OLTHandler) DiscoverOLTTopology(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Get OLT details using the existing service
	olt, err := h.service.GetByID(oltID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "OLT not found",
		})
		return
	}

	// Decrypt SNMP community if needed
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CONFIG_ERROR",
			Error: "SNMP community not configured for this OLT",
		})
		return
	}

	// Perform topology discovery
	topology, err := connectivity.DiscoverOLTTopology(olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":   oltID,
		"topology": topology,
	})
}

// DiscoverONTs handles POST /api/v1/olts/:id/discover
func (h *OLTHandler) DiscoverONTs(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Get OLT details using the existing service
	olt, err := h.service.GetByID(oltID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "OLT not found",
		})
		return
	}

	// Decrypt SNMP community if needed
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CONFIG_ERROR",
			Error: "SNMP community not configured for this OLT",
		})
		return
	}

	// Use legacy flat discovery (deprecated but kept for backwards compat)
	discovered, err := connectivity.DiscoverONTs(olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Convert to response format
	results := make([]gin.H, len(discovered))
	for i, ont := range discovered {
		results[i] = gin.H{
			"port_id":       ont.PortID,
			"ont_id":        ont.ONTID,
			"serial_number": ont.SerialNumber,
			"run_state":     ont.RunState,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"discovered": len(discovered),
		"onts":       results,
	})
}

// DiscoverAndRegisterONTs handles POST /api/v1/olts/:id/discover-and-register
func (h *OLTHandler) DiscoverAndRegisterONTs(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	// Perform discovery
	discovered, err := h.service.DiscoverONTs(oltID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DISCOVERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Get ONT service
	ontService := services.NewONTService(h.service.GetDB())

	// Bulk register
	result := ontService.BulkRegisterFromDiscovery(oltID, discovered)

	// Audit log
	if h.auditService != nil && result.Registered > 0 {
		actorID, _ := middleware.GetUserID(c)
		_ = h.auditService.Log(
			actorID,
			"bulk_create",
			"ont",
			oltID,
			map[string]interface{}{"registered": result.Registered},
			nil,
			fmt.Sprintf("Registered %d ONTs from discovery", result.Registered),
			"",
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"discovered": len(discovered),
		"registered": result.Registered,
		"skipped":    result.Skipped,
		"errors":     result.Errors,
	})
}

// GetCachedTopology handles GET /api/v1/olts/:id/topology/cached - returns topology from database
func (h *OLTHandler) GetCachedTopology(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	var onts []struct {
		PortID       int    `json:"port_id"`
		ONTID        int    `json:"ont_id"`
		SerialNumber string `json:"serial_number"`
		Status       string `json:"status"`
		Name         string `json:"name"`
		Description  string `json:"description"`
	}

	err = h.service.GetDB().
		Table("onts").
		Select("port_id, ont_id, serial_number, status, name, description").
		Where("olt_id = ?", oltID).
		Scan(&onts).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "QUERY_FAILED",
			Error: "Failed to query ONTs from database",
		})
		return
	}

	// Assume slot 1 for all (since ONT model doesn't have slot_id yet)
	slotMap := make(map[int]map[int][]map[string]interface{})

	for _, ont := range onts {
		slot := 1
		if slotMap[slot] == nil {
			slotMap[slot] = make(map[int][]map[string]interface{})
		}

		ontData := map[string]interface{}{
			"ont_id":        ont.ONTID,
			"port_id":       ont.PortID,
			"serial_number": ont.SerialNumber,
			"status":        ont.Status,
			"name":          ont.Name,
			"description":   ont.Description,
			"run_state":     mapStatusToRunState(ont.Status),
		}

		slotMap[slot][ont.PortID] = append(slotMap[slot][ont.PortID], ontData)
	}

	var topology []map[string]interface{}
	for slot, ports := range slotMap {
		var portList []map[string]interface{}
		for portID, ontList := range ports {
			portList = append(portList, map[string]interface{}{
				"port_id": portID,
				"onts":    ontList,
			})
		}

		topology = append(topology, map[string]interface{}{
			"slot":  slot,
			"ports": portList,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"topology": topology,
		"source":   "database",
	})
}

// mapStatusToRunState converts ONT status to run_state integer
func mapStatusToRunState(status string) int {
	switch status {
	case "online":
		return 3
	case "offline":
		return 6
	case "los":
		return 1
	case "dying_gasp":
		return 4
	default:
		return 0
	}
}
